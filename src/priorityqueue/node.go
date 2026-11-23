// Package priorityqueue es el contenedor "Cola de prioridades y Health Monitor BD" (AD §4.5).
package priorityqueue

import (
	"context"
	"sync"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// DefaultReadWorkers es la cantidad de workers extra de solo lectura por partición. [ADR-09][ADR-24]
const DefaultReadWorkers = 4

// PriorityQueueNode [ADR-09][ADR-11] Nodo intermedio: admisión, un carril por partición (cola de lecturas de
// prioridad alta y cola de escrituras de prioridad baja), conmutador de nodo de datos y monitor de heartbeats.
// Paralelismo: cada partición tiene un worker de escrituras (conserva el orden de sus escrituras y despacha
// lecturas primero) más workers extra de solo lectura; particiones distintas se ejecutan al mismo tiempo.
// Riesgo documentado: starvation de escrituras bajo carga sostenida de lecturas, porque el worker de escrituras
// siempre vacía primero la cola de lecturas. Aceptado en ADR-09 (sin envejecimiento de prioridades).
type PriorityQueueNode struct {
	Admission        *OperationAdmission
	Switch           *DataNodeSwitch
	HeartbeatMonitor *DataHeartbeatMonitor
	readWorkers      int
}

// NewPriorityQueueNode ensambla el nodo con sus componentes.
func NewPriorityQueueNode(dataSwitch *DataNodeSwitch, monitor *DataHeartbeatMonitor) *PriorityQueueNode {
	return &PriorityQueueNode{
		Admission:        NewOperationAdmission(dataSwitch),
		Switch:           dataSwitch,
		HeartbeatMonitor: monitor,
		readWorkers:      DefaultReadWorkers,
	}
}

// WithReadWorkers fija los workers extra de solo lectura por partición (0 los desactiva). Antes de Run.
func (n *PriorityQueueNode) WithReadWorkers(count int) *PriorityQueueNode {
	n.readWorkers = count
	return n
}

// QueueDepth devuelve las lecturas y escrituras pendientes en todos los carriles.
func (n *PriorityQueueNode) QueueDepth() (reads, writes int) {
	for _, l := range n.Admission.lanesList() {
		reads += l.reads.Len()
		writes += l.writes.Len()
	}
	return reads, writes
}

// Submit recibe una operación E8 y devuelve su resultado diferido.
func (n *PriorityQueueNode) Submit(ctx context.Context, op events.QueuedOperation) (ports.Future, error) {
	return n.Admission.Admit(ctx, op)
}

// Run ejecuta los workers de cada partición y el monitor de heartbeats de datos. Bloquea hasta que el
// contexto termina.
func (n *PriorityQueueNode) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	if n.HeartbeatMonitor != nil {
		wg.Add(1)
		go func() { defer wg.Done(); _ = n.HeartbeatMonitor.Run(ctx) }()
	}
	for _, l := range n.Admission.lanesList() {
		wg.Add(1)
		go func(l *lane) { defer wg.Done(); n.work(ctx, l, true) }(l)
		for i := 0; i < n.readWorkers; i++ {
			wg.Add(1)
			go func(l *lane) { defer wg.Done(); n.work(ctx, l, false) }(l)
		}
	}
	wg.Wait()
	return nil
}

// work despacha las operaciones del carril. El worker de escrituras (writer=true) toma lecturas y escrituras;
// los workers extra solo toman lecturas. Al cerrar, el worker de escrituras falla lo que quedó pendiente.
func (n *PriorityQueueNode) work(ctx context.Context, l *lane, writer bool) {
	take := func() *pending {
		if writer {
			return l.next()
		}
		return l.reads.pop()
	}
	for {
		if p := take(); p != nil {
			res, err := n.Switch.Dispatch(ctx, p.op)
			p.fut.resolve(res, err)
			continue
		}
		wake := l.wakeReaders
		if writer {
			wake = l.wakeWriter
		}
		select {
		case <-ctx.Done():
			if writer {
				for p := l.next(); p != nil; p = l.next() {
					p.fut.resolve(ports.OperationResult{}, ctx.Err())
				}
			}
			return
		case <-wake:
		}
	}
}
