// Package circuitbreaker es el componente "Circuit breaker y reintentos acotados" (AD §4.4).
package circuitbreaker

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
)

// ProviderCircuitBreaker [ADR-07] Un circuito por proveedor (CERRADO/ABIERTO/SEMIABIERTO), N reintentos
// con espera progresiva; al abrir publica E2 ProveedorDegradado en el bus de eventos. Lógica en la Fase 4.
type ProviderCircuitBreaker struct {
	bus ports.EventBusPort
}

var _ ports.CircuitBreakerPort = (*ProviderCircuitBreaker)(nil)

// NewProviderCircuitBreaker crea el circuit breaker.
func NewProviderCircuitBreaker(bus ports.EventBusPort) *ProviderCircuitBreaker {
	return &ProviderCircuitBreaker{bus: bus}
}

// Execute ejecuta la operación bajo la protección del circuito del proveedor.
func (*ProviderCircuitBreaker) Execute(context.Context, string, func(context.Context) error) error {
	return apierror.ErrNotImplemented
}

// State devuelve el estado del circuito del proveedor.
func (*ProviderCircuitBreaker) State(string) domain.CircuitState { return domain.CircuitClosed }
