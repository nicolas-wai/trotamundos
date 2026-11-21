// Package heartbeat es el componente "Emisor de heartbeat" de la réplica (AD §4.4).
package heartbeat

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Emitter [ADR-22] Pulso periódico (10 s) de la réplica hacia el balanceador (E3). Lógica en la Fase 2.
type Emitter struct {
	nodeID   string
	port     ports.HeartbeatPort
	interval time.Duration
}

// NewEmitter crea el emisor.
func NewEmitter(nodeID string, port ports.HeartbeatPort, interval time.Duration) *Emitter {
	return &Emitter{nodeID: nodeID, port: port, interval: interval}
}

// Run emite pulsos hasta que el contexto termina.
func (e *Emitter) Run(ctx context.Context) error { return apierror.ErrNotImplemented }
