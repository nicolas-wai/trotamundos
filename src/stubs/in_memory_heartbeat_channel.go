package stubs

import (
	"context"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// InMemoryHeartbeatChannel [ADR-22] Canal de heartbeats en memoria detrás de HeartbeatPort.
// Un nodo está CAIDO cuando pasaron más de interval*missThreshold sin pulso (3 pulsos de 10 s ⇒ ~30 s).
// [Prototipo] Reemplaza al transporte de red de producción; intervalo y umbral configurables en tests.
type InMemoryHeartbeatChannel struct {
	interval      time.Duration
	missThreshold int
	now           func() time.Time

	mu       sync.RWMutex
	lastSeen map[string]time.Time
	last     map[string]events.Heartbeat
}

var _ ports.HeartbeatPort = (*InMemoryHeartbeatChannel)(nil)

// NewInMemoryHeartbeatChannel crea el canal.
func NewInMemoryHeartbeatChannel(interval time.Duration, missThreshold int) *InMemoryHeartbeatChannel {
	return &InMemoryHeartbeatChannel{
		interval: interval, missThreshold: missThreshold, now: time.Now,
		lastSeen: map[string]time.Time{}, last: map[string]events.Heartbeat{},
	}
}

// SetClock inyecta el reloj (pruebas).
func (c *InMemoryHeartbeatChannel) SetClock(now func() time.Time) { c.now = now }

// Track da de alta nodos esperados con un período de gracia desde ahora, para que un nodo que
// todavía no emitió su primer pulso no se considere caído al arrancar.
func (c *InMemoryHeartbeatChannel) Track(nodeIDs ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range nodeIDs {
		if _, ok := c.lastSeen[id]; !ok {
			c.lastSeen[id] = c.now()
		}
	}
}

// Emit registra un pulso (E3).
func (c *InMemoryHeartbeatChannel) Emit(_ context.Context, hb events.Heartbeat) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSeen[hb.NodoID] = c.now()
	c.last[hb.NodoID] = hb
	return nil
}

// Observe devuelve VIVO o CAIDO según los pulsos recibidos.
func (c *InMemoryHeartbeatChannel) Observe(nodeID string) ports.NodeHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	seen, ok := c.lastSeen[nodeID]
	if !ok {
		return ports.NodeDead
	}
	if c.now().Sub(seen) > c.interval*time.Duration(c.missThreshold) {
		return ports.NodeDead
	}
	return ports.NodeAlive
}

// Last devuelve el último pulso de un nodo (para pruebas: rol, secuencia).
func (c *InMemoryHeartbeatChannel) Last(nodeID string) (events.Heartbeat, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	hb, ok := c.last[nodeID]
	return hb, ok
}
