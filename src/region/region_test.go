package region_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/config"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/region"
)

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type env struct {
	reg   *region.Region
	lb    *httptest.Server
	clock *testClock
	cfg   config.RegionConfig
	spy   *queueSpy
}

// queueSpy registra las operaciones E8 que las réplicas entregan al nodo de cola de prioridades.
type queueSpy struct {
	mu  sync.Mutex
	ops []events.QueuedOperation
}

func (s *queueSpy) record(op events.QueuedOperation) {
	s.mu.Lock()
	s.ops = append(s.ops, op)
	s.mu.Unlock()
}

// mark devuelve la cantidad de operaciones vistas hasta ahora; since devuelve las posteriores a esa marca.
func (s *queueSpy) mark() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ops)
}

func (s *queueSpy) since(mark int) []events.QueuedOperation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]events.QueuedOperation(nil), s.ops[mark:]...)
}

// newEnv arma una región completa con SQLite real en un directorio temporal e intervalos acelerados.
func newEnv(t *testing.T) *env {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.HeartbeatInterval = 20 * time.Millisecond
	cfg.ShardCount = 3
	clock := &testClock{t: time.Now()}
	spy := &queueSpy{}
	reg, err := region.New(cfg, region.Options{Clock: clock.Now, PasswordCost: 4, OnQueuedOperation: spy.record})
	if err != nil {
		t.Fatal(err)
	}
	var urls []*url.URL
	var servers []*httptest.Server
	for _, rep := range reg.Replicas {
		s := httptest.NewServer(rep.Handler())
		servers = append(servers, s)
		u, _ := url.Parse(s.URL)
		urls = append(urls, u)
	}
	lb := httptest.NewServer(reg.NewLoadBalancer(urls))
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
	return &env{reg: reg, lb: lb, clock: clock, cfg: cfg, spy: spy}
}

func (e *env) do(t *testing.T, method, path, token string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var rd bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = *bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, e.lb.URL+path, &rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no se cumplió a tiempo: %s", what)
}

func creds(email, pass string) map[string]string {
	return map[string]string{"email": email, "password": pass}
}

// Flujo AD 5.5: autenticación con tokens de vida corta validados localmente por cualquier réplica.
func TestFlow55AuthenticationWithShortLivedTokens(t *testing.T) {
	e := newEnv(t)

	// Registro 201 y email único 409
	resp, body := e.do(t, "POST", "/registro", "", creds("Ana@Example.com", "clave-segura-1"))
	if resp.StatusCode != 201 || body["idViajero"] == "" {
		t.Fatalf("registro: %d %v", resp.StatusCode, body)
	}
	if resp, body = e.do(t, "POST", "/registro", "", creds("ana@example.com", "otra-clave-123")); resp.StatusCode != 409 || body["codigo"] != "CONFLICTO" {
		t.Fatalf("duplicado: %d %v", resp.StatusCode, body)
	}
	if resp, body = e.do(t, "POST", "/registro", "", creds("sin-arroba", "clave-segura-1")); resp.StatusCode != 422 || body["codigo"] != "VALIDACION_FALLIDA" {
		t.Fatalf("email inválido: %d %v", resp.StatusCode, body)
	}
	if resp, _ = e.do(t, "POST", "/registro", "", creds("b@example.com", "corta")); resp.StatusCode != 422 {
		t.Fatalf("contraseña corta: %d", resp.StatusCode)
	}

	// Contraseña incorrecta y email desconocido → mismo 401
	for _, c := range []map[string]string{creds("ana@example.com", "incorrecta-123"), creds("nadie@example.com", "clave-segura-1")} {
		if resp, body = e.do(t, "POST", "/login", "", c); resp.StatusCode != 401 || body["codigo"] != "CREDENCIALES_INVALIDAS" {
			t.Fatalf("login inválido: %d %v", resp.StatusCode, body)
		}
	}

	// Login 200 (el email se normaliza)
	resp, body = e.do(t, "POST", "/login", "", creds("ANA@example.com", "clave-segura-1"))
	if resp.StatusCode != 200 || body["tokenType"] != "Bearer" || body["expiresIn"] != float64(900) {
		t.Fatalf("login: %d %v", resp.StatusCode, body)
	}
	access, refresh := body["accessToken"].(string), body["refreshToken"].(string)

	// Requests autenticados atendidos por réplicas distintas sin estado compartido: pasan el token (501 ≠ 401).
	// Se usa /recomendaciones, que sigue sin implementar hasta la Fase 5.
	replicas := map[string]bool{}
	for i := 0; i < 4; i++ {
		resp, body = e.do(t, "GET", "/recomendaciones", access, nil)
		if resp.StatusCode != 501 || body["codigo"] != "NO_IMPLEMENTADO" {
			t.Fatalf("con token válido: %d %v", resp.StatusCode, body)
		}
		replicas[resp.Header.Get("X-Replica-Id")] = true
	}
	if len(replicas) != 2 {
		t.Fatalf("debían atender las 2 réplicas, atendieron %v", replicas)
	}
	if resp, body = e.do(t, "GET", "/recomendaciones", "", nil); resp.StatusCode != 401 || body["codigo"] != "TOKEN_INVALIDO" {
		t.Fatalf("sin token: %d %v", resp.StatusCode, body)
	}

	// Access token vencido → 401 TOKEN_EXPIRADO (en ambas réplicas)
	e.clock.Advance(901 * time.Second)
	for i := 0; i < 2; i++ {
		if resp, body = e.do(t, "GET", "/recomendaciones", access, nil); resp.StatusCode != 401 || body["codigo"] != "TOKEN_EXPIRADO" {
			t.Fatalf("vencido: %d %v", resp.StatusCode, body)
		}
	}

	// Refresh 200 con access nuevo que vuelve a funcionar
	resp, body = e.do(t, "POST", "/token/refresh", "", map[string]string{"refreshToken": refresh})
	if resp.StatusCode != 200 || body["tokenType"] != "Bearer" || body["expiresIn"] != float64(900) {
		t.Fatalf("refresh: %d %v", resp.StatusCode, body)
	}
	if resp, _ = e.do(t, "GET", "/recomendaciones", body["accessToken"].(string), nil); resp.StatusCode != 501 {
		t.Fatalf("el access renovado debía ser aceptado: %d", resp.StatusCode)
	}

	// Logout 204 y el refresh queda revocado
	if resp, _ = e.do(t, "POST", "/logout", "", map[string]string{"refreshToken": refresh}); resp.StatusCode != 204 {
		t.Fatalf("logout: %d", resp.StatusCode)
	}
	if resp, body = e.do(t, "POST", "/token/refresh", "", map[string]string{"refreshToken": refresh}); resp.StatusCode != 401 || body["codigo"] != "REFRESH_REVOCADO_O_EXPIRADO" {
		t.Fatalf("refresh revocado: %d %v", resp.StatusCode, body)
	}
	if resp, _ = e.do(t, "POST", "/logout", "", map[string]string{"refreshToken": refresh}); resp.StatusCode != 204 {
		t.Fatalf("logout idempotente: %d", resp.StatusCode)
	}

	// JSON malformado → 400
	req, _ := http.NewRequest("POST", e.lb.URL+"/login", bytes.NewBufferString("{no es json"))
	r, _ := http.DefaultClient.Do(req)
	r.Body.Close()
	if r.StatusCode != 400 {
		t.Fatalf("JSON malformado: %d", r.StatusCode)
	}
}

// La contraseña nunca se guarda en claro y los datos quedan en archivos SQLite por nodo y shard. [ADR-14][ADR-10]
func TestDataLandsInSQLiteFilesAndPasswordIsHashed(t *testing.T) {
	e := newEnv(t)
	e.do(t, "POST", "/registro", "", creds("ana@example.com", "clave-segura-1"))
	dir := filepath.Join(e.cfg.DataDir, e.reg.ActiveNode.ID())
	for _, f := range []string{"primary-shard-0.db", "replica-shard-0.db", "primary-directory.db", "replica-directory.db"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("falta %s: %v", f, err)
		}
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.db"))
	for _, f := range files {
		raw, _ := os.ReadFile(f)
		if bytes.Contains(raw, []byte("clave-segura-1")) {
			t.Fatalf("%s contiene la contraseña en claro", f)
		}
	}
}

// Flujo AD 5.7 (réplica): tras 3 pulsos perdidos el LB la retira y el tráfico sigue 2xx.
func TestFlow57ReplicaFailureIsDetectedAndTrafficContinues(t *testing.T) {
	e := newEnv(t)
	e.do(t, "POST", "/registro", "", creds("ana@example.com", "clave-segura-1"))
	victim := e.reg.Replicas[0].ID

	e.reg.StopReplica(0)
	waitFor(t, "réplica retirada del LB", func() bool { return !e.reg.LB.IsHealthy(victim) })
	for i := 0; i < 6; i++ {
		resp, _ := e.do(t, "POST", "/login", "", creds("ana@example.com", "clave-segura-1"))
		if resp.StatusCode != 200 {
			t.Fatalf("tras la retirada el tráfico debía ser 2xx: %d", resp.StatusCode)
		}
		if resp.Header.Get("X-Replica-Id") == victim {
			t.Fatal("la réplica retirada no debía recibir tráfico")
		}
	}
}

// Flujo AD 5.7 (datos): caída del primario → E4, el pasivo atiende y los datos sincronizados se leen.
func TestFlow57DataNodeFailoverServesSyncedData(t *testing.T) {
	e := newEnv(t)
	e.do(t, "POST", "/registro", "", creds("ana@example.com", "clave-segura-1"))
	if _, err := e.reg.ActiveNode.SyncNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	oldActive, oldPassive := e.reg.ActiveNode, e.reg.PassiveNode

	oldActive.Stop()
	waitFor(t, "E4 emitido", func() bool { return len(e.reg.Swaps()) == 1 })
	swap := e.reg.Swaps()[0]
	if swap.NuevoPrimario != oldPassive.ID() || swap.NuevoPasivo != oldActive.ID() || swap.Motivo != "HEARTBEAT_PERDIDO" || swap.RegionID != e.cfg.RegionID {
		t.Fatalf("E4 inesperado: %+v", swap)
	}
	if e.reg.Switch.ActiveID() != oldPassive.ID() || oldPassive.Role() != "ACTIVO" || oldActive.Role() != "PASIVO" {
		t.Fatal("el pasivo debía quedar como primario")
	}

	// El usuario registrado antes de la caída puede autenticarse, y se pueden registrar usuarios nuevos.
	resp, body := e.do(t, "POST", "/login", "", creds("ana@example.com", "clave-segura-1"))
	if resp.StatusCode != 200 {
		t.Fatalf("login tras el failover: %d %v", resp.StatusCode, body)
	}
	if resp, body = e.do(t, "POST", "/registro", "", creds("luis@example.com", "clave-segura-2")); resp.StatusCode != 201 {
		t.Fatalf("registro tras el failover: %d %v", resp.StatusCode, body)
	}
	// El nodo promovido heartbeatea como ACTIVO.
	waitFor(t, "heartbeat del nuevo primario", func() bool {
		hb, ok := e.reg.DataHeartbeats.Last(oldPassive.ID())
		return ok && hb.Rol == "ACTIVO"
	})
}
