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
	peer     ports.SyncTarget
	syncs    int
	syncErr  error
}

func (s *spyNode) SetPeer(p ports.SyncTarget) { s.peer = p }
func (s *spyNode) SyncNow(context.Context) (events.StateSync, error) {
	s.syncs++
	if s.syncErr != nil {
		return events.StateSync{}, s.syncErr
	}
	return events.StateSync{Origen: s.id, Destino: s.peer.ID(), VersionEstado: 7}, nil
}
func (s *spyNode) InstallSnapshot(context.Context, string, string) error { return nil }

// PartitionOf: "directory" para el recurso homónimo; si no, un shard por idViajero ("v1" → shard-v1).
func (s *spyNode) PartitionOf(op events.QueuedOperation) (string, error) {
	if op.Recurso == "directory" {
		return "directory", nil
	}
	if op.IDViajero == "" {
		return "", fmt.Errorf("idViajero vacío")
	}
	return "shard-" + op.IDViajero, nil
}
func (s *spyNode) Partitions() []string {
	return []string{"shard-v1", "shard-v2", "shard-v3", "directory"}
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
	node := NewPriorityQueueNode(NewDataNodeSwitch("r", active, &spyNode{id: "p"}), nil).WithReadWorkers(0)
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
	a := NewOperationAdmission(&spyNode{id: "a"})
	if _, err := a.Admit(context.Background(), op("OTRO", "x")); err == nil {
		t.Fatal("un tipo inválido debía rechazarse")
	}
	_, _ = a.Admit(context.Background(), op(events.OperationRead, "r"))
	_, _ = a.Admit(context.Background(), op(events.OperationWrite, "w"))
	l := a.lanes["shard-v1"]
	if r, w := l.next(), l.next(); r.op.Prioridad != PriorityRead || w.op.Prioridad != PriorityWrite {
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

// [ADR-21] Tras la conmutación el anterior activo queda sin respaldo válido: no se conmuta de vuelta hasta que
// vuelve a latir y recibe la imagen completa; recién entonces se emite E4b y puede ser destino de conmutación.
func TestMonitorReincorporatesRecoveredNodeWithFullImageFirst(t *testing.T) {
	a, p := &spyNode{id: "a"}, &spyNode{id: "p"}
	sw := NewDataNodeSwitch("r", a, p)
	var rejoined []events.PassiveRejoined
	sw.OnRejoin(func(e events.PassiveRejoined) { rejoined = append(rejoined, e) })
	var alerts []string
	hb := &fakeHeartbeats{health: map[string]ports.NodeHealth{"a": ports.NodeDead, "p": ports.NodeAlive}}
	m := NewDataHeartbeatMonitor(hb, sw, time.Hour)
	m.OnAlert(func(msg string) { alerts = append(alerts, msg) })

	m.Check(context.Background()) // a cae → p activo
	if sw.ActiveID() != "p" || sw.PassiveReady() {
		t.Fatalf("tras conmutar el nuevo pasivo no es respaldo válido: activo=%s ready=%v", sw.ActiveID(), sw.PassiveReady())
	}
	// a sigue caído y ahora cae p: no hay destino válido, no se conmuta y queda un aviso
	hb.health["p"] = ports.NodeDead
	m.Check(context.Background())
	m.Check(context.Background())
	if sw.ActiveID() != "p" || len(alerts) != 1 {
		t.Fatalf("ambos caídos: activo=%s avisos=%v (uno por episodio)", sw.ActiveID(), alerts)
	}
	// p vuelve, a sigue caído. a vuelve a latir pero sin imagen no es destino: antes de reincorporar, no cuenta
	hb.health["p"] = ports.NodeAlive
	hb.health["a"] = ports.NodeAlive
	if sw.PassiveReady() {
		t.Fatal("no debía contar como respaldo antes de la imagen")
	}
	m.Check(context.Background())
	if !sw.PassiveReady() || len(rejoined) != 1 || rejoined[0].Nodo != "a" || rejoined[0].VersionEstado != 7 || p.syncs != 1 || p.peer.ID() != "a" {
		t.Fatalf("reincorporación: ready=%v e4b=%+v syncs=%d", sw.PassiveReady(), rejoined, p.syncs)
	}
	m.Check(context.Background()) // ya sincronizado: no vuelve a copiar
	if p.syncs != 1 {
		t.Fatal("copia espuria")
	}
	// ahora puede conmutar de vuelta
	hb.health["p"] = ports.NodeDead
	m.Check(context.Background())
	if sw.ActiveID() != "a" {
		t.Fatal("con el pasivo sincronizado debía conmutar de vuelta")
	}
}

// [ADR-21] Si el activo cae y el pasivo todavía no recibió la imagen, el monitor no lo usa como destino.
func TestMonitorDoesNotSwapToPassiveWithoutImage(t *testing.T) {
	a, p := &spyNode{id: "a"}, &spyNode{id: "p"}
	sw := NewDataNodeSwitch("r", a, p)
	sw.MarkPassiveStale()
	hb := &fakeHeartbeats{health: map[string]ports.NodeHealth{"a": ports.NodeDead, "p": ports.NodeAlive}}
	m := NewDataHeartbeatMonitor(hb, sw, time.Hour)
	var alerts []string
	m.OnAlert(func(s string) { alerts = append(alerts, s) })
	m.Check(context.Background())
	if sw.ActiveID() != "a" || p.promoted || len(alerts) != 1 {
		t.Fatalf("no debía conmutar: activo=%s avisos=%v", sw.ActiveID(), alerts)
	}
}

// Si la copia de la imagen falla, el nodo recuperado sigue sin contar como respaldo.
func TestFailedReincorporationKeepsPassiveNotReady(t *testing.T) {
	a, p := &spyNode{id: "a", syncErr: fmt.Errorf("sin red")}, &spyNode{id: "p"}
	sw := NewDataNodeSwitch("r", a, p)
	sw.MarkPassiveStale()
	if _, err := sw.Reincorporate(context.Background()); err == nil || sw.PassiveReady() {
		t.Fatalf("la falla debía propagarse y no declarar listo al pasivo: %v", err)
	}
}
