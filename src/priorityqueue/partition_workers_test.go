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

// gateNode bloquea cada operación hasta que el test la libera y registra cuáles están en ejecución a la vez.
type gateNode struct {
	spyNode
	delay   time.Duration
	started chan string
	release chan struct{}
	mu2     sync.Mutex
	running int
	peak    int
	log     []string
}

func (g *gateNode) run(res string) (ports.DataResult, error) {
	g.mu2.Lock()
	g.running++
	if g.running > g.peak {
		g.peak = g.running
	}
	g.log = append(g.log, res)
	g.mu2.Unlock()
	if g.started != nil {
		g.started <- res
	}
	if g.release != nil {
		<-g.release
	}
	if g.delay > 0 {
		time.Sleep(g.delay)
	}
	g.mu2.Lock()
	g.running--
	g.mu2.Unlock()
	return ports.DataResult{}, nil
}
func (g *gateNode) Read(_ context.Context, _ string, q ports.DataQuery) (ports.DataResult, error) {
	return g.run(q.Resource)
}
func (g *gateNode) Write(_ context.Context, _ string, c ports.DataCommand) (ports.DataResult, error) {
	return g.run(c.Resource)
}

func opFor(kind, name, traveler string) events.QueuedOperation {
	return events.QueuedOperation{OperacionID: name, Tipo: kind, IDViajero: traveler, Recurso: name}
}

// [ADR-09/11] Dos operaciones de shards distintos se ejecutan al mismo tiempo (barrera: ambas deben haber
// arrancado antes de que se libere cualquiera).
func TestOperationsOfDifferentPartitionsRunConcurrently(t *testing.T) {
	g := &gateNode{spyNode: spyNode{id: "a"}, started: make(chan string, 4), release: make(chan struct{})}
	node := NewPriorityQueueNode(NewDataNodeSwitch("r", g, &spyNode{id: "p"}), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go node.Run(ctx)
	f1, _ := node.Submit(ctx, opFor(events.OperationWrite, "W-a", "v1"))
	f2, _ := node.Submit(ctx, opFor(events.OperationWrite, "W-b", "v2"))
	for i := 0; i < 2; i++ {
		select {
		case <-g.started:
		case <-time.After(2 * time.Second):
			t.Fatal("las operaciones de shards distintos no se ejecutaron a la vez")
		}
	}
	close(g.release)
	for _, f := range []ports.Future{f1, f2} {
		if _, err := f.Await(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if g.peak < 2 {
		t.Fatalf("concurrencia máxima %d, se esperaba al menos 2", g.peak)
	}
}

// [ADR-09] Dos escrituras del mismo shard se ejecutan en orden y una a la vez; la lectura va antes que la escritura.
func TestSamePartitionKeepsWriteOrderAndReadsFirst(t *testing.T) {
	g := &gateNode{spyNode: spyNode{id: "a"}, delay: 5 * time.Millisecond}
	node := NewPriorityQueueNode(NewDataNodeSwitch("r", g, &spyNode{id: "p"}), nil)
	var futs []ports.Future
	for _, o := range []events.QueuedOperation{
		opFor(events.OperationWrite, "W1", "v1"), opFor(events.OperationWrite, "W2", "v1"),
		opFor(events.OperationWrite, "W3", "v1"), opFor(events.OperationRead, "R1", "v1"),
	} {
		f, _ := node.Submit(context.Background(), o)
		futs = append(futs, f)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go node.Run(ctx)
	for _, f := range futs {
		if _, err := f.Await(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := fmt.Sprint(g.log); got != "[R1 W1 W2 W3]" {
		t.Fatalf("orden %s, se esperaba [R1 W1 W2 W3]", got)
	}
}

// [ADR-24] Con workers extra de lectura, las lecturas de una misma partición (las del ConcurrentCollector)
// no se serializan: el total se acerca a la lectura más lenta y no a la suma.
func TestReadsOfOnePartitionOverlapWithReadWorkers(t *testing.T) {
	const delay = 80 * time.Millisecond
	g := &gateNode{spyNode: spyNode{id: "a"}, delay: delay}
	node := NewPriorityQueueNode(NewDataNodeSwitch("r", g, &spyNode{id: "p"}), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go node.Run(ctx)
	start := time.Now()
	var futs []ports.Future
	for i := 0; i < 4; i++ {
		f, _ := node.Submit(ctx, opFor(events.OperationRead, fmt.Sprintf("R%d", i), "v1"))
		futs = append(futs, f)
	}
	for _, f := range futs {
		if _, err := f.Await(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed > 3*delay {
		t.Fatalf("4 lecturas de %v tardaron %v: se serializaron (suma = %v)", delay, elapsed, 4*delay)
	}
}

// Una operación cuya partición no se resuelve igual llega al nodo de datos, que devuelve el error real.
func TestUnresolvablePartitionStillDispatched(t *testing.T) {
	g := &gateNode{spyNode: spyNode{id: "a"}}
	node := NewPriorityQueueNode(NewDataNodeSwitch("r", g, &spyNode{id: "p"}), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go node.Run(ctx)
	f, err := node.Submit(ctx, opFor(events.OperationRead, "R", ""))
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, c := context.WithTimeout(ctx, 2*time.Second)
	defer c()
	if _, err := f.Await(waitCtx); err != nil {
		t.Fatal(err)
	}
}

// Una escritura con los workers de lectura dormidos no se queda sin atender (la señal de una escritura nunca
// la consume un worker de solo lectura).
func TestWritesAreNotStarvedByIdleReadWorkers(t *testing.T) {
	g := &gateNode{spyNode: spyNode{id: "a"}}
	node := NewPriorityQueueNode(NewDataNodeSwitch("r", g, &spyNode{id: "p"}), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go node.Run(ctx)
	for i := 0; i < 100; i++ {
		f, _ := node.Submit(ctx, opFor(events.OperationWrite, fmt.Sprintf("W%d", i), "v1"))
		waitCtx, c := context.WithTimeout(ctx, 2*time.Second)
		_, err := f.Await(waitCtx)
		c()
		if err != nil {
			t.Fatalf("la escritura %d no se atendió: %v", i, err)
		}
	}
}
