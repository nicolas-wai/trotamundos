package priorityqueue

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// DataHeartbeatMonitor [ADR-22] Monitor de heartbeats de los nodos de datos: tras 3 pulsos perdidos
// (pulso de 10 s) del activo, ordena al DataNodeSwitch el intercambio de responsabilidades.
type DataHeartbeatMonitor struct {
	heartbeats    ports.HeartbeatPort
	dataSwitch    *DataNodeSwitch
	interval      time.Duration
	missThreshold int
}

// NewDataHeartbeatMonitor crea el monitor.
func NewDataHeartbeatMonitor(hb ports.HeartbeatPort, sw *DataNodeSwitch, interval time.Duration, missThreshold int) *DataHeartbeatMonitor {
	return &DataHeartbeatMonitor{heartbeats: hb, dataSwitch: sw, interval: interval, missThreshold: missThreshold}
}

// Run vigila los nodos de datos hasta que el contexto termina.
func (m *DataHeartbeatMonitor) Run(ctx context.Context) error { return apierror.ErrNotImplemented }
