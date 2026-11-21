package application

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// UnimplementedTravelArrangementsService es el stub de la Fase 1 (lo usan las pruebas del router).
type UnimplementedTravelArrangementsService struct{}

var _ TravelArrangementsService = UnimplementedTravelArrangementsService{}

// GetDashboard no implementado.
func (UnimplementedTravelArrangementsService) GetDashboard(context.Context, string) (Dashboard, error) {
	return Dashboard{}, apierror.ErrNotImplemented
}

// GetArrangementDetail no implementado.
func (UnimplementedTravelArrangementsService) GetArrangementDetail(context.Context, string, string) (domain.TravelArrangement, error) {
	return domain.TravelArrangement{}, apierror.ErrNotImplemented
}

// CreateArrangement no implementado.
func (UnimplementedTravelArrangementsService) CreateArrangement(context.Context, string, ArrangementInput) (string, error) {
	return "", apierror.ErrNotImplemented
}

// UpdateArrangement no implementado.
func (UnimplementedTravelArrangementsService) UpdateArrangement(context.Context, string, string, ArrangementInput) error {
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
