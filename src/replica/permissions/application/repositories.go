// Package application (permissions) contiene los casos de uso de Control de permisos y sus puertos propios.
package application

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/replica/permissions/domain"
)

// SharedItineraryRepository es el puerto de persistencia del agregado Itinerario compartido.
type SharedItineraryRepository interface {
	FindByID(ctx context.Context, itineraryID string) (domain.SharedItinerary, error)
	Save(ctx context.Context, itinerary domain.SharedItinerary) error
	Delete(ctx context.Context, ownerID, itineraryID string) error
}

// ItineraryDirectory resuelve el dueño (y por tanto el shard) a partir de idItinerario. ◇ [ADR-11]
type ItineraryDirectory interface {
	Register(ctx context.Context, itineraryID, ownerID string) error
	ResolveOwner(ctx context.Context, itineraryID string) (ownerID string, err error)
}
