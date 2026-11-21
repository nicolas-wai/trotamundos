package application

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
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

// UnimplementedPermissionsService es el stub de la Fase 1.
type UnimplementedPermissionsService struct{}

var _ PermissionsService = UnimplementedPermissionsService{}

// CanView no implementado.
func (UnimplementedPermissionsService) CanView(context.Context, string, string) (bool, error) {
	return false, apierror.ErrNotImplemented
}

// RequestAccess no implementado.
func (UnimplementedPermissionsService) RequestAccess(context.Context, string, string) (domain.AccessRequest, error) {
	return domain.AccessRequest{}, apierror.ErrNotImplemented
}

// Decide no implementado.
func (UnimplementedPermissionsService) Decide(context.Context, string, string, ports.AccessDecision) error {
	return apierror.ErrNotImplemented
}

// Revoke no implementado.
func (UnimplementedPermissionsService) Revoke(context.Context, string, string, string) error {
	return apierror.ErrNotImplemented
}

// CreateItinerary no implementado.
func (UnimplementedPermissionsService) CreateItinerary(context.Context, string, string, []string) (domain.SharedItinerary, error) {
	return domain.SharedItinerary{}, apierror.ErrNotImplemented
}

// DeleteItinerary no implementado.
func (UnimplementedPermissionsService) DeleteItinerary(context.Context, string, string) error {
	return apierror.ErrNotImplemented
}

// GetItinerary no implementado.
func (UnimplementedPermissionsService) GetItinerary(context.Context, string, string) (domain.SharedItinerary, error) {
	return domain.SharedItinerary{}, apierror.ErrNotImplemented
}

// AddArrangement no implementado.
func (UnimplementedPermissionsService) AddArrangement(context.Context, string, string, string) error {
	return apierror.ErrNotImplemented
}

// RemoveArrangement no implementado.
func (UnimplementedPermissionsService) RemoveArrangement(context.Context, string, string, string) error {
	return apierror.ErrNotImplemented
}

// ListRequests no implementado.
func (UnimplementedPermissionsService) ListRequests(context.Context, string, string) ([]domain.AccessRequest, error) {
	return nil, apierror.ErrNotImplemented
}

// DecideRequest no implementado.
func (UnimplementedPermissionsService) DecideRequest(context.Context, string, string, string, ports.AccessDecision) (domain.RequestStatus, error) {
	return "", apierror.ErrNotImplemented
}
