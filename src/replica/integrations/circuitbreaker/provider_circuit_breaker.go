// Package circuitbreaker es el componente "Circuit breaker y reintentos acotados" (AD §4.4).
package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
)

// Config parametriza el circuit breaker. [ADR-07]
type Config struct {
	MaxRetries int           // reintentos tras el primer intento (N)
	Backoff    time.Duration // espera base; se duplica en cada reintento (progresiva)
	Cooldown   time.Duration // tiempo con el circuito abierto antes de la prueba (semiabierto)
}

// DefaultConfig son los parámetros por defecto: 3 reintentos, 500 ms de espera base, 30 s abierto.
func DefaultConfig() Config {
	return Config{MaxRetries: 3, Backoff: 500 * time.Millisecond, Cooldown: 30 * time.Second}
}

type circuit struct {
	state    domain.CircuitState
	openedAt time.Time
}

// ProviderCircuitBreaker [ADR-07] Un circuito por proveedor (CERRADO/ABIERTO/SEMIABIERTO), N reintentos
// con espera progresiva; al abrir publica E2 ProveedorDegradado en el bus de eventos y al recuperarse publica
// E2 con el circuito CERRADO (para retirar el aviso al viajero).
type ProviderCircuitBreaker struct {
	bus   ports.EventBusPort
	cfg   Config
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error

	mu       sync.Mutex
	circuits map[string]*circuit
}

var _ ports.CircuitBreakerPort = (*ProviderCircuitBreaker)(nil)

// NewProviderCircuitBreaker crea el circuit breaker con la configuración por defecto.
func NewProviderCircuitBreaker(bus ports.EventBusPort) *ProviderCircuitBreaker {
	return &ProviderCircuitBreaker{bus: bus, cfg: DefaultConfig(), now: time.Now, sleep: sleepCtx, circuits: map[string]*circuit{}}
}

// WithConfig reemplaza la configuración.
func (b *ProviderCircuitBreaker) WithConfig(cfg Config) *ProviderCircuitBreaker {
	b.cfg = cfg
	return b
}

// WithClock inyecta reloj y espera (pruebas deterministas sin dormir de verdad).
func (b *ProviderCircuitBreaker) WithClock(now func() time.Time, sleep func(context.Context, time.Duration) error) *ProviderCircuitBreaker {
	b.now, b.sleep = now, sleep
	return b
}

// Execute ejecuta la operación bajo la protección del circuito del proveedor:
//   - CERRADO: intenta y, si falla, reintenta hasta MaxRetries veces con espera progresiva; si se agotan abre el
//     circuito y publica E2.
//   - ABIERTO: no ejecuta nada (ports.ErrCircuitOpen) hasta que pase el Cooldown.
//   - SEMIABIERTO: una única prueba; si funciona cierra el circuito, si falla lo vuelve a abrir.
//
// ports.ErrInvalidCredential no es una falla del proveedor: no se reintenta ni abre el circuito.
func (b *ProviderCircuitBreaker) Execute(ctx context.Context, providerID string, operation func(context.Context) error) error {
	switch b.State(providerID) {
	case domain.CircuitOpen:
		return ports.ErrCircuitOpen
	case domain.CircuitHalfOpen:
		if !b.beginTrial(providerID) { // otra llamada ya está probando: una única prueba a la vez
			return ports.ErrCircuitOpen
		}
		err := operation(ctx)
		if isProviderFault(err) {
			b.open(ctx, providerID, 0)
			return err
		}
		b.close(ctx, providerID)
		return err
	}

	var err error
	for attempt := 0; ; attempt++ {
		if err = operation(ctx); !isProviderFault(err) {
			b.close(ctx, providerID) // un éxito (o un error no atribuible al proveedor) mantiene el circuito cerrado
			return err
		}
		if attempt >= b.cfg.MaxRetries {
			break
		}
		if serr := b.sleep(ctx, b.cfg.Backoff<<attempt); serr != nil {
			return serr
		}
	}
	b.open(ctx, providerID, b.cfg.MaxRetries)
	return err
}

// State devuelve el estado del circuito. Un circuito abierto cuyo Cooldown ya pasó se informa SEMIABIERTO
// (listo para la prueba), así el planificador solo omite los que de verdad siguen en espera.
func (b *ProviderCircuitBreaker) State(providerID string) domain.CircuitState {
	b.mu.Lock()
	defer b.mu.Unlock()
	c, ok := b.circuits[providerID]
	if !ok {
		return domain.CircuitClosed
	}
	if c.state == domain.CircuitOpen && !b.now().Before(c.openedAt.Add(b.cfg.Cooldown)) {
		return domain.CircuitHalfOpen
	}
	return c.state
}

// beginTrial pasa de ABIERTO (con el Cooldown cumplido) a SEMIABIERTO; false si no corresponde o ya hay una prueba.
func (b *ProviderCircuitBreaker) beginTrial(providerID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	c, ok := b.circuits[providerID]
	if !ok || c.state != domain.CircuitOpen || b.now().Before(c.openedAt.Add(b.cfg.Cooldown)) {
		return false
	}
	c.state = domain.CircuitHalfOpen
	return true
}

func (b *ProviderCircuitBreaker) circuit(providerID string) *circuit {
	c, ok := b.circuits[providerID]
	if !ok {
		c = &circuit{state: domain.CircuitClosed}
		b.circuits[providerID] = c
	}
	return c
}

// open abre el circuito y publica E2 ABIERTO con el instante del próximo intento.
func (b *ProviderCircuitBreaker) open(ctx context.Context, providerID string, retries int) {
	b.mu.Lock()
	c := b.circuit(providerID)
	c.state, c.openedAt = domain.CircuitOpen, b.now()
	next := c.openedAt.Add(b.cfg.Cooldown)
	b.mu.Unlock()
	b.publish(ctx, events.ProviderDegraded{
		IDProveedor: providerID, Tipo: string(domain.ProviderTypeOf(providerID)), EstadoCircuito: string(domain.CircuitOpen),
		Reintentos: retries, ProximoIntento: next.UTC(),
	})
}

// close cierra el circuito; si estaba degradado publica E2 CERRADO.
func (b *ProviderCircuitBreaker) close(ctx context.Context, providerID string) {
	b.mu.Lock()
	c, ok := b.circuits[providerID]
	wasDegraded := ok && c.state != domain.CircuitClosed
	if ok {
		c.state = domain.CircuitClosed
	}
	b.mu.Unlock()
	if wasDegraded {
		b.publish(ctx, events.ProviderDegraded{
			IDProveedor: providerID, Tipo: string(domain.ProviderTypeOf(providerID)), EstadoCircuito: string(domain.CircuitClosed),
			ProximoIntento: b.now().UTC(),
		})
	}
}

func (b *ProviderCircuitBreaker) publish(ctx context.Context, e2 events.ProviderDegraded) {
	if b.bus != nil {
		_ = b.bus.Publish(ctx, events.TopicProviderDegraded, e2)
	}
}

// isProviderFault indica si el error cuenta como falla del proveedor. nil, credenciales inválidas y la
// cancelación del contexto no lo son.
func isProviderFault(err error) bool {
	return err != nil && !errors.Is(err, ports.ErrInvalidCredential) && !errors.Is(err, context.Canceled)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
