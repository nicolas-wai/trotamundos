package ports

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
)

// NodeHealth es el estado observado de un nodo.
type NodeHealth string

// Estados de salud (AD §6.4).
const (
	NodeAlive NodeHealth = "VIVO"
	NodeDead  NodeHealth = "CAIDO"
)

// HeartbeatPort [ADR-22] Salud por heartbeat. Un nodo está CAIDO tras 3 pulsos perdidos (pulso de 10 s).
type HeartbeatPort interface {
	Emit(ctx context.Context, heartbeat events.Heartbeat) error
	Observe(nodeID string) NodeHealth
}
