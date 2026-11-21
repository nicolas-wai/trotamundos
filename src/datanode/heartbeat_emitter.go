package datanode

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// DataNodeHeartbeatEmitter [ADR-22] Emisor de heartbeat del nodo de datos hacia el monitor de BD (E3).
// El rol se consulta en cada pulso: tras un intercambio de responsabilidades el pulso refleja el nuevo rol.
type DataNodeHeartbeatEmitter struct {
	nodeID   string
	port     ports.HeartbeatPort
	interval time.Duration
	role     func() Role
	seq      int64
}

// NewDataNodeHeartbeatEmitter crea el emisor para un nodo.
func NewDataNodeHeartbeatEmitter(nodeID string, port ports.HeartbeatPort, interval time.Duration, role func() Role) *DataNodeHeartbeatEmitter {
	return &DataNodeHeartbeatEmitter{nodeID: nodeID, port: port, interval: interval, role: role}
}

// Run emite un pulso inmediato y luego uno por intervalo hasta que el contexto termina.
func (e *DataNodeHeartbeatEmitter) Run(ctx context.Context) error {
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

func (e *DataNodeHeartbeatEmitter) beat(ctx context.Context) {
	e.seq++
	_ = e.port.Emit(ctx, events.Heartbeat{
		NodoID: e.nodeID, TipoNodo: "DATOS", Rol: string(e.role()), Secuencia: e.seq, EmitidoEn: time.Now().UTC(),
	})
}
