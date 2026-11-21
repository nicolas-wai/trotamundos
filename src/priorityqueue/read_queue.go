package priorityqueue

import (
	"sync"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
)

// pending es una operación admitida junto con su resultado diferido.
type pending struct {
	op  events.QueuedOperation
	fut *future
}

// fifo es una cola FIFO segura para concurrencia.
type fifo struct {
	mu    sync.Mutex
	items []*pending
}

func (q *fifo) push(p *pending) {
	q.mu.Lock()
	q.items = append(q.items, p)
	q.mu.Unlock()
}

func (q *fifo) pop() *pending {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	p := q.items[0]
	q.items[0] = nil
	q.items = q.items[1:]
	return p
}

func (q *fifo) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// ReadQueue [ADR-09] Cola de lecturas: prioridad alta, FIFO. Se despacha antes que la de escrituras.
type ReadQueue struct{ fifo }

// NewReadQueue crea la cola de lecturas.
func NewReadQueue() *ReadQueue { return &ReadQueue{} }

// Len devuelve la cantidad de lecturas pendientes.
func (q *ReadQueue) Len() int { return q.len() }
