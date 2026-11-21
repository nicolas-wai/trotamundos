// Package priorityqueue es el contenedor "Cola de prioridades y Health Monitor BD" (AD §4.5).
package priorityqueue

import (
	"context"
	"sync"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// PriorityQueueNode [ADR-09] Nodo intermedio: admisión, cola de lecturas (prioridad alta),
// cola de escrituras (baja), conmutador de nodo de datos y monitor de heartbeats de datos.
// Riesgo documentado: starvation de escrituras bajo carga sostenida de lecturas, porque el worker
// siempre vacía primero la cola de lecturas. Aceptado en ADR-09 (sin envejecimiento de prioridades).
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

// Run ejecuta el worker de despacho (una gorutina: serializa el acceso al nodo de datos) y el monitor
// de heartbeats de datos. Bloquea hasta que el contexto termina.
func (n *PriorityQueueNode) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	if n.HeartbeatMonitor != nil {
		wg.Add(1)
		go func() { defer wg.Done(); _ = n.HeartbeatMonitor.Run(ctx) }()
	}
	n.work(ctx)
	wg.Wait()
	return nil
}

func (n *PriorityQueueNode) work(ctx context.Context) {
	for {
		if p := n.Admission.next(); p != nil {
			res, err := n.Switch.Dispatch(ctx, p.op)
			p.fut.resolve(res, err)
			continue
		}
		select {
		case <-ctx.Done():
			for p := n.Admission.next(); p != nil; p = n.Admission.next() {
				p.fut.resolve(ports.OperationResult{}, ctx.Err())
			}
			return
		case <-n.Admission.wake:
		}
	}
}
