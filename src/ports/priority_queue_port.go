package ports

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
)

// OperationResult es el resultado de una operación encolada.
type OperationResult = DataResult

// Future es el resultado diferido de una operación encolada (Futuro<Resultado>).
type Future interface {
	Await(ctx context.Context) (OperationResult, error)
}

// PriorityQueuePort [ADR-09] Cola de prioridades: lecturas antes que escrituras, FIFO dentro de cada tipo.
// Riesgo conocido: starvation de escrituras bajo carga sostenida de lecturas.
type PriorityQueuePort interface {
	Enqueue(ctx context.Context, op events.QueuedOperation) (Future, error)
}
