// Package priorityqueueclient es el componente "Cliente de cola de prioridades" de la réplica (AD §4.4).
package priorityqueueclient

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Submitter es el extremo del nodo de cola de prioridades al que la réplica entrega las operaciones E8.
// [Prototipo] En producción sería el transporte de red hacia el nodo intermedio.
type Submitter interface {
	Submit(ctx context.Context, op events.QueuedOperation) (ports.Future, error)
}

// PriorityQueueClient [ADR-09][ADR-11] Encola toda operación de datos de la réplica como E8 con su tipo
// (lectura/escritura). idViajero lo entrega la capa de aplicación tomándolo del token, nunca del body.
type PriorityQueueClient struct {
	submitter Submitter
	now       func() time.Time
}

var (
	_ ports.PriorityQueuePort = (*PriorityQueueClient)(nil)
	_ ports.DataGateway       = (*PriorityQueueClient)(nil)
)

// NewPriorityQueueClient crea el cliente de cola sobre el nodo de cola de prioridades.
func NewPriorityQueueClient(submitter Submitter) *PriorityQueueClient {
	return &PriorityQueueClient{submitter: submitter, now: time.Now}
}

// SetClock inyecta el reloj (pruebas).
func (c *PriorityQueueClient) SetClock(now func() time.Time) { c.now = now }

// Enqueue encola la operación y devuelve su resultado diferido.
func (c *PriorityQueueClient) Enqueue(ctx context.Context, op events.QueuedOperation) (ports.Future, error) {
	if c.submitter == nil {
		return nil, errors.New("cliente de cola sin nodo de cola de prioridades")
	}
	return c.submitter.Submit(ctx, op)
}

// Read arma una operación E8 de LECTURA (prioridad alta), la encola y espera el resultado.
func (c *PriorityQueueClient) Read(ctx context.Context, travelerID, resource string, params map[string]any) (ports.DataResult, error) {
	return c.run(ctx, events.OperationRead, 1, travelerID, resource, params)
}

// Write arma una operación E8 de ESCRITURA (prioridad baja), la encola y espera el resultado.
func (c *PriorityQueueClient) Write(ctx context.Context, travelerID, resource string, params map[string]any) (ports.DataResult, error) {
	return c.run(ctx, events.OperationWrite, 2, travelerID, resource, params)
}

func (c *PriorityQueueClient) run(ctx context.Context, kind string, priority int, travelerID, resource string, params map[string]any) (ports.DataResult, error) {
	fut, err := c.Enqueue(ctx, events.QueuedOperation{
		OperacionID: newUUID(), Tipo: kind, Prioridad: priority, IDViajero: travelerID,
		Recurso: resource, Payload: params, EncoladaEn: c.now().UTC(),
	})
	if err != nil {
		return ports.DataResult{}, err
	}
	return fut.Await(ctx)
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
