// Package application (travelarrangements) contiene los casos de uso de Arreglos de viaje y sus puertos propios.
package application

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// TripRepository es el puerto de persistencia del agregado Viaje.
// El adapter lo implementa sobre PriorityQueuePort: lecturas con prioridad alta, escrituras baja [ADR-09].
type TripRepository interface {
	ListByTraveler(ctx context.Context, travelerID string) ([]domain.Trip, error)
	// FindByID devuelve ErrNotFound si el viaje no existe o no es del viajero.
	FindByID(ctx context.Context, travelerID, tripID string) (domain.Trip, error)
	// Save crea el viaje si no existe (idempotente por idViaje).
	Save(ctx context.Context, trip domain.Trip) error
}

// ArrangementRepository persiste los arreglos de un viaje.
type ArrangementRepository interface {
	// ListSummariesByTraveler devuelve solo datos críticos (sin detalle ni remitente) para el dashboard. [ADR-17]
	ListSummariesByTraveler(ctx context.Context, travelerID string) ([]domain.TravelArrangement, error)
	// FindByID devuelve ErrNotFound si el arreglo no existe o es de otro viajero.
	FindByID(ctx context.Context, travelerID, arrangementID string) (domain.TravelArrangement, error)
	FindByDeduplicationKey(ctx context.Context, travelerID string, key domain.DeduplicationKey) (domain.TravelArrangement, error)
	// Save inserta o actualiza (upsert por idArreglo).
	Save(ctx context.Context, arrangement domain.TravelArrangement) error
	// Delete devuelve ErrNotFound si no hay nada que borrar.
	Delete(ctx context.Context, travelerID, arrangementID string) error
}

// ProviderNoticeRecord es un aviso persistido de proveedor degradado (E2). TravelerID vacío = afecta a todos. [ADR-07]
type ProviderNoticeRecord struct {
	ProviderID string
	TravelerID string
	Message    string
	RetryAt    time.Time
}

// ProviderNoticeRepository persiste los avisos E2 para que cualquier réplica stateless los muestre. [ADR-07]
type ProviderNoticeRepository interface {
	Save(ctx context.Context, notice ProviderNoticeRecord) error
	Clear(ctx context.Context, providerID string) error
	List(ctx context.Context) ([]ProviderNoticeRecord, error)
}
