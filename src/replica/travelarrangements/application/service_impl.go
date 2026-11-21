package application

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

const maxTitleLength = 200

// Service implementa el caso de uso de Arreglos de viaje de la Fase 3 (dashboard y CRUD manual)
// sobre puertos propios. [ADR-02] Las operaciones de datos salen por los repositorios, que el adapter
// implementa sobre PriorityQueuePort [ADR-09]. La ingesta de proveedores (E1/E2) llega en la Fase 4.
type Service struct {
	trips        TripRepository
	arrangements ArrangementRepository
	registry     *domain.ArrangementTypeRegistry // [QA-10]
	now          func() time.Time
}

var _ TravelArrangementsService = (*Service)(nil)

// NewService crea el servicio. registry nil = tipos base; now nil = time.Now.
func NewService(trips TripRepository, arrangements ArrangementRepository, registry *domain.ArrangementTypeRegistry, now func() time.Time) *Service {
	if registry == nil {
		registry = domain.DefaultArrangementTypeRegistry()
	}
	if now == nil {
		now = time.Now
	}
	return &Service{trips: trips, arrangements: arrangements, registry: registry, now: now}
}

// GetDashboard arma el dashboard con solo datos críticos [ADR-17]: viajes vigentes con sus arreglos sin detalle.
// Los viajes completados se retiran de la vista (US03). Los avisos de proveedores llegan en la Fase 4.
func (s *Service) GetDashboard(ctx context.Context, travelerID string) (Dashboard, error) {
	trips, err := s.trips.ListByTraveler(ctx, travelerID)
	if err != nil {
		return Dashboard{}, err
	}
	arrangements, err := s.arrangements.ListSummariesByTraveler(ctx, travelerID)
	if err != nil {
		return Dashboard{}, err
	}
	today := startOfDay(s.now())
	byTrip := map[string][]domain.TravelArrangement{}
	for _, a := range arrangements {
		byTrip[a.TripID] = append(byTrip[a.TripID], a)
	}
	visible := make([]domain.Trip, 0, len(trips))
	for _, t := range trips {
		if t.Status == domain.TripCompleted || (!t.EndDate.IsZero() && t.EndDate.Before(today)) {
			continue
		}
		list := byTrip[t.TripID]
		sort.SliceStable(list, func(i, j int) bool { return list[i].StartAt.Before(list[j].StartAt) })
		t.Arrangements = list
		visible = append(visible, t)
	}
	sort.SliceStable(visible, func(i, j int) bool { return visible[i].StartDate.Before(visible[j].StartDate) })
	return Dashboard{Trips: visible, ProviderNotices: []ProviderNotice{}}, nil
}

// GetArrangementDetail devuelve el arreglo completo (con detalle) bajo demanda. [ADR-17]
func (s *Service) GetArrangementDetail(ctx context.Context, travelerID, arrangementID string) (domain.TravelArrangement, error) {
	return s.arrangements.FindByID(ctx, travelerID, arrangementID)
}

// CreateArrangement da de alta un arreglo manual (origen MANUAL). Escritura con prioridad baja. [ADR-09]
func (s *Service) CreateArrangement(ctx context.Context, travelerID string, input ArrangementInput) (string, error) {
	detail, err := s.validate(ctx, travelerID, input)
	if err != nil {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	err = s.arrangements.Save(ctx, domain.TravelArrangement{
		ArrangementID: id, TripID: input.TripID, TravelerID: travelerID, Type: input.Type,
		Origin: domain.OriginManual, Status: domain.StatusConfirmed, Title: strings.TrimSpace(input.Title),
		StartAt: input.StartAt.UTC(), EndAt: input.EndAt.UTC(), Detail: detail, UpdatedAt: s.now().UTC(),
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// UpdateArrangement edita un arreglo propio conservando su origen, remitente, estado y clave de deduplicación.
func (s *Service) UpdateArrangement(ctx context.Context, travelerID, arrangementID string, input ArrangementInput) error {
	current, err := s.arrangements.FindByID(ctx, travelerID, arrangementID)
	if err != nil {
		return err
	}
	detail, err := s.validate(ctx, travelerID, input)
	if err != nil {
		return err
	}
	current.TripID = input.TripID
	current.Type = input.Type
	current.Title = strings.TrimSpace(input.Title)
	current.StartAt = input.StartAt.UTC()
	current.EndAt = input.EndAt.UTC()
	current.Detail = detail
	current.UpdatedAt = s.now().UTC()
	return s.arrangements.Save(ctx, current)
}

// DeleteArrangement da de baja un arreglo propio.
func (s *Service) DeleteArrangement(ctx context.Context, travelerID, arrangementID string) error {
	return s.arrangements.Delete(ctx, travelerID, arrangementID)
}

// ProvisionDemoTrip crea el viaje demo determinístico del viajero recién registrado ◇ (idempotente).
// Es la gestión de viajes mínima de la Fase 3: sin ella POST /arreglos no tendría un idViaje válido.
func (s *Service) ProvisionDemoTrip(ctx context.Context, travelerID string) error {
	today := startOfDay(s.now())
	return s.trips.Save(ctx, domain.Trip{
		TripID: DemoTripID(travelerID), TravelerID: travelerID, Name: "Mi primer viaje", Destination: "Por definir",
		StartDate: today, EndDate: today.AddDate(0, 0, 30), Status: domain.TripActive,
	})
}

// DemoTripID es el id determinístico del viaje demo de un viajero.
func DemoTripID(travelerID string) string { return "demo-" + travelerID }

// ApplyProviderUpdate llega en la Fase 4.
func (s *Service) ApplyProviderUpdate(context.Context, events.ProviderUpdateNormalized) error {
	return apierror.ErrNotImplemented
}

// ReportProviderDegraded llega en la Fase 4.
func (s *Service) ReportProviderDegraded(context.Context, events.ProviderDegraded) error {
	return apierror.ErrNotImplemented
}

// validate comprueba el alta/edición manual (→ ErrValidation, 422) y devuelve el detalle tipado.
func (s *Service) validate(ctx context.Context, travelerID string, in ArrangementInput) (domain.ArrangementDetail, error) {
	title := strings.TrimSpace(in.Title)
	switch {
	case strings.TrimSpace(in.TripID) == "":
		return nil, fmt.Errorf("%w: falta idViaje", ErrValidation)
	case !s.registry.Supports(in.Type):
		return nil, fmt.Errorf("%w: tipo de arreglo no soportado: %q", ErrValidation, in.Type)
	case title == "":
		return nil, fmt.Errorf("%w: falta el título", ErrValidation)
	case len([]rune(title)) > maxTitleLength:
		return nil, fmt.Errorf("%w: el título supera los %d caracteres", ErrValidation, maxTitleLength)
	case in.StartAt.IsZero():
		return nil, fmt.Errorf("%w: falta fechaInicio", ErrValidation)
	case !in.EndAt.IsZero() && in.EndAt.Before(in.StartAt):
		return nil, fmt.Errorf("%w: fechaFin es anterior a fechaInicio", ErrValidation)
	}
	raw, err := json.Marshal(in.Detail)
	if err != nil {
		return nil, fmt.Errorf("%w: detalle inválido", ErrValidation)
	}
	detail, err := s.registry.DecodeDetail(in.Type, raw, true)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if _, err := s.trips.FindByID(ctx, travelerID, in.TripID); err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: el viaje indicado no existe", ErrValidation)
		}
		return nil, err
	}
	return detail, nil
}

func isNotFound(err error) bool { return err != nil && errors.Is(err, ErrNotFound) }

func startOfDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
