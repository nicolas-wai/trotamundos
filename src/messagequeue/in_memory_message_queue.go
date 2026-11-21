// Package messagequeue es el contenedor "Cola de mensajes punto a punto" (AD §3.2).
package messagequeue

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// InMemoryMessageQueue [ADR-04] Cola P2P en memoria (canal + mapa de no confirmados con re-entrega).
// Un único consumidor. Lógica en la Fase 4.
type InMemoryMessageQueue struct{}

var _ ports.MessageQueuePort = (*InMemoryMessageQueue)(nil)

// NewInMemoryMessageQueue crea la cola.
func NewInMemoryMessageQueue() *InMemoryMessageQueue { return &InMemoryMessageQueue{} }

// Publish encola E1.
func (*InMemoryMessageQueue) Publish(context.Context, events.ProviderUpdateNormalized) error {
	return apierror.ErrNotImplemented
}

// Consume entrega el siguiente E1 al único consumidor.
func (*InMemoryMessageQueue) Consume(context.Context) (events.ProviderUpdateNormalized, error) {
	return events.ProviderUpdateNormalized{}, apierror.ErrNotImplemented
}

// Ack confirma un mensaje; los no confirmados se re-entregan.
func (*InMemoryMessageQueue) Ack(context.Context, string) error { return apierror.ErrNotImplemented }
