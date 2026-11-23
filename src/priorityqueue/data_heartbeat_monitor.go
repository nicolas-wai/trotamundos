package priorityqueue

import (
	"context"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

// DataHeartbeatMonitor [ADR-22] Monitor de heartbeats de los nodos de datos: cuando el activo se declara
// CAIDO (3 pulsos perdidos de 10 s, umbral que resuelve el HeartbeatPort) ordena al DataNodeSwitch el
// intercambio de responsabilidades, siempre que el pasivo esté vivo y sincronizado. Cuando el nodo caído vuelve
// a latir lo reincorpora como pasivo (imagen completa primero) [ADR-21]. Si caen ambos deja un aviso.
type DataHeartbeatMonitor struct {
	heartbeats ports.HeartbeatPort
	dataSwitch *DataNodeSwitch
	interval   time.Duration

	mu        sync.Mutex
	onAlert   func(string)
	lastAlert string
}

// NewDataHeartbeatMonitor crea el monitor.
func NewDataHeartbeatMonitor(hb ports.HeartbeatPort, sw *DataNodeSwitch, interval time.Duration) *DataHeartbeatMonitor {
	return &DataHeartbeatMonitor{heartbeats: hb, dataSwitch: sw, interval: interval}
}

// OnAlert registra un observador de avisos (ambos nodos caídos, pasivo sin sincronizar). Cada aviso se
// emite una vez por episodio.
func (m *DataHeartbeatMonitor) OnAlert(fn func(string)) {
	m.mu.Lock()
	m.onAlert = fn
	m.mu.Unlock()
}

func (m *DataHeartbeatMonitor) alert(msg string) {
	m.mu.Lock()
	notify := m.onAlert
	repeated := m.lastAlert == msg
	m.lastAlert = msg
	m.mu.Unlock()
	if notify != nil && !repeated && msg != "" {
		notify(msg)
	}
}

// Check evalúa una vez el estado de los nodos: conmuta o reincorpora según corresponda.
func (m *DataHeartbeatMonitor) Check(ctx context.Context) {
	activeHealth := m.heartbeats.Observe(m.dataSwitch.ActiveID())
	passiveHealth := m.heartbeats.Observe(m.dataSwitch.PassiveID())
	switch {
	case activeHealth == ports.NodeDead && passiveHealth == ports.NodeDead:
		m.alert("ambos nodos de datos están caídos: la región no puede atender datos")
	case activeHealth == ports.NodeDead && !m.dataSwitch.PassiveReady():
		m.alert("el nodo de datos activo cayó pero el pasivo aún no tiene la imagen completa: no se conmuta")
	case activeHealth == ports.NodeDead:
		m.alert("")
		_, _ = m.dataSwitch.SwapResponsibilities(ctx)
	case passiveHealth == ports.NodeDead:
		m.dataSwitch.MarkPassiveStale()
		m.alert("el nodo de datos pasivo está caído: la región funciona sin respaldo")
	case !m.dataSwitch.PassiveReady():
		// El nodo caído volvió a latir: primero imagen completa, recién entonces cuenta como pasivo.
		if _, err := m.dataSwitch.Reincorporate(ctx); err != nil {
			m.alert("no se pudo reincorporar el nodo de datos recuperado: " + err.Error())
		} else {
			m.alert("")
		}
	default:
		m.alert("")
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
