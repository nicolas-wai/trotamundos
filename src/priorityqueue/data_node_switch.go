package priorityqueue

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// DataNodeSwitch [ADR-21][ADR-22] Conmutador de nodo de datos: dirige el tráfico al nodo activo y,
// ante fallo detectado por DataHeartbeatMonitor, redirige al pasivo y emite E4.
type DataNodeSwitch struct {
	active  ports.DataPort
	passive ports.DataPort
}

// NewDataNodeSwitch crea el conmutador con el nodo activo y el pasivo.
func NewDataNodeSwitch(active, passive ports.DataPort) *DataNodeSwitch {
	return &DataNodeSwitch{active: active, passive: passive}
}

// Dispatch envía la operación al nodo de datos activo.
func (s *DataNodeSwitch) Dispatch(context.Context, events.QueuedOperation) (ports.OperationResult, error) {
	return ports.OperationResult{}, apierror.ErrNotImplemented
}

// SwapResponsibilities intercambia primario y pasivo y emite E4.
func (s *DataNodeSwitch) SwapResponsibilities(context.Context) (events.ResponsibilitySwap, error) {
	return events.ResponsibilitySwap{}, apierror.ErrNotImplemented
}
