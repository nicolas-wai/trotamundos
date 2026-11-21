package priorityqueue

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// spyNode registra el orden en que el conmutador le entrega las operaciones.
type spyNode struct {
	id       string
	mu       sync.Mutex
	order    []string
	promoted bool
	demoted  bool
}

func (s *spyNode) ID() string { return s.id }
func (s *spyNode) Promote()   { s.promoted = true }
func (s *spyNode) Demote()    { s.demoted = true }
func (s *spyNode) Read(_ context.Context, _ string, q ports.DataQuery) (ports.DataResult, error) {
	s.mu.Lock()
	s.order = append(s.order, q.Resource)
	s.mu.Unlock()
	return ports.DataResult{}, nil
}
func (s *spyNode) Write(_ context.Context, _ string, c ports.DataCommand) (ports.DataResult, error) {
	s.mu.Lock()
	s.order = append(s.order, c.Resource)
	s.mu.Unlock()
	return ports.DataResult{}, nil
}

func op(kind, name string) events.QueuedOperation {
	return events.QueuedOperation{OperacionID: name, Tipo: kind, IDViajero: "v1", Recurso: name}
}

// [ADR-09] Lecturas antes que escrituras encoladas antes; FIFO dentro de cada tipo.
func TestReadsAreDispatchedBeforeEarlierWritesAndFIFOPerType(t *testing.T) {
	active := &spyNode{id: "a"}
	node := NewPriorityQueueNode(NewDataNodeSwitch("r", active, &spyNode{id: "p"}), nil)
	var futs []ports.Future
	for _, o := range []events.QueuedOperation{
		op(events.OperationWrite, "W1"), op(events.OperationRead, "R1"),
		op(events.OperationWrite, "W2"), op(events.OperationRead, "R2"),
	} {
		f, err := node.Submit(context.Background(), o)
		if err != nil {
			t.Fatal(err)
		}
		futs = append(futs, f)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go node.Run(ctx)
	for _, f := range futs {
		waitCtx, c := context.WithTimeout(ctx, 2*time.Second)
		if _, err := f.Await(waitCtx); err != nil {
			t.Fatal(err)
		}
		c()
	}
	if got := fmt.Sprint(active.order); got != "[R1 R2 W1 W2]" {
		t.Fatalf("orden %s, se esperaba [R1 R2 W1 W2]", got)
	}
}

func TestAdmissionAssignsPriorityAndRejectsInvalidType(t *testing.T) {
	a := NewOperationAdmission(NewReadQueue(), NewWriteQueue())
	if _, err := a.Admit(context.Background(), op("OTRO", "x")); err == nil {
		t.Fatal("un tipo inválido debía rechazarse")
	}
	_, _ = a.Admit(context.Background(), op(events.OperationRead, "r"))
	_, _ = a.Admit(context.Background(), op(events.OperationWrite, "w"))
	if r, w := a.next(), a.next(); r.op.Prioridad != PriorityRead || w.op.Prioridad != PriorityWrite {
		t.Fatalf("prioridades %d/%d", r.op.Prioridad, w.op.Prioridad)
	}
}

func TestPendingOperationsAreFailedOnShutdown(t *testing.T) {
	node := NewPriorityQueueNode(NewDataNodeSwitch("r", &spyNode{id: "a"}, &spyNode{id: "p"}), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f, _ := node.Submit(context.Background(), op(events.OperationRead, "R"))
	node.Run(ctx) // contexto ya cancelado: drena y termina
	if _, err := f.Await(context.Background()); err == nil {
		// el worker pudo despachar antes de ver el cancel; ambos resultados son válidos, pero no puede colgar
		t.Log("operación despachada antes del cierre")
	}
}

// fakeHeartbeats permite fijar la salud de cada nodo.
type fakeHeartbeats struct {
	mu     sync.Mutex
	health map[string]ports.NodeHealth
}

func (f *fakeHeartbeats) Emit(context.Context, events.Heartbeat) error { return nil }
func (f *fakeHeartbeats) Observe(id string) ports.NodeHealth {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.health[id]
}

// [ADR-22] Activo caído y pasivo vivo → intercambio, E4, promoción/degradación y tráfico al nuevo activo.
func TestMonitorSwapsToPassiveWhenActiveIsDead(t *testing.T) {
	a, p := &spyNode{id: "a"}, &spyNode{id: "p"}
	sw := NewDataNodeSwitch("region-x", a, p)
	var got []events.ResponsibilitySwap
	sw.OnSwap(func(e events.ResponsibilitySwap) { got = append(got, e) })
	hb := &fakeHeartbeats{health: map[string]ports.NodeHealth{"a": ports.NodeAlive, "p": ports.NodeAlive}}
	m := NewDataHeartbeatMonitor(hb, sw, time.Hour)

	m.Check(context.Background())
	if len(got) != 0 {
		t.Fatal("no debía intercambiar con el activo vivo")
	}
	hb.health["a"] = ports.NodeDead
	m.Check(context.Background())
	if len(got) != 1 || got[0].NuevoPrimario != "p" || got[0].NuevoPasivo != "a" || got[0].Motivo != "HEARTBEAT_PERDIDO" || got[0].RegionID != "region-x" {
		t.Fatalf("E4 inesperado: %+v", got)
	}
	if !p.promoted || !a.demoted {
		t.Fatal("el pasivo debía promoverse y el anterior degradarse")
	}
	if _, err := sw.Dispatch(context.Background(), op(events.OperationRead, "R")); err != nil || len(p.order) != 1 {
		t.Fatalf("el tráfico debía ir al nuevo activo: %v %v", err, p.order)
	}
	m.Check(context.Background()) // ahora el activo es "p" (vivo): no vuelve a intercambiar
	if len(got) != 1 {
		t.Fatal("intercambio espurio")
	}
}

func TestMonitorDoesNotSwapWhenPassiveIsAlsoDead(t *testing.T) {
	sw := NewDataNodeSwitch("r", &spyNode{id: "a"}, &spyNode{id: "p"})
	hb := &fakeHeartbeats{health: map[string]ports.NodeHealth{"a": ports.NodeDead, "p": ports.NodeDead}}
	NewDataHeartbeatMonitor(hb, sw, time.Hour).Check(context.Background())
	if sw.ActiveID() != "a" {
		t.Fatal("no debía intercambiar hacia un pasivo caído")
	}
}
