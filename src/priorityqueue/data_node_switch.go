package priorityqueue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// DataNodeHandle es lo que el conmutador necesita de un nodo de datos (el contenedor datanode lo satisface).
type DataNodeHandle interface {
	ports.DataPort
	ports.SyncTarget
	ports.PartitionResolver
	Promote()
	Demote()
	SetPeer(ports.SyncTarget)
	SyncNow(ctx context.Context) (events.StateSync, error)
}

// DataNodeSwitch [ADR-21][ADR-22] Conmutador de nodo de datos: dirige el tráfico al nodo activo y,
// ante fallo detectado por DataHeartbeatMonitor, redirige al pasivo y emite E4.
type DataNodeSwitch struct {
	regionID string

	mu      sync.RWMutex
	active  DataNodeHandle
	passive DataNodeHandle
	// passiveReady: el pasivo tiene una imagen completa y vigente del activo. Si es false no se usa como
	// destino de conmutación. [ADR-21]
	passiveReady bool
	onSwap       func(events.ResponsibilitySwap)
	onRejoin     func(events.PassiveRejoined)
}

// NewDataNodeSwitch crea el conmutador con el nodo activo y el pasivo.
func NewDataNodeSwitch(regionID string, active, passive DataNodeHandle) *DataNodeSwitch {
	active.SetPeer(passive)
	return &DataNodeSwitch{regionID: regionID, active: active, passive: passive, passiveReady: true}
}

// OnSwap registra un observador de E4 IntercambioResponsabilidades.
func (s *DataNodeSwitch) OnSwap(fn func(events.ResponsibilitySwap)) {
	s.mu.Lock()
	s.onSwap = fn
	s.mu.Unlock()
}

// OnRejoin registra un observador de E4b PasivoReincorporado.
func (s *DataNodeSwitch) OnRejoin(fn func(events.PassiveRejoined)) {
	s.mu.Lock()
	s.onRejoin = fn
	s.mu.Unlock()
}

// PassiveReady indica si el pasivo cuenta como respaldo válido (ya recibió la imagen del activo).
func (s *DataNodeSwitch) PassiveReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.passiveReady
}

// MarkPassiveStale quita al pasivo de la lista de respaldos válidos (cayó y se perdió escrituras). [ADR-21]
func (s *DataNodeSwitch) MarkPassiveStale() {
	s.mu.Lock()
	s.passiveReady = false
	s.mu.Unlock()
}

// Reincorporate instala en el pasivo una imagen completa del activo (VACUUM INTO, igual que SyncEmitter) y solo
// entonces lo declara respaldo válido; emite E4b. Si la instalación falla, el pasivo sigue sin contar. [ADR-21]
func (s *DataNodeSwitch) Reincorporate(ctx context.Context) (events.PassiveRejoined, error) {
	s.mu.RLock()
	active, passive := s.active, s.passive
	s.mu.RUnlock()
	active.SetPeer(passive)          // el activo apunta al nodo recuperado y retoma la sincronización periódica
	sync, err := active.SyncNow(ctx) // fuera del candado: el tráfico sigue fluyendo durante la copia
	if err != nil {
		return events.PassiveRejoined{}, fmt.Errorf("sincronizando imagen completa en %s: %w", passive.ID(), err)
	}
	s.mu.Lock()
	if s.active != active || s.passive != passive { // hubo una conmutación mientras copiábamos
		s.mu.Unlock()
		return events.PassiveRejoined{}, errors.New("los roles cambiaron durante la reincorporación")
	}
	s.passiveReady = true
	e := events.PassiveRejoined{RegionID: s.regionID, Nodo: passive.ID(), VersionEstado: sync.VersionEstado, OcurridoEn: time.Now().UTC()}
	notify := s.onRejoin
	s.mu.Unlock()
	if notify != nil {
		notify(e)
	}
	return e, nil
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

// PartitionOf delega en el nodo activo (ambos nodos tienen el mismo particionado). [ADR-11]
func (s *DataNodeSwitch) PartitionOf(op events.QueuedOperation) (string, error) {
	s.mu.RLock()
	node := s.active
	s.mu.RUnlock()
	return node.PartitionOf(op)
}

// Partitions lista las particiones del nodo activo.
func (s *DataNodeSwitch) Partitions() []string {
	s.mu.RLock()
	node := s.active
	s.mu.RUnlock()
	return node.Partitions()
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
	s.active.SetPeer(s.passive) // retoma la sincronización periódica hacia el nodo caído cuando vuelva
	s.passiveReady = false      // el anterior activo no tiene imagen vigente hasta reincorporarse
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
