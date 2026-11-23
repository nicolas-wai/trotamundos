package integrations

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/stubs"
)

// Con reloj inyectado: lidera la réplica viva de menor posición y el liderazgo se cede y se recupera.
func TestLeaderIsLowestAliveReplica(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hb := stubs.NewInMemoryHeartbeatChannel(10*time.Second, 3)
	hb.SetClock(func() time.Time { return now })
	ids := []string{"r1", "r2", "r3"}
	hb.Track(ids...)
	beat := func(id string) { _ = hb.Emit(context.Background(), events.Heartbeat{NodoID: id}) }
	el := func(id string) *IntegrationsLeaderElector {
		return NewIntegrationsLeaderElector(id, ids, hb, time.Millisecond)
	}

	if !el("r1").IsLeader() || el("r2").IsLeader() {
		t.Fatal("al inicio lidera r1")
	}
	now = now.Add(31 * time.Second) // r1 no late; r2 y r3 sí
	beat("r2")
	beat("r3")
	if el("r1").IsLeader() || !el("r2").IsLeader() || el("r3").IsLeader() {
		t.Fatalf("lidera r2: %q", el("r1").LeaderID())
	}
	beat("r1") // r1 vuelve
	if !el("r1").IsLeader() || el("r2").IsLeader() {
		t.Fatal("r1 retoma el liderazgo")
	}
	now = now.Add(31 * time.Second) // nadie late
	if el("r1").LeaderID() != "" {
		t.Fatal("sin réplicas vivas no hay líder")
	}
}

// Run arranca start al volverse líder, cancela su contexto al dejar de serlo y nunca solapa dos ejecuciones.
func TestRunStartsAndStopsOnLeadershipChange(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	hb := stubs.NewInMemoryHeartbeatChannel(10*time.Second, 3)
	hb.SetClock(func() time.Time { mu.Lock(); defer mu.Unlock(); return now })
	hb.Track("r1", "r2")
	advance := func(d time.Duration) { mu.Lock(); now = now.Add(d); mu.Unlock() }
	beat := func(id string) { _ = hb.Emit(context.Background(), events.Heartbeat{NodoID: id}) }

	// start imita al consumidor de la cola P2P: admite uno solo (como ErrSingleConsumer) y quien llega mientras
	// el anterior sigue activo reintenta hasta que libere. Así el tramo de solapamiento del relevo queda cubierto.
	var running, maxRunning, starts atomic.Int32
	errSingle := errors.New("un único consumidor")
	start := func(ctx context.Context) {
		RunWithRetry(ctx, time.Millisecond, func(ctx context.Context) error {
			if !running.CompareAndSwap(0, 1) {
				return errSingle
			}
			starts.Add(1)
			maxRunning.Store(1)
			<-ctx.Done()
			running.Store(0)
			return nil
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, id := range []string{"r1", "r2"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			NewIntegrationsLeaderElector(id, []string{"r1", "r2"}, hb, time.Millisecond).Run(ctx, start)
		}()
	}
	waitFor := func(cond func() bool, msg string) {
		t.Helper()
		for dl := time.Now().Add(2 * time.Second); time.Now().Before(dl); time.Sleep(time.Millisecond) {
			if cond() {
				return
			}
		}
		t.Fatal("timeout: " + msg)
	}
	waitFor(func() bool { return running.Load() == 1 }, "arranca la líder")
	advance(31 * time.Second) // r1 deja de latir
	beat("r2")
	waitFor(func() bool { return starts.Load() == 2 && running.Load() == 1 }, "r2 releva y r1 cede")
	beat("r1") // r1 vuelve: r2 cede y r1 retoma
	waitFor(func() bool { return starts.Load() == 3 && running.Load() == 1 }, "r1 retoma")
	cancel()
	wg.Wait()
	if running.Load() != 0 {
		t.Fatal("al terminar el contexto nada debe seguir corriendo")
	}
	if maxRunning.Load() != 1 {
		t.Fatalf("hubo %d ejecuciones simultáneas", maxRunning.Load())
	}
}
