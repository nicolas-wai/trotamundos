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

// fallbackLane atiende las operaciones cuya partición no se pudo resolver (recurso desconocido, idViajero
// vacío): el nodo de datos devuelve el error real al despacharlas.
const fallbackLane = ""

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

// lane [ADR-09][ADR-11] Carril de una partición: su propio par de colas de lectura (prioridad 1) y escritura
// (prioridad 2). Lo atienden un worker de escrituras (que despacha lecturas primero) y workers extra de solo lectura.
type lane struct {
	name   string
	reads  *ReadQueue
	writes *WriteQueue
	// Un token por operación encolada (con holgura) en cada canal: así un worker de solo lectura nunca se lleva
	// la señal de una escritura. Las lecturas despiertan a ambos canales; las escrituras, solo al de escrituras.
	wakeWriter  chan struct{}
	wakeReaders chan struct{}
}

func newLane(name string) *lane {
	return &lane{name: name, reads: NewReadQueue(), writes: NewWriteQueue(),
		wakeWriter: make(chan struct{}, 32), wakeReaders: make(chan struct{}, 32)}
}

func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// next entrega la siguiente operación: lecturas primero, luego escrituras; FIFO dentro de cada tipo.
func (l *lane) next() *pending {
	if p := l.reads.pop(); p != nil {
		return p
	}
	return l.writes.pop()
}

// OperationAdmission [ADR-09][ADR-11] "Recepción de operaciones": clasifica E8 por partición y por tipo hacia
// la cola de lecturas o de escrituras del carril de la partición. Los carriles se fijan al construir.
type OperationAdmission struct {
	resolver ports.PartitionResolver
	lanes    map[string]*lane
	order    []*lane
}

// NewOperationAdmission crea la recepción con un carril por partición del resolver más el carril de respaldo.
func NewOperationAdmission(resolver ports.PartitionResolver) *OperationAdmission {
	a := &OperationAdmission{resolver: resolver, lanes: map[string]*lane{}}
	for _, name := range append(resolver.Partitions(), fallbackLane) {
		l := newLane(name)
		a.lanes[name] = l
		a.order = append(a.order, l)
	}
	return a
}

// laneFor devuelve el carril de la operación (el de respaldo si la partición no se resuelve).
func (a *OperationAdmission) laneFor(op events.QueuedOperation) *lane {
	partition, err := a.resolver.PartitionOf(op)
	if err != nil {
		return a.lanes[fallbackLane]
	}
	if l, ok := a.lanes[partition]; ok {
		return l
	}
	return a.lanes[fallbackLane]
}

// Admit encola la operación según su partición y su tipo y devuelve su resultado diferido.
func (a *OperationAdmission) Admit(_ context.Context, op events.QueuedOperation) (ports.Future, error) {
	f := newFuture()
	switch op.Tipo {
	case events.OperationRead:
		if op.Prioridad == 0 {
			op.Prioridad = PriorityRead
		}
		l := a.laneFor(op)
		l.reads.push(&pending{op: op, fut: f})
		signal(l.wakeReaders)
		signal(l.wakeWriter)
	case events.OperationWrite:
		if op.Prioridad == 0 {
			op.Prioridad = PriorityWrite
		}
		l := a.laneFor(op)
		l.writes.push(&pending{op: op, fut: f})
		signal(l.wakeWriter)
	default:
		return nil, fmt.Errorf("tipo de operación inválido: %q", op.Tipo)
	}
	return f, nil
}

// lanesList devuelve los carriles (uno por partición y el de respaldo).
func (a *OperationAdmission) lanesList() []*lane { return a.order }
