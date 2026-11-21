package domain

// Subtipos de ArrangementDetail (AD §2.1: Alojamiento, Vuelo, AutoRentado, Tren, Autobús).
// [QA-10] Agregar un tipo de arreglo = nuevo subtipo + registro en el ArrangementTypeRegistry, sin tocar el resto del dominio. [ADR-02]
// Las etiquetas json definen las claves de "detalle" del contrato (AD §6.2), que es un objeto abierto por tipo.

// Accommodation es el detalle de una reserva de alojamiento.
type Accommodation struct {
	HotelName string `json:"hotel,omitempty"`
	Address   string `json:"direccion,omitempty"`
	Stars     int    `json:"estrellas,omitempty"`
}

// ArrangementType implementa ArrangementDetail.
func (Accommodation) ArrangementType() ArrangementType { return TypeAccommodation }

// Flight es el detalle de un vuelo.
type Flight struct {
	Airline         string `json:"aerolinea,omitempty"`
	FlightNumber    string `json:"numeroVuelo,omitempty"`
	OriginIATA      string `json:"origenIATA,omitempty"`
	DestinationIATA string `json:"destinoIATA,omitempty"`
}

// ArrangementType implementa ArrangementDetail.
func (Flight) ArrangementType() ArrangementType { return TypeFlight }

// RentalCar es el detalle de un auto rentado.
type RentalCar struct {
	Company         string `json:"empresa,omitempty"`
	PickupLocation  string `json:"retiro,omitempty"`
	DropoffLocation string `json:"devolucion,omitempty"`
}

// ArrangementType implementa ArrangementDetail.
func (RentalCar) ArrangementType() ArrangementType { return TypeRentalCar }

// Train es el detalle de un viaje en tren.
type Train struct {
	Operator    string `json:"operador,omitempty"`
	Origin      string `json:"origen,omitempty"`
	Destination string `json:"destino,omitempty"`
}

// ArrangementType implementa ArrangementDetail.
func (Train) ArrangementType() ArrangementType { return TypeTrain }

// Bus es el detalle de un viaje en autobús.
type Bus struct {
	Operator    string `json:"operador,omitempty"`
	Origin      string `json:"origen,omitempty"`
	Destination string `json:"destino,omitempty"`
}

// ArrangementType implementa ArrangementDetail.
func (Bus) ArrangementType() ArrangementType { return TypeBus }
