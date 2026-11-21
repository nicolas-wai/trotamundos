// Package domain (permissions) modela el agregado "Itinerario compartido" (AD §2.2).
package domain

import (
	"errors"
	"time"
)

// RequestStatus es el estado de una solicitud de acceso.
type RequestStatus string

// Estados de solicitud (AD §6.2).
const (
	RequestPending  RequestStatus = "PENDIENTE"
	RequestAccepted RequestStatus = "ACEPTADA"
	RequestRejected RequestStatus = "RECHAZADA"
)

// Errores de invariantes del agregado. [ADR-13][QA-09]
var (
	ErrOwnerAlreadyHasAccess = errors.New("el dueño ya tiene acceso al itinerario")
	ErrAlreadyAuthorized     = errors.New("el viajero ya está autorizado")
	ErrAccessBlocked         = errors.New("el dueño rechazó la solicitud de acceso")
	ErrRequestNotFound       = errors.New("solicitud de acceso no encontrada")
	ErrRequestNotPending     = errors.New("la solicitud ya fue decidida")
	ErrPermissionNotFound    = errors.New("el viajero no tiene un acceso vigente")
)

// SharedItinerary es la raíz del agregado Itinerario compartido.
// Invariantes: solo ven el itinerario los autorizados (QA09, 0 accesos no autorizados);
// todo acceso nuevo requiere decisión del dueño [ADR-13].
// Partición: idViajero del dueño ◇ [ADR-11].
type SharedItinerary struct {
	ItineraryID    string
	OwnerID        string
	Name           string
	PublicLink     string
	ArrangementIDs []string
	// Arrangements son las reservas referenciadas con datos críticos (solo se cargan al ver el itinerario).
	Arrangements []ArrangementRef
	Permissions  []AccessPermission
	Requests     []AccessRequest
}

// ArrangementRef es la vista reducida de una reserva referenciada por el itinerario (payload reducido, ADR-17).
type ArrangementRef struct {
	ArrangementID string
	Type          string
	Title         string
	StartAt       time.Time
	Status        string
	Origin        string
}

// AccessPermission es la autorización vigente y revocable de un viajero. [ADR-13]
type AccessPermission struct {
	AuthorizedTravelerID string
	Active               bool
}

// AccessRequest es la solicitud pendiente de un viajero llegado por el link. [ADR-13]
type AccessRequest struct {
	RequestID           string
	RequesterTravelerID string
	Status              RequestStatus
	RequestedAt         time.Time
}

// CanView indica si el viajero puede ver el itinerario: solo el dueño y los autorizados vigentes. [QA-09]
// Un anónimo (travelerID vacío), un solicitante pendiente, uno rechazado o uno revocado no ven nada.
func (s SharedItinerary) CanView(travelerID string) bool {
	if travelerID == "" {
		return false
	}
	if travelerID == s.OwnerID {
		return true
	}
	for _, p := range s.Permissions {
		if p.AuthorizedTravelerID == travelerID && p.Active {
			return true
		}
	}
	return false
}

// CanRequestAccess indica si a quien recibe un 403 se le ofrece solicitar acceso: no si ya es dueño/autorizado
// ni si el dueño ya rechazó su solicitud (el rechazo bloquea). [ADR-13]
func (s SharedItinerary) CanRequestAccess(travelerID string) bool {
	return travelerID != "" && !s.CanView(travelerID) && !s.isBlocked(travelerID)
}

func (s SharedItinerary) isBlocked(travelerID string) bool {
	for _, r := range s.Requests {
		if r.RequesterTravelerID == travelerID && r.Status == RequestRejected {
			return true
		}
	}
	return false
}

func (s SharedItinerary) pendingFor(travelerID string) (AccessRequest, bool) {
	for _, r := range s.Requests {
		if r.RequesterTravelerID == travelerID && r.Status == RequestPending {
			return r, true
		}
	}
	return AccessRequest{}, false
}

// NewAccessRequest valida y crea una solicitud PENDIENTE. Si el viajero ya tiene una pendiente la devuelve con
// created=false (idempotente: no se duplican solicitudes). Todo acceso nuevo requiere decisión del dueño. [ADR-13]
func (s SharedItinerary) NewAccessRequest(travelerID, requestID string, now time.Time) (req AccessRequest, created bool, err error) {
	switch {
	case travelerID == s.OwnerID:
		return AccessRequest{}, false, ErrOwnerAlreadyHasAccess
	case s.CanView(travelerID):
		return AccessRequest{}, false, ErrAlreadyAuthorized
	case s.isBlocked(travelerID):
		return AccessRequest{}, false, ErrAccessBlocked
	}
	if pending, ok := s.pendingFor(travelerID); ok {
		return pending, false, nil
	}
	return AccessRequest{RequestID: requestID, RequesterTravelerID: travelerID, Status: RequestPending, RequestedAt: now}, true, nil
}

// Decide aplica la decisión del dueño a una solicitud PENDIENTE. Devuelve la solicitud ya decidida y, si se
// aceptó, el permiso a otorgar (nil si se rechazó: el rechazo no concede nada). [ADR-13]
func (s SharedItinerary) Decide(requestID string, accept bool) (AccessRequest, *AccessPermission, error) {
	for _, r := range s.Requests {
		if r.RequestID != requestID {
			continue
		}
		if r.Status != RequestPending {
			return AccessRequest{}, nil, ErrRequestNotPending
		}
		if !accept {
			r.Status = RequestRejected
			return r, nil, nil
		}
		r.Status = RequestAccepted
		return r, &AccessPermission{AuthorizedTravelerID: r.RequesterTravelerID, Active: true}, nil
	}
	return AccessRequest{}, nil, ErrRequestNotFound
}

// Revoke desautoriza a un viajero con acceso vigente. Devuelve el permiso ya inactivo.
func (s SharedItinerary) Revoke(travelerID string) (AccessPermission, error) {
	for _, p := range s.Permissions {
		if p.AuthorizedTravelerID == travelerID && p.Active {
			p.Active = false
			return p, nil
		}
	}
	return AccessPermission{}, ErrPermissionNotFound
}
