// Package messagequeue es el contenedor "Cola de mensajes punto a punto" (AD §3.2).
package messagequeue

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Errores de la cola P2P.
var (
	// ErrSingleConsumer: ya hay un consumidor activo; la cola P2P admite uno solo. [ADR-04]
	ErrSingleConsumer = errors.New("la cola P2P admite un único consumidor")
	// ErrUnknownMessage: Ack de un mensaje que no está pendiente de confirmación.
	ErrUnknownMessage = errors.New("mensaje desconocido o ya confirmado")
)

// DefaultRedeliveryTimeout es el plazo para confirmar un mensaje antes de que se re-entregue.
const DefaultRedeliveryTimeout = 30 * time.Second

// InMemoryMessageQueue [ADR-04] Cola P2P en memoria: FIFO de mensajes listos + mapa de no confirmados.
// Un único consumidor. Un mensaje entregado y no confirmado (Ack) dentro del plazo vuelve al frente de la cola
// y se re-entrega: si el consumidor cae a mitad de camino no se pierde nada (QA-04). Consistencia eventual.
type InMemoryMessageQueue struct {
	mu        sync.Mutex
	ready     []events.ProviderUpdateNormalized
	inflight  map[string]inflightMessage
	consuming bool
	wake      chan struct{}
	timeout   time.Duration
}

type inflightMessage struct {
	event       events.ProviderUpdateNormalized
	deliveredAt time.Time
}

var _ ports.MessageQueuePort = (*InMemoryMessageQueue)(nil)

// NewInMemoryMessageQueue crea la cola con el plazo de re-entrega por defecto.
func NewInMemoryMessageQueue() *InMemoryMessageQueue {
	return &InMemoryMessageQueue{inflight: map[string]inflightMessage{}, wake: make(chan struct{}, 1), timeout: DefaultRedeliveryTimeout}
}

// SetRedeliveryTimeout cambia el plazo de re-entrega (pruebas).
func (q *InMemoryMessageQueue) SetRedeliveryTimeout(d time.Duration) {
	q.mu.Lock()
	q.timeout = d
	q.mu.Unlock()
}

// Publish encola E1; los mensajes persisten aunque el consumidor esté caído.
func (q *InMemoryMessageQueue) Publish(ctx context.Context, e events.ProviderUpdateNormalized) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	q.mu.Lock()
	q.ready = append(q.ready, e)
	q.mu.Unlock()
	q.signal()
	return nil
}

// Consume entrega el siguiente E1 al único consumidor y bloquea hasta que haya uno o venza el contexto.
// Una segunda llamada concurrente devuelve ErrSingleConsumer.
func (q *InMemoryMessageQueue) Consume(ctx context.Context) (events.ProviderUpdateNormalized, error) {
	q.mu.Lock()
	if q.consuming {
		q.mu.Unlock()
		return events.ProviderUpdateNormalized{}, ErrSingleConsumer
	}
	q.consuming = true
	q.mu.Unlock()
	defer func() { q.mu.Lock(); q.consuming = false; q.mu.Unlock() }()

	for {
		q.mu.Lock()
		q.requeueExpiredLocked(time.Now())
		if len(q.ready) > 0 {
			e := q.ready[0]
			q.ready = q.ready[1:]
			q.inflight[e.EventID] = inflightMessage{event: e, deliveredAt: time.Now()}
			q.mu.Unlock()
			return e, nil
		}
		wait := q.nextExpiryLocked()
		q.mu.Unlock()

		var timer <-chan time.Time
		if wait > 0 {
			t := time.NewTimer(wait)
			defer t.Stop()
			timer = t.C
		}
		select {
		case <-ctx.Done():
			return events.ProviderUpdateNormalized{}, ctx.Err()
		case <-q.wake:
		case <-timer:
		}
	}
}

// Ack confirma un mensaje: deja de re-entregarse.
func (q *InMemoryMessageQueue) Ack(_ context.Context, eventID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.inflight[eventID]; !ok {
		return ErrUnknownMessage
	}
	delete(q.inflight, eventID)
	return nil
}

// Pending devuelve cuántos mensajes hay listos más los entregados sin confirmar.
func (q *InMemoryMessageQueue) Pending() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.ready) + len(q.inflight)
}

// requeueExpiredLocked devuelve al frente de la cola los mensajes cuyo plazo de confirmación venció.
func (q *InMemoryMessageQueue) requeueExpiredLocked(now time.Time) {
	var expired []events.ProviderUpdateNormalized
	for id, m := range q.inflight {
		if now.Sub(m.deliveredAt) >= q.timeout {
			expired = append(expired, m.event)
			delete(q.inflight, id)
		}
	}
	if len(expired) > 0 {
		q.ready = append(expired, q.ready...)
	}
}

// nextExpiryLocked devuelve en cuánto vence el próximo mensaje sin confirmar (0 si no hay).
func (q *InMemoryMessageQueue) nextExpiryLocked() time.Duration {
	var next time.Duration
	for _, m := range q.inflight {
		left := q.timeout - time.Since(m.deliveredAt)
		if left < time.Millisecond {
			left = time.Millisecond
		}
		if next == 0 || left < next {
			next = left
		}
	}
	return next
}

func (q *InMemoryMessageQueue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}
