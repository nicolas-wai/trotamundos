// Package priorityqueueclient es el componente "Cliente de cola de prioridades" de la réplica (AD §4.4).
package priorityqueueclient

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Client [ADR-09][ADR-11] Encola toda operación de datos de la réplica como E8 con su tipo
// (lectura/escritura). idViajero siempre se toma del token, nunca del body. Lógica en la Fase 2.
type Client struct{}

var _ ports.PriorityQueuePort = (*Client)(nil)

// NewClient crea el cliente de cola.
func NewClient() *Client { return &Client{} }

// Enqueue encola la operación y devuelve su resultado diferido.
func (*Client) Enqueue(context.Context, events.QueuedOperation) (ports.Future, error) {
	return nil, apierror.ErrNotImplemented
}
