package messagequeue

import (
	"context"
	"sync"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

// InMemoryEventBus [ADR-07][ADR-13] Bus de eventos interno en memoria para E2 (ProveedorDegradado) y E7.
// Publica de forma síncrona y en orden de suscripción. [Prototipo] equivale al bus interno de una región.
type InMemoryEventBus struct {
	mu       sync.RWMutex
	handlers map[string][]ports.EventHandler
}

var _ ports.EventBusPort = (*InMemoryEventBus)(nil)

// NewInMemoryEventBus crea el bus.
func NewInMemoryEventBus() *InMemoryEventBus {
	return &InMemoryEventBus{handlers: map[string][]ports.EventHandler{}}
}

// Publish entrega el evento a los suscriptores del tópico.
func (b *InMemoryEventBus) Publish(ctx context.Context, topic string, payload any) error {
	b.mu.RLock()
	hs := append([]ports.EventHandler(nil), b.handlers[topic]...)
	b.mu.RUnlock()
	for _, h := range hs {
		h(ctx, payload)
	}
	return nil
}

// Subscribe registra un suscriptor del tópico.
func (b *InMemoryEventBus) Subscribe(topic string, handler ports.EventHandler) {
	b.mu.Lock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	b.mu.Unlock()
}
