// Servicios "no implementados" de los módulos del monolito (esqueleto de la Fase 1). [ADR-02] [Prototipo]
// Son dobles de prueba: devuelven apierror.ErrNotImplemented. Viven en stubs, fuera del núcleo
// (domain/application), para que este no dependa de contracts/apierror ni, transitivamente, de net/http.
package stubs

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	authapp "github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	permapp "github.com/nicolas-wai/trotamundos/src/replica/permissions/application"
	permdomain "github.com/nicolas-wai/trotamundos/src/replica/permissions/domain"
	taapp "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
	tadomain "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

var (
	_ authapp.AuthenticationService   = UnimplementedAuthenticationService{}
	_ authapp.TravelerProfileService  = UnimplementedTravelerProfileService{}
	_ taapp.TravelArrangementsService = UnimplementedTravelArrangementsService{}
	_ permapp.PermissionsService      = UnimplementedPermissionsService{}
)

// UnimplementedAuthenticationService es el stub de la Fase 1.
type UnimplementedAuthenticationService struct{}

// Register no implementado.
func (UnimplementedAuthenticationService) Register(context.Context, string, string) (string, error) {
	return "", apierror.ErrNotImplemented
}

// Login no implementado.
func (UnimplementedAuthenticationService) Login(context.Context, string, string) (ports.TokenPair, error) {
	return ports.TokenPair{}, apierror.ErrNotImplemented
}

// Refresh no implementado.
func (UnimplementedAuthenticationService) Refresh(context.Context, string) (ports.AccessToken, error) {
	return ports.AccessToken{}, apierror.ErrNotImplemented
}

// Logout no implementado.
func (UnimplementedAuthenticationService) Logout(context.Context, string) error {
	return apierror.ErrNotImplemented
}

// UnimplementedTravelerProfileService es el stub de la Fase 1.
type UnimplementedTravelerProfileService struct{}

// UpdatePreferences no implementado.
func (UnimplementedTravelerProfileService) UpdatePreferences(context.Context, string, map[string]any) error {
	return apierror.ErrNotImplemented
}

// UnimplementedPermissionsService es el stub de la Fase 1 (lo usan las pruebas del router sin módulo de datos).
type UnimplementedPermissionsService struct{}

// CanView no implementado.
func (UnimplementedPermissionsService) CanView(context.Context, string, string) (bool, error) {
	return false, apierror.ErrNotImplemented
}

// RequestAccess no implementado.
func (UnimplementedPermissionsService) RequestAccess(context.Context, string, string) (permdomain.AccessRequest, error) {
	return permdomain.AccessRequest{}, apierror.ErrNotImplemented
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
func (UnimplementedPermissionsService) CreateItinerary(context.Context, string, string, []string) (permdomain.SharedItinerary, error) {
	return permdomain.SharedItinerary{}, apierror.ErrNotImplemented
}

// DeleteItinerary no implementado.
func (UnimplementedPermissionsService) DeleteItinerary(context.Context, string, string) error {
	return apierror.ErrNotImplemented
}

// GetItinerary no implementado.
func (UnimplementedPermissionsService) GetItinerary(context.Context, string, string) (permdomain.SharedItinerary, error) {
	return permdomain.SharedItinerary{}, apierror.ErrNotImplemented
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
func (UnimplementedPermissionsService) ListRequests(context.Context, string, string) ([]permdomain.AccessRequest, error) {
	return nil, apierror.ErrNotImplemented
}

// DecideRequest no implementado.
func (UnimplementedPermissionsService) DecideRequest(context.Context, string, string, string, ports.AccessDecision) (permdomain.RequestStatus, error) {
	return "", apierror.ErrNotImplemented
}

// UnimplementedTravelArrangementsService es el stub de la Fase 1 (lo usan las pruebas del router).
type UnimplementedTravelArrangementsService struct{}

// GetDashboard no implementado.
func (UnimplementedTravelArrangementsService) GetDashboard(context.Context, string) (taapp.Dashboard, error) {
	return taapp.Dashboard{}, apierror.ErrNotImplemented
}

// GetArrangementDetail no implementado.
func (UnimplementedTravelArrangementsService) GetArrangementDetail(context.Context, string, string) (tadomain.TravelArrangement, error) {
	return tadomain.TravelArrangement{}, apierror.ErrNotImplemented
}

// CreateArrangement no implementado.
func (UnimplementedTravelArrangementsService) CreateArrangement(context.Context, string, taapp.ArrangementInput) (string, error) {
	return "", apierror.ErrNotImplemented
}

// UpdateArrangement no implementado.
func (UnimplementedTravelArrangementsService) UpdateArrangement(context.Context, string, string, taapp.ArrangementInput) error {
	return apierror.ErrNotImplemented
}

// DeleteArrangement no implementado.
func (UnimplementedTravelArrangementsService) DeleteArrangement(context.Context, string, string) error {
	return apierror.ErrNotImplemented
}

// ApplyProviderUpdate no implementado.
func (UnimplementedTravelArrangementsService) ApplyProviderUpdate(context.Context, events.ProviderUpdateNormalized) error {
	return apierror.ErrNotImplemented
}

// ReportProviderDegraded no implementado.
func (UnimplementedTravelArrangementsService) ReportProviderDegraded(context.Context, events.ProviderDegraded) error {
	return apierror.ErrNotImplemented
}
