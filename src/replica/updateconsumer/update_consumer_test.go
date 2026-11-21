package updateconsumer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/messagequeue"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
)

// fakeService aplica E1 según el resultado programado para cada eventId.
type fakeService struct {
	application.UnimplementedTravelArrangementsService
	mu      sync.Mutex
	results map[string][]error // por eventId: resultados sucesivos (el último se repite)
	applied []string
}

func (f *fakeService) ApplyProviderUpdate(_ context.Context, u events.ProviderUpdateNormalized) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	rs := f.results[u.EventID]
	err := rs[0]
	if len(rs) > 1 {
		f.results[u.EventID] = rs[1:]
	}
	if err == nil {
		f.applied = append(f.applied, u.EventID)
	}
	return err
}

func (f *fakeService) appliedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.applied...)
}

func run(t *testing.T, q *messagequeue.InMemoryMessageQueue, svc *fakeService) (*UpdateConsumer, context.CancelFunc) {
	t.Helper()
	c := NewUpdateConsumer(q, svc)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = c.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return c, cancel
}

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timeout: %s", msg)
}

// Confirma (Ack) solo tras una escritura exitosa; ante una falla transitoria no confirma y la cola re-entrega (ADR-04).
func TestAcksOnlyAfterSuccessAndRedeliversOnTransientFailure(t *testing.T) {
	q := messagequeue.NewInMemoryMessageQueue()
	q.SetRedeliveryTimeout(30 * time.Millisecond)
	svc := &fakeService{results: map[string][]error{
		"ok":        {nil},
		"transient": {errors.New("nodo de datos caído"), nil}, // falla una vez, luego funciona
	}}
	run(t, q, svc)
	_ = q.Publish(context.Background(), events.ProviderUpdateNormalized{EventID: "transient"})
	_ = q.Publish(context.Background(), events.ProviderUpdateNormalized{EventID: "ok"})

	waitFor(t, func() bool { return len(svc.appliedIDs()) == 2 }, "ambos aplicados (el transitorio tras la re-entrega)")
	waitFor(t, func() bool { return q.Pending() == 0 }, "todo confirmado")
	if got := svc.appliedIDs(); got[0] != "ok" || got[1] != "transient" {
		t.Fatalf("orden de aplicación: %v", got)
	}
}

// Un mensaje inválido (ErrValidation) se confirma y reporta para no re-entregarse eternamente.
func TestInvalidMessageIsAckedAndReported(t *testing.T) {
	q := messagequeue.NewInMemoryMessageQueue()
	q.SetRedeliveryTimeout(20 * time.Millisecond)
	svc := &fakeService{results: map[string][]error{"bad": {fmt.Errorf("%w: tipo", application.ErrValidation)}}}
	c, _ := run(t, q, svc)
	var mu sync.Mutex
	var reported []string
	c.OnError = func(u events.ProviderUpdateNormalized, err error) {
		mu.Lock()
		reported = append(reported, u.EventID)
		mu.Unlock()
	}
	_ = q.Publish(context.Background(), events.ProviderUpdateNormalized{EventID: "bad"})
	waitFor(t, func() bool { return q.Pending() == 0 }, "mensaje inválido confirmado")
	time.Sleep(60 * time.Millisecond) // no debe volver
	mu.Lock()
	defer mu.Unlock()
	if len(reported) != 1 || q.Pending() != 0 {
		t.Fatalf("reportes=%v pendientes=%d", reported, q.Pending())
	}
}

func TestRunWithoutQueueFails(t *testing.T) {
	if err := NewUpdateConsumer(nil, &fakeService{}).Run(context.Background()); err == nil {
		t.Fatal("sin cola debe fallar")
	}
}
