package domain

import "time"

// ArrangementType es el tipo de reserva. Se resuelve mediante ArrangementTypeRegistry [QA-10].
type ArrangementType string

// Tipos de arreglo del contrato (AD §6.2).
const (
	TypeFlight        ArrangementType = "VUELO"
	TypeAccommodation ArrangementType = "ALOJAMIENTO"
	TypeRentalCar     ArrangementType = "AUTO"
	TypeTrain         ArrangementType = "TREN"
	TypeBus           ArrangementType = "BUS"
)

// Origin es la fuente de un arreglo (manual, correo, agencia o sistema de viajes).
type Origin string

// Orígenes del contrato.
const (
	OriginManual       Origin = "MANUAL"
	OriginEmail        Origin = "EMAIL"
	OriginAgency       Origin = "AGENCIA"
	OriginTravelSystem Origin = "VIAJES"
)

// Status es el estado de un arreglo.
type Status string

// Estados del contrato.
const (
	StatusConfirmed Status = "CONFIRMADO"
	StatusCancelled Status = "CANCELADO"
	StatusModified  Status = "MODIFICADO"
)

// DeduplicationKey identifica una misma reserva recibida desde varias fuentes (US01).
type DeduplicationKey string

// Sender es el remitente del correo del que proviene el arreglo (US03). Vacío si no aplica.
type Sender string

// TravelArrangement es una reserva (hotel, auto, vuelo, tren, bus) dentro de un Trip.
type TravelArrangement struct {
	ArrangementID     string
	TripID            string
	TravelerID        string
	Type              ArrangementType
	Origin            Origin
	Sender            Sender
	Status            Status
	Title             string
	StartAt           time.Time
	EndAt             time.Time
	Detail            ArrangementDetail
	DeduplicationKey  DeduplicationKey
	GeneratedAtSource time.Time
	UpdatedAt         time.Time
}

// ArrangementDetail es el detalle específico de cada subtipo, cargado bajo demanda [ADR-17].
type ArrangementDetail interface {
	// ArrangementType devuelve el tipo al que pertenece el detalle.
	ArrangementType() ArrangementType
}
