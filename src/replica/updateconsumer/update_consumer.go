// Package updateconsumer es el componente "Consumidor de actualizaciones" de la réplica (AD §4.4).
package updateconsumer

import (
	"context"
	"errors"
	"fmt"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
)

// UpdateConsumer [ADR-04] Único consumidor de la cola P2P: delega E1 en Arreglos de viaje y
// confirma (Ack) solo tras una escritura exitosa. Un mensaje inválido (ErrValidation) se confirma y se reporta
// para no re-entregarlo eternamente; cualquier otra falla lo deja sin confirmar y la cola lo re-entrega.
type UpdateConsumer struct {
	queue        ports.MessageQueuePort
	arrangements application.TravelArrangementsService
	// OnError recibe los errores de procesamiento (nil = se ignoran). Pruebas y logging.
	OnError func(events.ProviderUpdateNormalized, error)
}

// NewUpdateConsumer crea el consumidor.
func NewUpdateConsumer(queue ports.MessageQueuePort, arrangements application.TravelArrangementsService) *UpdateConsumer {
	return &UpdateConsumer{queue: queue, arrangements: arrangements}
}

// Run consume la cola hasta que el contexto termina.
func (c *UpdateConsumer) Run(ctx context.Context) error {
	if c.queue == nil {
		return errors.New("consumidor sin cola P2P")
	}
	for {
		update, err := c.queue.Consume(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("consumiendo de la cola P2P: %w", err)
		}
		c.Handle(ctx, update)
	}
}

// Handle aplica un E1 y lo confirma si corresponde.
func (c *UpdateConsumer) Handle(ctx context.Context, update events.ProviderUpdateNormalized) {
	err := c.arrangements.ApplyProviderUpdate(ctx, update)
	if err != nil {
		if c.OnError != nil {
			c.OnError(update, err)
		}
		if !errors.Is(err, application.ErrValidation) {
			return // sin Ack: se re-entrega [ADR-04]
		}
	}
	if err := c.queue.Ack(ctx, update.EventID); err != nil && c.OnError != nil {
		c.OnError(update, err)
	}
}
