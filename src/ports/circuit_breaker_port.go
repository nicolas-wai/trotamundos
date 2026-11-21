package ports

import (
	"context"

	integrations "github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
)

// CircuitBreakerPort [ADR-07] Circuit breaker por proveedor con reintentos acotados y espera progresiva.
// Al abrir el circuito publica E2 ProveedorDegradado.
type CircuitBreakerPort interface {
	Execute(ctx context.Context, providerID string, operation func(ctx context.Context) error) error
	State(providerID string) integrations.CircuitState
}
