package application

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/permissions/domain"
)

// PermissionsService [ADR-13] Itinerarios compartidos con aprobación explícita del dueño (QA09).
// Incluye ports.ItineraryPermissionsPort (CanView, RequestAccess, Decide, Revoke).
type PermissionsService interface {
	ports.ItineraryPermissionsPort

	CreateItinerary(ctx context.Context, ownerID, name string, arrangementIDs []string) (domain.SharedItinerary, error)
	DeleteItinerary(ctx context.Context, ownerID, itineraryID string) error
	GetItinerary(ctx context.Context, viewerID, itineraryID string) (domain.SharedItinerary, error)
	AddArrangement(ctx context.Context, ownerID, itineraryID, arrangementID string) error
	RemoveArrangement(ctx context.Context, ownerID, itineraryID, arrangementID string) error
	ListRequests(ctx context.Context, ownerID, itineraryID string) ([]domain.AccessRequest, error)
	// DecideRequest decide una solicitud verificando que el itinerario pertenezca al dueño.
	DecideRequest(ctx context.Context, ownerID, itineraryID, requestID string, decision ports.AccessDecision) (domain.RequestStatus, error)
}
