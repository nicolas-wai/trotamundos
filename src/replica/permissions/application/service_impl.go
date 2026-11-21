package application

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/permissions/domain"
)

// MaxItineraryNameLength es el largo máximo del nombre de un itinerario.
const MaxItineraryNameLength = 200

// AccessDeniedError es el 403 de GET /itinerarios/{id}: informa si al viajero se le ofrece solicitar acceso.
// Es un ErrAccessDenied para errors.Is. [ADR-13][QA-09]
type AccessDeniedError struct{ CanRequest bool }

func (e *AccessDeniedError) Error() string { return ErrAccessDenied.Error() }

// Is permite errors.Is(err, ErrAccessDenied).
func (e *AccessDeniedError) Is(target error) bool { return target == ErrAccessDenied }

// Service [ADR-13] implementa PermissionsService sobre puertos propios (sin librerías concretas).
// Todo acceso nuevo requiere la decisión explícita del dueño (QA09: 0 accesos no autorizados).
type Service struct {
	itineraries  SharedItineraryRepository
	arrangements ArrangementCatalog
	bus          ports.EventBusPort // E7; nil = sin notificación
	now          func() time.Time
}

var _ PermissionsService = (*Service)(nil)

// NewService crea el servicio de permisos.
func NewService(itineraries SharedItineraryRepository, arrangements ArrangementCatalog, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{itineraries: itineraries, arrangements: arrangements, now: now}
}

// WithEventBus fija el bus de eventos por el que se publica E7 `SolicitudAccesoCreada` [ADR-13].
func (s *Service) WithEventBus(bus ports.EventBusPort) *Service {
	s.bus = bus
	return s
}

// CreateItinerary crea un itinerario con link público por defecto y las reservas propias indicadas. [ADR-13]
func (s *Service) CreateItinerary(ctx context.Context, ownerID, name string, arrangementIDs []string) (domain.SharedItinerary, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > MaxItineraryNameLength {
		return domain.SharedItinerary{}, fmt.Errorf("%w: el nombre es obligatorio (máx. %d caracteres)", ErrValidation, MaxItineraryNameLength)
	}
	ids := uniqueIDs(arrangementIDs)
	for _, id := range ids {
		ok, err := s.arrangements.Exists(ctx, ownerID, id)
		if err != nil {
			return domain.SharedItinerary{}, err
		}
		if !ok {
			return domain.SharedItinerary{}, fmt.Errorf("%w: el arreglo %q no existe", ErrValidation, id)
		}
	}
	id, err := newID("itin")
	if err != nil {
		return domain.SharedItinerary{}, err
	}
	it := domain.SharedItinerary{
		ItineraryID: id, OwnerID: ownerID, Name: name, PublicLink: "/itinerarios/" + id, ArrangementIDs: ids,
	}
	if err := s.itineraries.Create(ctx, it); err != nil {
		return domain.SharedItinerary{}, err
	}
	return it, nil
}

// DeleteItinerary elimina el itinerario (solo el dueño).
func (s *Service) DeleteItinerary(ctx context.Context, ownerID, itineraryID string) error {
	if _, err := s.ownedItinerary(ctx, ownerID, itineraryID); err != nil {
		return err
	}
	return s.itineraries.Delete(ctx, ownerID, itineraryID)
}

// GetItinerary devuelve el itinerario con sus reservas si el viajero es el dueño o está autorizado; si no,
// *AccessDeniedError (403 con la opción de solicitar acceso). [QA-09]
func (s *Service) GetItinerary(ctx context.Context, viewerID, itineraryID string) (domain.SharedItinerary, error) {
	it, err := s.itineraries.FindByID(ctx, itineraryID)
	if err != nil {
		return domain.SharedItinerary{}, err
	}
	if !it.CanView(viewerID) {
		return domain.SharedItinerary{}, &AccessDeniedError{CanRequest: it.CanRequestAccess(viewerID)}
	}
	refs, err := s.arrangements.Summaries(ctx, it.OwnerID, it.ArrangementIDs)
	if err != nil {
		return domain.SharedItinerary{}, err
	}
	it.Arrangements = refs
	// El invitado no necesita ver quién más tiene acceso ni las solicitudes: se devuelven solo al dueño.
	if viewerID != it.OwnerID {
		it.Permissions, it.Requests = nil, nil
	}
	return it, nil
}

// CanView indica si el viajero puede ver el itinerario (dueño o autorizado vigente).
func (s *Service) CanView(ctx context.Context, travelerID, itineraryID string) (bool, error) {
	it, err := s.itineraries.FindByID(ctx, itineraryID)
	if err != nil {
		return false, err
	}
	return it.CanView(travelerID), nil
}

// AddArrangement agrega una reserva propia al itinerario (solo el dueño).
func (s *Service) AddArrangement(ctx context.Context, ownerID, itineraryID, arrangementID string) error {
	if _, err := s.ownedItinerary(ctx, ownerID, itineraryID); err != nil {
		return err
	}
	if strings.TrimSpace(arrangementID) == "" {
		return fmt.Errorf("%w: idArreglo es obligatorio", ErrValidation)
	}
	ok, err := s.arrangements.Exists(ctx, ownerID, arrangementID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrArrangementNotFound
	}
	return s.itineraries.AddArrangement(ctx, ownerID, itineraryID, arrangementID)
}

// RemoveArrangement quita una reserva del itinerario (solo el dueño).
func (s *Service) RemoveArrangement(ctx context.Context, ownerID, itineraryID, arrangementID string) error {
	it, err := s.ownedItinerary(ctx, ownerID, itineraryID)
	if err != nil {
		return err
	}
	for _, id := range it.ArrangementIDs {
		if id == arrangementID {
			return s.itineraries.RemoveArrangement(ctx, ownerID, itineraryID, arrangementID)
		}
	}
	return ErrArrangementNotFound
}

// RequestAccess registra una solicitud PENDIENTE y publica E7 para notificar al dueño. [ADR-13]
func (s *Service) RequestAccess(ctx context.Context, travelerID, itineraryID string) (domain.AccessRequest, error) {
	it, err := s.itineraries.FindByID(ctx, itineraryID)
	if err != nil {
		return domain.AccessRequest{}, err
	}
	requestID, err := newID("sol")
	if err != nil {
		return domain.AccessRequest{}, err
	}
	req, created, err := it.NewAccessRequest(travelerID, requestID, s.now())
	if err != nil {
		return domain.AccessRequest{}, err
	}
	if !created {
		return req, nil // ya había una pendiente: no se duplica ni se vuelve a notificar
	}
	if err := s.itineraries.SaveRequest(ctx, it.OwnerID, it.ItineraryID, req); err != nil {
		return domain.AccessRequest{}, err
	}
	if s.bus != nil {
		// La notificación es un efecto secundario: su falla no revierte la solicitud ya registrada.
		_ = s.bus.Publish(ctx, events.TopicAccessRequestCreated, events.AccessRequestCreated{
			IDItinerario: it.ItineraryID, IDDueno: it.OwnerID, IDSolicitante: travelerID,
			IDSolicitud: req.RequestID, CreadaEn: req.RequestedAt,
		})
	}
	return req, nil
}

// ListRequests devuelve las solicitudes pendientes (solo el dueño).
func (s *Service) ListRequests(ctx context.Context, ownerID, itineraryID string) ([]domain.AccessRequest, error) {
	it, err := s.ownedItinerary(ctx, ownerID, itineraryID)
	if err != nil {
		return nil, err
	}
	pending := []domain.AccessRequest{}
	for _, r := range it.Requests {
		if r.Status == domain.RequestPending {
			pending = append(pending, r)
		}
	}
	return pending, nil
}

// Decide decide una solicitud identificada solo por su id: resuelve su itinerario dentro del shard del dueño.
func (s *Service) Decide(ctx context.Context, ownerID, requestID string, decision ports.AccessDecision) error {
	itineraryID, err := s.itineraries.FindItineraryIDByRequest(ctx, ownerID, requestID)
	if err != nil {
		if errors.Is(err, ErrItineraryNotFound) {
			return domain.ErrRequestNotFound
		}
		return err
	}
	_, err = s.DecideRequest(ctx, ownerID, itineraryID, requestID, decision)
	return err
}

// DecideRequest acepta o rechaza una solicitud pendiente (solo el dueño). Aceptar otorga el permiso;
// rechazar bloquea el acceso del solicitante. [ADR-13]
func (s *Service) DecideRequest(ctx context.Context, ownerID, itineraryID, requestID string, decision ports.AccessDecision) (domain.RequestStatus, error) {
	if decision != ports.DecisionAccept && decision != ports.DecisionReject {
		return "", fmt.Errorf("%w: la decisión debe ser ACEPTAR o RECHAZAR", ErrValidation)
	}
	it, err := s.ownedItinerary(ctx, ownerID, itineraryID)
	if err != nil {
		return "", err
	}
	req, permission, err := it.Decide(requestID, decision == ports.DecisionAccept)
	if err != nil {
		return "", err
	}
	// Primero el permiso: si falla el segundo paso, reintentar la decisión es idempotente.
	if permission != nil {
		if err := s.itineraries.SavePermission(ctx, ownerID, itineraryID, *permission); err != nil {
			return "", err
		}
	}
	if err := s.itineraries.SaveRequest(ctx, ownerID, itineraryID, req); err != nil {
		return "", err
	}
	return req.Status, nil
}

// Revoke desautoriza a un viajero (solo el dueño).
func (s *Service) Revoke(ctx context.Context, ownerID, itineraryID, travelerID string) error {
	it, err := s.ownedItinerary(ctx, ownerID, itineraryID)
	if err != nil {
		return err
	}
	permission, err := it.Revoke(travelerID)
	if err != nil {
		return err
	}
	return s.itineraries.SavePermission(ctx, ownerID, itineraryID, permission)
}

// ownedItinerary carga el itinerario y exige que lo gestione su dueño (ErrForbidden en otro caso). [QA-09]
func (s *Service) ownedItinerary(ctx context.Context, ownerID, itineraryID string) (domain.SharedItinerary, error) {
	it, err := s.itineraries.FindByID(ctx, itineraryID)
	if err != nil {
		return domain.SharedItinerary{}, err
	}
	if ownerID == "" || it.OwnerID != ownerID {
		return domain.SharedItinerary{}, ErrForbidden
	}
	return it, nil
}

func uniqueIDs(ids []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func newID(prefix string) (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%x", prefix, b[:]), nil
}
