// Package scheduler es el componente "Planificador de sondeo" de Integraciones externas (AD §4.4).
package scheduler

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// PollingScheduler [ADR-05][QA-01] Dispara sondeos cada <= 2 min sobre cuentas y proveedores,
// pasando por el circuit breaker; omite proveedores con circuito abierto. Lógica en la Fase 4.
type PollingScheduler struct {
	interval time.Duration
	breaker  ports.CircuitBreakerPort
}

// NewPollingScheduler crea el planificador.
func NewPollingScheduler(interval time.Duration, breaker ports.CircuitBreakerPort) *PollingScheduler {
	return &PollingScheduler{interval: interval, breaker: breaker}
}

// Run sondea periódicamente hasta que el contexto termina.
func (s *PollingScheduler) Run(ctx context.Context) error { return apierror.ErrNotImplemented }
