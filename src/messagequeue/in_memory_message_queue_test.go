package messagequeue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
)

func msg(id string) events.ProviderUpdateNormalized {
	return events.ProviderUpdateNormalized{EventID: id}
}

func consume(t *testing.T, q *InMemoryMessageQueue) events.ProviderUpdateNormalized {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	e, err := q.Consume(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestFIFOAndAck(t *testing.T) {
	q := NewInMemoryMessageQueue()
	for _, id := range []string{"a", "b", "c"} {
		_ = q.Publish(context.Background(), msg(id))
	}
	for _, want := range []string{"a", "b", "c"} {
		got := consume(t, q)
		if got.EventID != want {
			t.Fatalf("FIFO: %s, se esperaba %s", got.EventID, want)
		}
		if err := q.Ack(context.Background(), got.EventID); err != nil {
			t.Fatal(err)
		}
	}
	if q.Pending() != 0 {
		t.Fatalf("pendientes: %d", q.Pending())
	}
	if err := q.Ack(context.Background(), "a"); !errors.Is(err, ErrUnknownMessage) {
		t.Fatalf("doble Ack: %v", err)
	}
}

// Un mensaje no confirmado se re-entrega (tolerancia a caída del consumidor, QA-04) y conserva su lugar al frente.
func TestUnackedMessageIsRedelivered(t *testing.T) {
	q := NewInMemoryMessageQueue()
	q.SetRedeliveryTimeout(30 * time.Millisecond)
	_ = q.Publish(context.Background(), msg("a"))
	_ = q.Publish(context.Background(), msg("b"))
	if got := consume(t, q); got.EventID != "a" {
		t.Fatal(got.EventID)
	}
	// El consumidor "cae" sin confirmar "a": tras el plazo vuelve antes que "b".
	time.Sleep(50 * time.Millisecond)
	if got := consume(t, q); got.EventID != "a" {
		t.Fatalf("se esperaba la re-entrega de a, llegó %s", got.EventID)
	}
	_ = q.Ack(context.Background(), "a")
	if got := consume(t, q); got.EventID != "b" {
		t.Fatalf("luego b: %s", got.EventID)
	}
}

// Consume bloquea hasta que hay mensajes, y se re-entrega aunque no se publique nada nuevo.
func TestConsumeBlocksUntilPublishAndRedeliversWhileWaiting(t *testing.T) {
	q := NewInMemoryMessageQueue()
	q.SetRedeliveryTimeout(40 * time.Millisecond)
	go func() { time.Sleep(20 * time.Millisecond); _ = q.Publish(context.Background(), msg("x")) }()
	if got := consume(t, q); got.EventID != "x" {
		t.Fatal(got.EventID)
	}
	if got := consume(t, q); got.EventID != "x" { // espera bloqueado y vence el plazo
		t.Fatalf("re-entrega esperada: %s", got.EventID)
	}
}

func TestConsumeStopsWithContext(t *testing.T) {
	q := NewInMemoryMessageQueue()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := q.Consume(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%v", err)
	}
}

// La cola P2P admite un único consumidor.
func TestSingleConsumer(t *testing.T) {
	q := NewInMemoryMessageQueue()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _, _ = q.Consume(ctx); close(done) }()
	time.Sleep(20 * time.Millisecond) // el primer consumidor queda esperando
	if _, err := q.Consume(context.Background()); !errors.Is(err, ErrSingleConsumer) {
		t.Fatalf("un segundo consumidor debe rechazarse: %v", err)
	}
	cancel()
	<-done
	_ = q.Publish(context.Background(), msg("z"))
	if got := consume(t, q); got.EventID != "z" { // liberado el primero, otro puede consumir
		t.Fatal(got.EventID)
	}
}
