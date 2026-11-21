package domain

// Subtipos de ArrangementDetail (AD §2.1: Alojamiento, Vuelo, AutoRentado, Tren, Autobús).

// Accommodation es el detalle de una reserva de alojamiento.
type Accommodation struct {
	HotelName string
	Address   string
	Stars     int
}

// ArrangementType implementa ArrangementDetail.
func (Accommodation) ArrangementType() ArrangementType { return TypeAccommodation }

// Flight es el detalle de un vuelo.
type Flight struct {
	Airline         string
	FlightNumber    string
	OriginIATA      string
	DestinationIATA string
}

// ArrangementType implementa ArrangementDetail.
func (Flight) ArrangementType() ArrangementType { return TypeFlight }

// RentalCar es el detalle de un auto rentado.
type RentalCar struct {
	Company         string
	PickupLocation  string
	DropoffLocation string
}

// ArrangementType implementa ArrangementDetail.
func (RentalCar) ArrangementType() ArrangementType { return TypeRentalCar }

// Train es el detalle de un viaje en tren.
type Train struct {
	Operator    string
	Origin      string
	Destination string
}

// ArrangementType implementa ArrangementDetail.
func (Train) ArrangementType() ArrangementType { return TypeTrain }

// Bus es el detalle de un viaje en autobús.
type Bus struct {
	Operator    string
	Origin      string
	Destination string
}

// ArrangementType implementa ArrangementDetail.
func (Bus) ArrangementType() ArrangementType { return TypeBus }
