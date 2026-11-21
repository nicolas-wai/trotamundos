package priorityqueue

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

// DataHeartbeatMonitor [ADR-22] Monitor de heartbeats de los nodos de datos: cuando el activo se declara
// CAIDO (3 pulsos perdidos de 10 s, umbral que resuelve el HeartbeatPort) ordena al DataNodeSwitch el
// intercambio de responsabilidades, siempre que el pasivo esté vivo.
type DataHeartbeatMonitor struct {
	heartbeats ports.HeartbeatPort
	dataSwitch *DataNodeSwitch
	interval   time.Duration
}

// NewDataHeartbeatMonitor crea el monitor.
func NewDataHeartbeatMonitor(hb ports.HeartbeatPort, sw *DataNodeSwitch, interval time.Duration) *DataHeartbeatMonitor {
	return &DataHeartbeatMonitor{heartbeats: hb, dataSwitch: sw, interval: interval}
}

// Check evalúa una vez el estado del nodo activo y conmuta si corresponde.
func (m *DataHeartbeatMonitor) Check(ctx context.Context) {
	if m.heartbeats.Observe(m.dataSwitch.ActiveID()) == ports.NodeDead &&
		m.heartbeats.Observe(m.dataSwitch.PassiveID()) == ports.NodeAlive {
		_, _ = m.dataSwitch.SwapResponsibilities(ctx)
	}
}

// Run vigila los nodos de datos hasta que el contexto termina.
func (m *DataHeartbeatMonitor) Run(ctx context.Context) error {
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			m.Check(ctx)
		}
	}
}
