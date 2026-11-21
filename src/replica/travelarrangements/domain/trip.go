// Package domain (travelarrangements) modela el agregado "Viaje" (AD §2.2).
package domain

import "time"

// TripStatus es el estado del viaje; al completarse se retiran sus reservas de la vista (US03).
type TripStatus string

// Estados del viaje.
const (
	TripActive    TripStatus = "ACTIVO"
	TripCompleted TripStatus = "COMPLETADO"
)

// Trip es la raíz del agregado Viaje.
// Invariantes: todo arreglo pertenece a un único viaje; un arreglo recibido de varias fuentes
// se identifica y no se duplica (US01). Clave de partición: TravelerID [ADR-11].
type Trip struct {
	TripID       string
	TravelerID   string
	Name         string
	Destination  string
	StartDate    time.Time
	EndDate      time.Time
	Status       TripStatus
	Arrangements []TravelArrangement
}
