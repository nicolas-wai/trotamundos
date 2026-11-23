//go:build load

// Prueba de carga [ADR-08][QA-02]: genera pedidos a ritmo fijo (modelo abierto) contra GET /dashboard por el
// balanceador sobre https y mide latencias. Solo corre con `make test-load` (etiqueta `load`): es lenta y
// depende de la máquina, por eso no forma parte de `make test`.
//
// Variables: LOAD_RATE (req/s, 500), LOAD_DURATION (30s), LOAD_USERS (30), LOAD_WRITE_PCT (porcentaje de
// escrituras para el escenario mixto, 10), LOAD_P95_MS (objetivo, 800), LOAD_SUMMARY (archivo del resumen),
// LOAD_CPUPROFILE (archivo de perfil de CPU de pprof, opcional).
package region_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/config"
	"github.com/nicolas-wai/trotamundos/src/loadbalancer"
	"github.com/nicolas-wai/trotamundos/src/region"
)

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v >= 0 {
		return v
	}
	return def
}

func envDuration(name string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

type loadUser struct{ id, token string }

type loadSystem struct {
	base   string
	client *http.Client
	users  []loadUser
}

// newLoadSystem arma la región completa con SQLite real, https con certificado propio y reloj real.
func newLoadSystem(t *testing.T, users int) *loadSystem {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.ShardCount = 3
	reg, err := region.New(cfg, region.Options{PasswordCost: 4})
	if err != nil {
		t.Fatal(err)
	}
	material, err := loadbalancer.NewSelfSignedTLS()
	if err != nil {
		t.Fatal(err)
	}
	startTLS := func(h http.Handler) *httptest.Server {
		s := httptest.NewUnstartedServer(h)
		s.TLS = material.ServerConfig()
		s.StartTLS()
		return s
	}
	var urls []*url.URL
	var servers []*httptest.Server
	for _, rep := range reg.Replicas {
		s := startTLS(rep.Handler())
		servers = append(servers, s)
		u, _ := url.Parse(s.URL)
		urls = append(urls, u)
	}
	lb := startTLS(reg.NewLoadBalancer(urls, loadbalancer.WithBackendTLS(material)))
	ctx, cancel := context.WithCancel(context.Background())
	reg.Start(ctx)
	t.Cleanup(func() {
		cancel()
		lb.Close()
		for _, s := range servers {
			s.Close()
		}
		reg.Close()
	})
	sys := &loadSystem{base: lb.URL, client: material.HTTPClient()}
	for i := 0; i < users; i++ {
		email := fmt.Sprintf("carga%d@example.com", i)
		creds := map[string]string{"email": email, "password": "clave-segura-1"}
		var reg map[string]any
		sys.call(t, "POST", "/registro", "", creds, &reg)
		var login map[string]any
		sys.call(t, "POST", "/login", "", creds, &login)
		u := loadUser{id: reg["idViajero"].(string), token: login["accessToken"].(string)}
		for j := 0; j < 5; j++ { // datos de prueba: cinco arreglos por usuario
			sys.call(t, "POST", "/arreglos", u.token, arrangementBody("demo-"+u.id, fmt.Sprintf("Reserva %d", j), time.Now().AddDate(0, 0, 10+j), nil), nil)
		}
		sys.users = append(sys.users, u)
	}
	return sys
}

func (s *loadSystem) call(t *testing.T, method, path, token string, body, out any) {
	t.Helper()
	var rd io.Reader = http.NoBody
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, s.base+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, raw)
	}
	if out != nil {
		_ = json.Unmarshal(raw, out)
	}
}

type loadResult struct {
	name                      string
	rate                      int
	duration                  time.Duration
	sent, ok, failed, dropped int64
	p50, p95, p99, max        time.Duration
	reads, writes             int64
	achieved                  float64
	readP95, writeP95         time.Duration
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(float64(len(sorted))*p) - 1
	if i < 0 {
		i = 0
	}
	return sorted[i]
}

// runLoad lanza pedidos a ritmo fijo (ticker, modelo abierto): la latencia se mide desde el instante en que el
// pedido debía salir, así una cola en el cliente no esconde la espera (sin omisión coordinada).
func runLoad(t *testing.T, sys *loadSystem, name string, rate int, duration time.Duration, writePct int) loadResult {
	t.Helper()
	const maxInFlight = 4000
	var (
		mu                        sync.Mutex
		all, readLat, writeLat    []time.Duration
		sent, ok, failed, dropped atomic.Int64
		wg                        sync.WaitGroup
		inFlight                  = make(chan struct{}, maxInFlight)
		seq                       atomic.Int64
	)
	fire := func(n int64, scheduled time.Time) {
		defer wg.Done()
		defer func() { <-inFlight }()
		u := sys.users[int(n)%len(sys.users)]
		isWrite := writePct > 0 && int(n%100) < writePct
		var req *http.Request
		if isWrite {
			b, _ := json.Marshal(arrangementBody("demo-"+u.id, "Carga "+strconv.FormatInt(n, 10), time.Now().AddDate(0, 0, 30), nil))
			req, _ = http.NewRequest("POST", sys.base+"/arreglos", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest("GET", sys.base+"/dashboard", nil)
		}
		req.Header.Set("Authorization", "Bearer "+u.token)
		resp, err := sys.client.Do(req)
		good := false
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			good = resp.StatusCode == 200 || resp.StatusCode == 201
		}
		lat := time.Since(scheduled)
		mu.Lock()
		all = append(all, lat)
		if isWrite {
			writeLat = append(writeLat, lat)
		} else {
			readLat = append(readLat, lat)
		}
		mu.Unlock()
		if good {
			ok.Add(1)
		} else {
			failed.Add(1)
		}
	}

	start := time.Now()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for elapsed := time.Duration(0); elapsed < duration; elapsed = time.Since(start) {
		<-tick.C
		due := int64(float64(rate) * time.Since(start).Seconds())
		for sent.Load() < due {
			n := seq.Add(1)
			sent.Add(1)
			scheduled := start.Add(time.Duration(float64(n) / float64(rate) * float64(time.Second)))
			select {
			case inFlight <- struct{}{}:
				wg.Add(1)
				go fire(n, scheduled)
			default: // demasiados pedidos en vuelo: el sistema no da abasto, se cuenta como perdido
				dropped.Add(1)
			}
		}
	}
	wg.Wait()
	total := time.Since(start)

	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	sort.Slice(readLat, func(i, j int) bool { return readLat[i] < readLat[j] })
	sort.Slice(writeLat, func(i, j int) bool { return writeLat[i] < writeLat[j] })
	res := loadResult{
		name: name, rate: rate, duration: duration, sent: sent.Load(), ok: ok.Load(), failed: failed.Load(), dropped: dropped.Load(),
		p50: percentile(all, 0.50), p95: percentile(all, 0.95), p99: percentile(all, 0.99),
		reads: int64(len(readLat)), writes: int64(len(writeLat)),
		readP95: percentile(readLat, 0.95), writeP95: percentile(writeLat, 0.95),
		achieved: float64(len(all)) / total.Seconds(),
	}
	if len(all) > 0 {
		res.max = all[len(all)-1]
	}
	return res
}

func (r loadResult) summary() string {
	errRate := 0.0
	if r.sent > 0 {
		errRate = float64(r.failed+r.dropped) / float64(r.sent) * 100
	}
	return fmt.Sprintf("%s: objetivo %d req/s x %v | enviados %d (lecturas %d, escrituras %d) | completados %.0f req/s\n"+
		"  p50=%v p95=%v p99=%v max=%v | p95 lecturas=%v p95 escrituras=%v | errores %d + perdidos %d = %.2f%%\n",
		r.name, r.rate, r.duration, r.sent, r.reads, r.writes, r.achieved,
		r.p50.Round(time.Microsecond), r.p95.Round(time.Microsecond), r.p99.Round(time.Microsecond), r.max.Round(time.Microsecond),
		r.readP95.Round(time.Microsecond), r.writeP95.Round(time.Microsecond), r.failed, r.dropped, errRate)
}

// TestLoad500RequestsPerSecond [ADR-08][QA-02] 500 req/s sostenidos: solo lecturas del dashboard y mezcla 90/10.
func TestLoad500RequestsPerSecond(t *testing.T) {
	rate := envInt("LOAD_RATE", 500)
	duration := envDuration("LOAD_DURATION", 30*time.Second)
	writePct := envInt("LOAD_WRITE_PCT", 10)
	target := time.Duration(envInt("LOAD_P95_MS", 800)) * time.Millisecond
	sys := newLoadSystem(t, envInt("LOAD_USERS", 30))

	if path := os.Getenv("LOAD_CPUPROFILE"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		_ = pprof.StartCPUProfile(f)
		defer func() { pprof.StopCPUProfile(); f.Close() }()
	}

	// Calentamiento: llena la caché y las conexiones https.
	runLoad(t, sys, "calentamiento", rate/2, 3*time.Second, 0)

	var out bytes.Buffer
	fmt.Fprintf(&out, "Prueba de carga %s | máquina: %d núcleos (GOMAXPROCS=%d), %s/%s, %s\n",
		time.Now().Format(time.RFC3339), runtime.NumCPU(), runtime.GOMAXPROCS(0), runtime.GOOS, runtime.GOARCH, runtime.Version())
	reads := runLoad(t, sys, "solo lecturas GET /dashboard", rate, duration, 0)
	mixed := runLoad(t, sys, fmt.Sprintf("mezcla %d/%d lecturas/escrituras", 100-writePct, writePct), rate, duration, writePct)
	for _, r := range []loadResult{reads, mixed} {
		out.WriteString(r.summary())
	}
	t.Log("\n" + out.String())
	path := os.Getenv("LOAD_SUMMARY")
	if path == "" {
		path = "../../bin/load-summary.txt"
	}
	if err := os.MkdirAll(dirOf(path), 0o755); err == nil {
		_ = os.WriteFile(path, out.Bytes(), 0o644)
	}

	for _, r := range []loadResult{reads, mixed} {
		if r.p95 >= target {
			t.Errorf("%s: p95 = %v, el objetivo es < %v", r.name, r.p95, target)
		}
		if r.failed+r.dropped > 0 {
			t.Errorf("%s: %d pedidos fallidos y %d perdidos", r.name, r.failed, r.dropped)
		}
	}
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
