// Package application (permissions) contiene los casos de uso de Control de permisos y sus puertos propios.
package application

import (
	"context"
	"errors"

	"github.com/nicolas-wai/trotamundos/src/replica/permissions/domain"
)

// Errores de la capa de aplicación de Permisos.
var (
	// ErrItineraryNotFound: el itinerario no existe (o no es del dueño que lo gestiona; no se revela cuál).
	ErrItineraryNotFound = errors.New("itinerario no encontrado")
	// ErrArrangementNotFound: el arreglo no existe o no es del dueño.
	ErrArrangementNotFound = errors.New("arreglo de viaje no encontrado")
	// ErrForbidden: la operación solo la puede hacer el dueño del itinerario. [ADR-13][QA-09]
	ErrForbidden = errors.New("solo el dueño del itinerario puede realizar esta operación")
	// ErrAccessDenied: el viajero no está autorizado a ver el itinerario. [QA-09]
	ErrAccessDenied = errors.New("acceso no autorizado al itinerario")
	// ErrValidation: datos de entrada inválidos.
	ErrValidation = errors.New("datos inválidos")
)

// SharedItineraryRepository es el puerto de persistencia del agregado Itinerario compartido.
// El adapter lo implementa sobre la cola de prioridades [ADR-09] y resuelve el shard del dueño mediante
// ItineraryDirectory [ADR-11]. Las operaciones de escritura reciben ownerID porque es la clave de partición.
type SharedItineraryRepository interface {
	// FindByID carga el agregado completo (itinerario, arreglos referenciados, permisos y solicitudes).
	// Devuelve ErrItineraryNotFound si no existe.
	FindByID(ctx context.Context, itineraryID string) (domain.SharedItinerary, error)
	// Create registra el itinerario en el directorio y lo guarda con sus arreglos iniciales.
	Create(ctx context.Context, itinerary domain.SharedItinerary) error
	// Delete elimina el itinerario con sus permisos y solicitudes.
	Delete(ctx context.Context, ownerID, itineraryID string) error
	AddArrangement(ctx context.Context, ownerID, itineraryID, arrangementID string) error
	RemoveArrangement(ctx context.Context, ownerID, itineraryID, arrangementID string) error
	SavePermission(ctx context.Context, ownerID, itineraryID string, permission domain.AccessPermission) error
	SaveRequest(ctx context.Context, ownerID, itineraryID string, request domain.AccessRequest) error
	// FindItineraryIDByRequest devuelve el itinerario de una solicitud dentro del shard del dueño
	// (ErrItineraryNotFound si no existe en ese shard).
	FindItineraryIDByRequest(ctx context.Context, ownerID, requestID string) (string, error)
}

// ItineraryDirectory resuelve el dueño (y por tanto el shard) a partir de idItinerario. ◇ [ADR-11]
type ItineraryDirectory interface {
	Register(ctx context.Context, itineraryID, ownerID string) error
	ResolveOwner(ctx context.Context, itineraryID string) (ownerID string, err error)
}

// ArrangementCatalog es el puerto con el que Permisos consulta Arreglos de viaje (AD §4.4: PERM → VIA).
// El adapter lo implementa sobre el repositorio del módulo Arreglos de viaje; Permisos no conoce su dominio.
type ArrangementCatalog interface {
	// Exists indica si el arreglo existe y es del dueño (los de otro viajero cuentan como inexistentes).
	Exists(ctx context.Context, ownerID, arrangementID string) (bool, error)
	// Summaries devuelve los datos críticos de los arreglos del dueño; los que ya no existen se omiten.
	Summaries(ctx context.Context, ownerID string, arrangementIDs []string) ([]domain.ArrangementRef, error)
}
