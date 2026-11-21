package priorityqueue

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// OperationAdmission [ADR-09] "Recepción de operaciones": clasifica E8 por tipo hacia la cola de lecturas o escrituras.
type OperationAdmission struct {
	reads  *ReadQueue
	writes *WriteQueue
}

// NewOperationAdmission crea la recepción de operaciones.
func NewOperationAdmission(reads *ReadQueue, writes *WriteQueue) *OperationAdmission {
	return &OperationAdmission{reads: reads, writes: writes}
}

// Admit encola la operación según su tipo.
func (a *OperationAdmission) Admit(context.Context, events.QueuedOperation) (ports.Future, error) {
	return nil, apierror.ErrNotImplemented
}
