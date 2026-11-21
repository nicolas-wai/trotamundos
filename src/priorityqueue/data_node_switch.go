package priorityqueue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// DataNodeHandle es lo que el conmutador necesita de un nodo de datos (el contenedor datanode lo satisface).
type DataNodeHandle interface {
	ports.DataPort
	ID() string
	Promote()
	Demote()
}

// DataNodeSwitch [ADR-21][ADR-22] Conmutador de nodo de datos: dirige el tráfico al nodo activo y,
// ante fallo detectado por DataHeartbeatMonitor, redirige al pasivo y emite E4.
type DataNodeSwitch struct {
	regionID string

	mu      sync.RWMutex
	active  DataNodeHandle
	passive DataNodeHandle
	onSwap  func(events.ResponsibilitySwap)
}

// NewDataNodeSwitch crea el conmutador con el nodo activo y el pasivo.
func NewDataNodeSwitch(regionID string, active, passive DataNodeHandle) *DataNodeSwitch {
	return &DataNodeSwitch{regionID: regionID, active: active, passive: passive}
}

// OnSwap registra un observador de E4 IntercambioResponsabilidades.
func (s *DataNodeSwitch) OnSwap(fn func(events.ResponsibilitySwap)) {
	s.mu.Lock()
	s.onSwap = fn
	s.mu.Unlock()
}

// ActiveID devuelve el id del nodo activo.
func (s *DataNodeSwitch) ActiveID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active.ID()
}

// PassiveID devuelve el id del nodo pasivo.
func (s *DataNodeSwitch) PassiveID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.passive.ID()
}

// Dispatch envía la operación al nodo de datos activo.
func (s *DataNodeSwitch) Dispatch(ctx context.Context, op events.QueuedOperation) (ports.OperationResult, error) {
	s.mu.RLock()
	node := s.active
	s.mu.RUnlock()
	switch op.Tipo {
	case events.OperationRead:
		return node.Read(ctx, op.IDViajero, ports.DataQuery{Resource: op.Recurso, Params: op.Payload})
	case events.OperationWrite:
		return node.Write(ctx, op.IDViajero, ports.DataCommand{Resource: op.Recurso, Params: op.Payload})
	}
	return ports.OperationResult{}, fmt.Errorf("tipo de operación inválido: %q", op.Tipo)
}

// SwapResponsibilities intercambia primario y pasivo: el pasivo se promueve (activa su heartbeat y
// sus sincronizaciones), el anterior pasa a pasivo, y se emite E4. [ADR-22]
func (s *DataNodeSwitch) SwapResponsibilities(context.Context) (events.ResponsibilitySwap, error) {
	s.mu.Lock()
	s.active, s.passive = s.passive, s.active
	s.active.Promote()
	s.passive.Demote()
	e4 := events.ResponsibilitySwap{
		RegionID: s.regionID, NuevoPrimario: s.active.ID(), NuevoPasivo: s.passive.ID(),
		Motivo: "HEARTBEAT_PERDIDO", OcurridoEn: time.Now().UTC(),
	}
	notify := s.onSwap
	s.mu.Unlock()
	if notify != nil {
		notify(e4)
	}
	return e4, nil
}
