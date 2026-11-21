// Package email es el componente "Adapter Email: traduce la API de correo al modelo del puerto [ADR-05][ADR-15][QA-15]".
package email

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
)

// Adapter implementa ports.ExternalProviderPort para proveedores de tipo EMAIL. Lógica en la Fase 4.
type Adapter struct{}

var _ ports.ExternalProviderPort = (*Adapter)(nil)

// NewAdapter crea el adapter.
func NewAdapter() *Adapter { return &Adapter{} }

// ProviderType devuelve el tipo de proveedor que atiende.
func (*Adapter) ProviderType() domain.ProviderType { return domain.ProviderEmail }

// Poll sondea al proveedor externo y traduce su respuesta.
func (*Adapter) Poll(context.Context, ports.ProviderReference, time.Time) ([]events.ProviderUpdateNormalized, error) {
	return nil, apierror.ErrNotImplemented
}
