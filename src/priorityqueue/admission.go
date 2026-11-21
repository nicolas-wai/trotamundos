package priorityqueue

import (
	"context"
	"fmt"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Prioridades de E8: número menor = mayor prioridad. [ADR-09]
const (
	PriorityRead  = 1
	PriorityWrite = 2
)

// future es el resultado diferido de una operación encolada (Futuro<Resultado>).
type future struct {
	done chan struct{}
	res  ports.OperationResult
	err  error
}

func newFuture() *future { return &future{done: make(chan struct{})} }

func (f *future) resolve(res ports.OperationResult, err error) {
	f.res, f.err = res, err
	close(f.done)
}

// Await espera el resultado o la cancelación del contexto.
func (f *future) Await(ctx context.Context) (ports.OperationResult, error) {
	select {
	case <-f.done:
		return f.res, f.err
	case <-ctx.Done():
		return ports.OperationResult{}, ctx.Err()
	}
}

// OperationAdmission [ADR-09] "Recepción de operaciones": clasifica E8 por tipo hacia la cola de lecturas o escrituras.
type OperationAdmission struct {
	reads  *ReadQueue
	writes *WriteQueue
	wake   chan struct{}
}

// NewOperationAdmission crea la recepción de operaciones.
func NewOperationAdmission(reads *ReadQueue, writes *WriteQueue) *OperationAdmission {
	return &OperationAdmission{reads: reads, writes: writes, wake: make(chan struct{}, 1)}
}

// Admit encola la operación según su tipo y devuelve su resultado diferido.
func (a *OperationAdmission) Admit(_ context.Context, op events.QueuedOperation) (ports.Future, error) {
	f := newFuture()
	switch op.Tipo {
	case events.OperationRead:
		if op.Prioridad == 0 {
			op.Prioridad = PriorityRead
		}
		a.reads.push(&pending{op: op, fut: f})
	case events.OperationWrite:
		if op.Prioridad == 0 {
			op.Prioridad = PriorityWrite
		}
		a.writes.push(&pending{op: op, fut: f})
	default:
		return nil, fmt.Errorf("tipo de operación inválido: %q", op.Tipo)
	}
	select { // despierta al worker sin bloquear
	case a.wake <- struct{}{}:
	default:
	}
	return f, nil
}

// next entrega la siguiente operación: lecturas primero, luego escrituras; FIFO dentro de cada tipo.
func (a *OperationAdmission) next() *pending {
	if p := a.reads.pop(); p != nil {
		return p
	}
	return a.writes.pop()
}
