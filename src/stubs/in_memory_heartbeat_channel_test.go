package stubs

import (
	"context"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// [ADR-22] CAIDO tras 3 pulsos perdidos de 10 s.
func TestNodeIsDeadAfterThreeMissedBeats(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewInMemoryHeartbeatChannel(10*time.Second, 3)
	c.SetClock(func() time.Time { return now })

	if c.Observe("n") != ports.NodeDead {
		t.Fatal("un nodo desconocido no está vivo")
	}
	c.Track("n")
	if c.Observe("n") != ports.NodeAlive {
		t.Fatal("un nodo esperado tiene período de gracia")
	}
	_ = c.Emit(context.Background(), events.Heartbeat{NodoID: "n", TipoNodo: "BACKEND", Rol: "N/A", Secuencia: 1})
	now = now.Add(30 * time.Second)
	if c.Observe("n") != ports.NodeAlive {
		t.Fatal("a los 30 s aún no pasaron 3 pulsos completos")
	}
	now = now.Add(time.Second)
	if c.Observe("n") != ports.NodeDead {
		t.Fatal("pasados 3 pulsos debía estar caído")
	}
	_ = c.Emit(context.Background(), events.Heartbeat{NodoID: "n", Secuencia: 2})
	if c.Observe("n") != ports.NodeAlive {
		t.Fatal("un nuevo pulso lo reincorpora")
	}
}
