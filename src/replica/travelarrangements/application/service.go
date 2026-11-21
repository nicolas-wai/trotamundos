package application

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// ProviderNotice es un aviso de proveedor degradado (E2) expuesto en el dashboard. [ADR-07]
type ProviderNotice struct {
	Provider string
	Message  string
}

// Dashboard contiene solo datos críticos [ADR-17] y los avisos de proveedores.
type Dashboard struct {
	Trips           []domain.Trip
	ProviderNotices []ProviderNotice
}

// ArrangementInput es el dato de alta/edición manual de un arreglo.
type ArrangementInput struct {
	TripID  string
	Type    domain.ArrangementType
	Title   string
	StartAt time.Time
	EndAt   time.Time
	Detail  map[string]any
}

// TravelArrangementsService [ADR-02][ADR-04] Lógica central: dashboard, altas/bajas/cambios manuales,
// deduplicación y consumo de actualizaciones de proveedores.
type TravelArrangementsService interface {
	GetDashboard(ctx context.Context, travelerID string) (Dashboard, error)
	GetArrangementDetail(ctx context.Context, travelerID, arrangementID string) (domain.TravelArrangement, error)
	CreateArrangement(ctx context.Context, travelerID string, input ArrangementInput) (arrangementID string, err error)
	UpdateArrangement(ctx context.Context, travelerID, arrangementID string, input ArrangementInput) error
	DeleteArrangement(ctx context.Context, travelerID, arrangementID string) error
	// ApplyProviderUpdate aplica E1 deduplicando por claveDeduplicacion (lo invoca UpdateConsumer). [ADR-04]
	ApplyProviderUpdate(ctx context.Context, update events.ProviderUpdateNormalized) error
	// ReportProviderDegraded registra E2 para exponerlo como avisosProveedores. [ADR-07]
	ReportProviderDegraded(ctx context.Context, degraded events.ProviderDegraded) error
}
