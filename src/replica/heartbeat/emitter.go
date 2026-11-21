// Package heartbeat es el componente "Emisor de heartbeat" de la réplica (AD §4.4).
package heartbeat

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// ReplicaHeartbeatEmitter [ADR-22] Pulso periódico (10 s) de la réplica hacia el balanceador (E3).
type ReplicaHeartbeatEmitter struct {
	nodeID   string
	port     ports.HeartbeatPort
	interval time.Duration
	seq      int64
}

// NewReplicaHeartbeatEmitter crea el emisor.
func NewReplicaHeartbeatEmitter(nodeID string, port ports.HeartbeatPort, interval time.Duration) *ReplicaHeartbeatEmitter {
	return &ReplicaHeartbeatEmitter{nodeID: nodeID, port: port, interval: interval}
}

// Run emite un pulso inmediato y luego uno por intervalo hasta que el contexto termina.
func (e *ReplicaHeartbeatEmitter) Run(ctx context.Context) error {
	if e.port == nil || e.interval <= 0 {
		return nil
	}
	e.beat(ctx)
	t := time.NewTicker(e.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			e.beat(ctx)
		}
	}
}

func (e *ReplicaHeartbeatEmitter) beat(ctx context.Context) {
	e.seq++
	_ = e.port.Emit(ctx, events.Heartbeat{
		NodoID: e.nodeID, TipoNodo: "BACKEND", Rol: "N/A", Secuencia: e.seq, EmitidoEn: time.Now().UTC(),
	})
}
