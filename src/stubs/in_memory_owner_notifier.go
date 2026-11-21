package stubs

import (
	"context"
	"sync"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// InMemoryOwnerNotifier [ADR-13] Registra en memoria las notificaciones al dueño (E7).
// El canal real de entrega (correo, push…) no está definido en el AD. [Prototipo]
type InMemoryOwnerNotifier struct {
	mu            sync.Mutex
	notifications []events.AccessRequestCreated
}

var _ ports.OwnerNotifierPort = (*InMemoryOwnerNotifier)(nil)

// NewInMemoryOwnerNotifier crea el notificador.
func NewInMemoryOwnerNotifier() *InMemoryOwnerNotifier { return &InMemoryOwnerNotifier{} }

// NotifyAccessRequest registra la notificación.
func (n *InMemoryOwnerNotifier) NotifyAccessRequest(_ context.Context, event events.AccessRequestCreated) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notifications = append(n.notifications, event)
	return nil
}

// Notifications devuelve todas las notificaciones registradas, en orden de llegada.
func (n *InMemoryOwnerNotifier) Notifications() []events.AccessRequestCreated {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]events.AccessRequestCreated(nil), n.notifications...)
}

// NotificationsFor devuelve las notificaciones dirigidas a un dueño.
func (n *InMemoryOwnerNotifier) NotificationsFor(ownerID string) []events.AccessRequestCreated {
	var out []events.AccessRequestCreated
	for _, e := range n.Notifications() {
		if e.IDDueno == ownerID {
			out = append(out, e)
		}
	}
	return out
}
