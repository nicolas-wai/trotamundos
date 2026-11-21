package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/messagequeue"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
)

type fakeClock struct {
	mu     sync.Mutex
	t      time.Time
	sleeps []time.Duration
}

func (c *fakeClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) sleep(_ context.Context, d time.Duration) error {
	c.mu.Lock()
	c.sleeps = append(c.sleeps, d)
	c.t = c.t.Add(d)
	c.mu.Unlock()
	return nil
}
func (c *fakeClock) advance(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

const provider = "agencia:agencia-sol"

func setup(t *testing.T) (*ProviderCircuitBreaker, *fakeClock, *[]events.ProviderDegraded) {
	t.Helper()
	bus := messagequeue.NewInMemoryEventBus()
	var mu sync.Mutex
	var e2s []events.ProviderDegraded
	bus.Subscribe(events.TopicProviderDegraded, func(_ context.Context, p any) {
		mu.Lock()
		e2s = append(e2s, p.(events.ProviderDegraded))
		mu.Unlock()
	})
	clk := &fakeClock{t: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)}
	b := NewProviderCircuitBreaker(bus).WithConfig(Config{MaxRetries: 3, Backoff: time.Second, Cooldown: time.Minute}).WithClock(clk.now, clk.sleep)
	return b, clk, &e2s
}

var errBoom = errors.New("503")

// Tras N fallas abre y publica E2; mientras está abierto no se llama al proveedor; pasa a semiabierto y cierra
// tras un éxito (ADR-07).
func TestCircuitOpensPublishesE2SkipsCallsHalfOpensAndCloses(t *testing.T) {
	b, clk, e2s := setup(t)
	calls := 0
	failing := func(context.Context) error { calls++; return errBoom }

	if err := b.Execute(context.Background(), provider, failing); !errors.Is(err, errBoom) {
		t.Fatalf("la falla se propaga: %v", err)
	}
	if calls != 4 { // 1 intento + 3 reintentos
		t.Fatalf("1 intento + 3 reintentos = 4 llamadas, hubo %d", calls)
	}
	if want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}; len(clk.sleeps) != 3 || clk.sleeps[0] != want[0] || clk.sleeps[1] != want[1] || clk.sleeps[2] != want[2] {
		t.Fatalf("espera progresiva esperada %v, fue %v", want, clk.sleeps)
	}
	if b.State(provider) != domain.CircuitOpen {
		t.Fatalf("estado %s, se esperaba ABIERTO", b.State(provider))
	}
	if len(*e2s) != 1 || (*e2s)[0].EstadoCircuito != "ABIERTO" || (*e2s)[0].IDProveedor != provider || (*e2s)[0].Tipo != "AGENCIA" ||
		(*e2s)[0].Reintentos != 3 || !(*e2s)[0].ProximoIntento.After(clk.now()) || (*e2s)[0].IDViajero != nil {
		t.Fatalf("E2 inesperado: %+v", *e2s)
	}

	// Abierto: no ejecuta la operación.
	calls = 0
	if err := b.Execute(context.Background(), provider, failing); !errors.Is(err, ports.ErrCircuitOpen) || calls != 0 {
		t.Fatalf("abierto debe omitir: err=%v calls=%d", err, calls)
	}

	// Pasado el cooldown pasa a semiabierto; una prueba fallida lo reabre sin reintentos.
	clk.advance(time.Minute)
	if b.State(provider) != domain.CircuitHalfOpen {
		t.Fatalf("estado %s, se esperaba SEMIABIERTO", b.State(provider))
	}
	if err := b.Execute(context.Background(), provider, failing); !errors.Is(err, errBoom) || calls != 1 {
		t.Fatalf("la prueba semiabierta es de un solo intento: err=%v calls=%d", err, calls)
	}
	if b.State(provider) != domain.CircuitOpen || len(*e2s) != 2 {
		t.Fatalf("debe reabrirse: %s, E2=%d", b.State(provider), len(*e2s))
	}

	// Otra prueba, ahora exitosa: cierra y publica E2 CERRADO (retira el aviso).
	clk.advance(time.Minute)
	if err := b.Execute(context.Background(), provider, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if b.State(provider) != domain.CircuitClosed {
		t.Fatalf("estado %s, se esperaba CERRADO", b.State(provider))
	}
	if last := (*e2s)[len(*e2s)-1]; last.EstadoCircuito != "CERRADO" {
		t.Fatalf("E2 de cierre esperado: %+v", last)
	}
}

func TestSuccessAfterRetriesKeepsCircuitClosed(t *testing.T) {
	b, _, e2s := setup(t)
	calls := 0
	err := b.Execute(context.Background(), provider, func(context.Context) error {
		calls++
		if calls < 3 {
			return errBoom
		}
		return nil
	})
	if err != nil || calls != 3 || b.State(provider) != domain.CircuitClosed || len(*e2s) != 0 {
		t.Fatalf("err=%v calls=%d estado=%s E2=%d", err, calls, b.State(provider), len(*e2s))
	}
}

// Una credencial inválida no es falla del proveedor: no se reintenta ni abre el circuito.
func TestInvalidCredentialDoesNotRetryNorOpen(t *testing.T) {
	b, _, e2s := setup(t)
	calls := 0
	err := b.Execute(context.Background(), "email:gmail", func(context.Context) error { calls++; return ports.ErrInvalidCredential })
	if !errors.Is(err, ports.ErrInvalidCredential) || calls != 1 || b.State("email:gmail") != domain.CircuitClosed || len(*e2s) != 0 {
		t.Fatalf("err=%v calls=%d estado=%s", err, calls, b.State("email:gmail"))
	}
}

// Un circuito por proveedor: la falla de uno no afecta a otro.
func TestCircuitsAreIndependentPerProvider(t *testing.T) {
	b, _, _ := setup(t)
	_ = b.Execute(context.Background(), provider, func(context.Context) error { return errBoom })
	if b.State("viajes:sistema-viajes") != domain.CircuitClosed {
		t.Fatal("el circuito de otro proveedor debe seguir cerrado")
	}
	if err := b.Execute(context.Background(), "viajes:sistema-viajes", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

// Semiabierto admite una única prueba a la vez.
func TestHalfOpenAllowsASingleTrial(t *testing.T) {
	b, clk, _ := setup(t)
	_ = b.Execute(context.Background(), provider, func(context.Context) error { return errBoom })
	clk.advance(time.Minute)
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error)
	go func() {
		done <- b.Execute(context.Background(), provider, func(context.Context) error { close(started); <-release; return nil })
	}()
	<-started
	if err := b.Execute(context.Background(), provider, func(context.Context) error { return nil }); !errors.Is(err, ports.ErrCircuitOpen) {
		t.Fatalf("una segunda llamada durante la prueba debe omitirse: %v", err)
	}
	close(release)
	if err := <-done; err != nil || b.State(provider) != domain.CircuitClosed {
		t.Fatalf("la prueba exitosa cierra: %v %s", err, b.State(provider))
	}
}
