// Package domain (permissions) modela el agregado "Itinerario compartido" (AD §2.2).
package domain

import "time"

// RequestStatus es el estado de una solicitud de acceso.
type RequestStatus string

// Estados de solicitud (AD §6.2).
const (
	RequestPending  RequestStatus = "PENDIENTE"
	RequestAccepted RequestStatus = "ACEPTADA"
	RequestRejected RequestStatus = "RECHAZADA"
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
	Permissions    []AccessPermission
	Requests       []AccessRequest
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
