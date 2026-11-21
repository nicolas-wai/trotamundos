package datanode

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
)

// HeartbeatEmitter [ADR-22] Emisor de heartbeat del nodo de datos hacia el monitor de BD (E3).
type HeartbeatEmitter struct {
	nodeID string
}

// NewHeartbeatEmitter crea el emisor para un nodo.
func NewHeartbeatEmitter(nodeID string) *HeartbeatEmitter { return &HeartbeatEmitter{nodeID: nodeID} }

// Run emite pulsos periódicos hasta que el contexto termina.
func (e *HeartbeatEmitter) Run(ctx context.Context) error { return apierror.ErrNotImplemented }
