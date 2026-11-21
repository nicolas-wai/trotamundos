package stubs

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// InMemoryOwnerNotifier [ADR-13] Registra en memoria las notificaciones al dueño (E7).
// El canal real de entrega no está definido en el AD.
type InMemoryOwnerNotifier struct{}

var _ ports.OwnerNotifierPort = (*InMemoryOwnerNotifier)(nil)

// NewInMemoryOwnerNotifier crea el notificador.
func NewInMemoryOwnerNotifier() *InMemoryOwnerNotifier { return &InMemoryOwnerNotifier{} }

// NotifyAccessRequest registra la notificación.
func (*InMemoryOwnerNotifier) NotifyAccessRequest(context.Context, events.AccessRequestCreated) error {
	return apierror.ErrNotImplemented
}
