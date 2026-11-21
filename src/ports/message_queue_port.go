package ports

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
)

// MessageQueuePort [ADR-04] Cola de mensajes punto a punto entre el broker y Arreglos de viaje (E1).
// Un único consumidor; sin invocación directa entre Integraciones y Arreglos de viaje.
type MessageQueuePort interface {
	Publish(ctx context.Context, event events.ProviderUpdateNormalized) error
	Consume(ctx context.Context) (events.ProviderUpdateNormalized, error)
	// Ack confirma el mensaje; los no confirmados se re-entregan.
	Ack(ctx context.Context, eventID string) error
}
