package ports

import (
	"context"
	"errors"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	integrations "github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
)

// Errores del puerto de proveedores (AD §6.4).
var (
	ErrProviderUnavailable = errors.New("proveedor no disponible")
	ErrInvalidCredential   = errors.New("credencial inválida")
	ErrUnparseableResponse = errors.New("respuesta no parseable")
)

// ProviderReference identifica qué cuenta/proveedor se sondea.
type ProviderReference struct {
	ProviderID string
	TravelerID string
	AccountID  string // cuenta de correo vinculada, si aplica
}

// ExternalProviderPort [ADR-05][ADR-06] Puerto único de proveedores externos (Ports & Adapters).
// Un adapter por tipo implementa este puerto y traduce el formato del proveedor al modelo interno.
type ExternalProviderPort interface {
	ProviderType() integrations.ProviderType
	// Poll sondea al proveedor desde un instante. Errores: ErrProviderUnavailable,
	// ErrInvalidCredential, ErrUnparseableResponse.
	Poll(ctx context.Context, ref ProviderReference, since time.Time) ([]events.ProviderUpdateNormalized, error)
}
