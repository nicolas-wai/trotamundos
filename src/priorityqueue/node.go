// Package priorityqueue es el contenedor "Cola de prioridades y Health Monitor BD" (AD §4.5).
package priorityqueue

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// PriorityQueueNode [ADR-09] Nodo intermedio: admisión, cola de lecturas (prioridad alta),
// cola de escrituras (baja), conmutador de nodo de datos y monitor de heartbeats de datos.
// Riesgo documentado: starvation de escrituras bajo carga sostenida de lecturas.
type PriorityQueueNode struct {
	Admission        *OperationAdmission
	ReadQueue        *ReadQueue
	WriteQueue       *WriteQueue
	Switch           *DataNodeSwitch
	HeartbeatMonitor *DataHeartbeatMonitor
}

// NewPriorityQueueNode ensambla el nodo con sus componentes.
func NewPriorityQueueNode(dataSwitch *DataNodeSwitch, monitor *DataHeartbeatMonitor) *PriorityQueueNode {
	rq, wq := NewReadQueue(), NewWriteQueue()
	return &PriorityQueueNode{
		Admission:        NewOperationAdmission(rq, wq),
		ReadQueue:        rq,
		WriteQueue:       wq,
		Switch:           dataSwitch,
		HeartbeatMonitor: monitor,
	}
}

// Submit recibe una operación E8 y devuelve su resultado diferido.
func (n *PriorityQueueNode) Submit(ctx context.Context, op events.QueuedOperation) (ports.Future, error) {
	return n.Admission.Admit(ctx, op)
}
