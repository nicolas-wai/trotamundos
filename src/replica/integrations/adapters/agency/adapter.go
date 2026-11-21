// Package agency es el componente "Adapter Agencias: traduce la API de agencias al modelo del puerto [ADR-05][QA-14]".
package agency

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
)

// Adapter implementa ports.ExternalProviderPort para proveedores de tipo AGENCIA. Lógica en la Fase 4.
type Adapter struct{}

var _ ports.ExternalProviderPort = (*Adapter)(nil)

// NewAdapter crea el adapter.
func NewAdapter() *Adapter { return &Adapter{} }

// ProviderType devuelve el tipo de proveedor que atiende.
func (*Adapter) ProviderType() domain.ProviderType { return domain.ProviderAgency }

// Poll sondea al proveedor externo y traduce su respuesta.
func (*Adapter) Poll(context.Context, ports.ProviderReference, time.Time) ([]events.ProviderUpdateNormalized, error) {
	return nil, apierror.ErrNotImplemented
}
