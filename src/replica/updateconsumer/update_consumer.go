// Package updateconsumer es el componente "Consumidor de actualizaciones" de la réplica (AD §4.4).
package updateconsumer

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
)

// UpdateConsumer [ADR-04] Único consumidor de la cola P2P: delega E1 en Arreglos de viaje y
// confirma (Ack) solo tras una escritura exitosa. Lógica en la Fase 4.
type UpdateConsumer struct {
	queue        ports.MessageQueuePort
	arrangements application.TravelArrangementsService
}

// NewUpdateConsumer crea el consumidor.
func NewUpdateConsumer(queue ports.MessageQueuePort, arrangements application.TravelArrangementsService) *UpdateConsumer {
	return &UpdateConsumer{queue: queue, arrangements: arrangements}
}

// Run consume la cola hasta que el contexto termina.
func (c *UpdateConsumer) Run(ctx context.Context) error { return apierror.ErrNotImplemented }
