package ports

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
)

// OwnerNotifierPort [ADR-13] Notificación al dueño de un itinerario (E7).
// El canal real de entrega no está definido en el AD; el prototipo usa InMemoryOwnerNotifier.
type OwnerNotifierPort interface {
	NotifyAccessRequest(ctx context.Context, event events.AccessRequestCreated) error
}
