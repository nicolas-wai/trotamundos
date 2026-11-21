// Package broker es el componente "Broker (middleware con normalización)" (AD §4.4).
package broker

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Broker [ADR-06][ADR-04] Recibe datos del puerto único, normaliza a E1 (claveDeduplicacion, generadoEnFuente,
// recibidoEn, origenRemitente) y publica en la cola P2P. La lógica nunca recibe datos sin formatear.
type Broker struct {
	queue ports.MessageQueuePort
}

// NewBroker crea el broker.
func NewBroker(queue ports.MessageQueuePort) *Broker { return &Broker{queue: queue} }

// Ingest normaliza y encola las actualizaciones recibidas de un adapter.
func (b *Broker) Ingest(ctx context.Context, updates []events.ProviderUpdateNormalized) error {
	return apierror.ErrNotImplemented
}
