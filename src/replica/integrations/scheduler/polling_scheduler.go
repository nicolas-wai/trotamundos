// Package scheduler es el componente "Planificador de sondeo" de Integraciones externas (AD §4.4).
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
)

// Target es lo que se sondea en un ciclo: un proveedor de un tipo para una referencia (viajero y cuenta).
type Target struct {
	Type domain.ProviderType
	Ref  ports.ProviderReference
}

// TargetSource enumera los objetivos de sondeo (cuentas de correo vinculadas, agencias, sistemas de viajes).
type TargetSource interface {
	Targets(ctx context.Context) ([]Target, error)
}

// AdapterRegistry resuelve el adapter de un tipo de proveedor (lo implementa providerport.ProviderPortRegistry). [ADR-05]
type AdapterRegistry interface {
	Adapter(t domain.ProviderType) (ports.ExternalProviderPort, bool)
}

// UpdateSink recibe las actualizaciones sondeadas (lo implementa el Broker). [ADR-06]
type UpdateSink interface {
	Ingest(ctx context.Context, updates []events.ProviderUpdateNormalized) error
}

// PollingScheduler [ADR-05][QA-01] Dispara sondeos cada <= 2 min sobre cuentas y proveedores, pasando por el
// circuit breaker; omite proveedores con circuito abierto. Cada sondeo pide solo lo posterior al último exitoso.
type PollingScheduler struct {
	interval time.Duration
	breaker  ports.CircuitBreakerPort
	adapters AdapterRegistry
	targets  TargetSource
	sink     UpdateSink

	timeout time.Duration
	now     func() time.Time
	// OnError recibe los errores de sondeo e ingesta (nil = se ignoran). Pruebas y logging.
	OnError func(Target, error)

	mu    sync.Mutex
	since map[string]time.Time
}

// NewPollingScheduler crea el planificador.
func NewPollingScheduler(interval time.Duration, breaker ports.CircuitBreakerPort, adapters AdapterRegistry, targets TargetSource, sink UpdateSink) *PollingScheduler {
	return &PollingScheduler{
		interval: interval, breaker: breaker, adapters: adapters, targets: targets, sink: sink,
		timeout: 10 * time.Second, now: time.Now, since: map[string]time.Time{},
	}
}

// WithPollTimeout fija el límite de cada sondeo a un proveedor.
func (s *PollingScheduler) WithPollTimeout(d time.Duration) *PollingScheduler {
	s.timeout = d
	return s
}

// Run sondea inmediatamente y luego cada intervalo hasta que el contexto termina.
func (s *PollingScheduler) Run(ctx context.Context) error {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		s.PollOnce(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// PollOnce ejecuta un ciclo: sondea todos los objetivos en paralelo y espera a que terminen.
func (s *PollingScheduler) PollOnce(ctx context.Context) {
	targets, err := s.targets.Targets(ctx)
	if err != nil {
		s.report(Target{}, fmt.Errorf("enumerando objetivos de sondeo: %w", err))
		return
	}
	var wg sync.WaitGroup
	for _, t := range targets {
		wg.Add(1)
		go func() { defer wg.Done(); s.poll(ctx, t) }()
	}
	wg.Wait()
}

func (s *PollingScheduler) poll(ctx context.Context, t Target) {
	if s.breaker.State(t.Ref.ProviderID) == domain.CircuitOpen {
		return // circuito abierto: se omite y se reintenta más tarde [ADR-07]
	}
	adapter, ok := s.adapters.Adapter(t.Type)
	if !ok {
		s.report(t, fmt.Errorf("sin adapter registrado para %s", t.Type))
		return
	}
	key := t.Ref.ProviderID + "|" + t.Ref.TravelerID + "|" + t.Ref.AccountID
	s.mu.Lock()
	since := s.since[key]
	s.mu.Unlock()

	started := s.now()
	var updates []events.ProviderUpdateNormalized
	err := s.breaker.Execute(ctx, t.Ref.ProviderID, func(ctx context.Context) error {
		pctx, cancel := context.WithTimeout(ctx, s.timeout)
		defer cancel()
		var err error
		updates, err = adapter.Poll(pctx, t.Ref, since)
		return err
	})
	switch {
	case errors.Is(err, ports.ErrCircuitOpen):
		return
	case err != nil:
		s.report(t, err)
		return
	}
	if len(updates) > 0 {
		if err := s.sink.Ingest(ctx, updates); err != nil {
			s.report(t, err)
		}
	}
	s.mu.Lock()
	s.since[key] = started
	s.mu.Unlock()
}

func (s *PollingScheduler) report(t Target, err error) {
	if s.OnError != nil {
		s.OnError(t, err)
	}
}
