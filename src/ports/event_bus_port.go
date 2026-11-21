package ports

import "context"

// EventHandler procesa un evento publicado en el bus.
type EventHandler func(ctx context.Context, payload any)

// EventBusPort [ADR-07][ADR-13] Bus de eventos interno en memoria para E2 (ProveedorDegradado)
// y E7 (SolicitudAccesoCreada). Los tópicos están en contracts/events.
type EventBusPort interface {
	Publish(ctx context.Context, topic string, payload any) error
	Subscribe(topic string, handler EventHandler)
}
