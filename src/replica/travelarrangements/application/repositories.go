// Package application (travelarrangements) contiene los casos de uso de Arreglos de viaje y sus puertos propios.
package application

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// TripRepository es el puerto de persistencia del agregado Viaje.
// El adapter lo implementa sobre PriorityQueuePort: lecturas con prioridad alta, escrituras baja [ADR-09].
type TripRepository interface {
	ListByTraveler(ctx context.Context, travelerID string) ([]domain.Trip, error)
	FindByID(ctx context.Context, travelerID, tripID string) (domain.Trip, error)
	Save(ctx context.Context, trip domain.Trip) error
}

// ArrangementRepository persiste los arreglos de un viaje.
type ArrangementRepository interface {
	FindByID(ctx context.Context, travelerID, arrangementID string) (domain.TravelArrangement, error)
	FindByDeduplicationKey(ctx context.Context, travelerID string, key domain.DeduplicationKey) (domain.TravelArrangement, error)
	Save(ctx context.Context, arrangement domain.TravelArrangement) error
	Delete(ctx context.Context, travelerID, arrangementID string) error
}
