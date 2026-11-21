package ssr

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

func sampleDashboard() application.Dashboard {
	start := time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC)
	return application.Dashboard{
		Trips: []domain.Trip{{
			TripID: "t1", Name: "Vacaciones <b>en Perú</b>", Destination: "Lima",
			StartDate: start, EndDate: start.AddDate(0, 0, 14),
			Arrangements: []domain.TravelArrangement{
				// Aunque llegara con detalle, el SSR del dashboard jamás lo imprime [ADR-17].
				{ArrangementID: "a1", Title: "Vuelo a Lima", Type: domain.TypeFlight, Origin: domain.OriginManual, Status: domain.StatusConfirmed,
					StartAt: start, Detail: &domain.Flight{Airline: "AEROLINEA-SECRETA", FlightNumber: "XX-9999"}},
				{ArrangementID: "a2", Title: "Hotel Miraflores", Type: domain.TypeAccommodation, Origin: domain.OriginEmail, Status: domain.StatusCancelled,
					StartAt: start.AddDate(0, 0, 1), Detail: &domain.Accommodation{Address: "CALLE-RESERVADA 123"}},
			},
		}, {TripID: "t2", Name: "Sin reservas", Destination: "Cusco"}},
		ProviderNotices: []application.ProviderNotice{{Provider: "Agencia Sol", Message: "No se pudo conectar con el proveedor"}},
	}
}

func TestRenderDashboardShowsCriticalDataOnly(t *testing.T) {
	html, err := NewSSRRenderer().RenderDashboard(context.Background(), sampleDashboard())
	if err != nil {
		t.Fatal(err)
	}
	out := string(html)
	for _, want := range []string{
		"Vuelo a Lima", "Hotel Miraflores", "Lima", "Vuelo", "Alojamiento", "Cancelado", "Correo", "Manual",
		`data-id="a1"`, `data-trip-id="t1"`, "Aún no hay reservas en este viaje.", "Agencia Sol", "No se pudo conectar con el proveedor",
		"01/12/2026 10:00",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("el HTML debe contener %q", want)
		}
	}
	for _, forbidden := range []string{"AEROLINEA-SECRETA", "XX-9999", "CALLE-RESERVADA", "numeroVuelo", "direccion"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("payload reducido: el HTML no debe contener datos de detalle (%q)", forbidden)
		}
	}
	if strings.Contains(out, "<b>en Perú</b>") || !strings.Contains(out, "&lt;b&gt;en Perú&lt;/b&gt;") {
		t.Errorf("html/template debe escapar los datos del viajero")
	}
}

func TestRenderDashboardWithoutTrips(t *testing.T) {
	html, err := NewSSRRenderer().RenderDashboard(context.Background(), application.Dashboard{})
	if err != nil || !strings.Contains(string(html), "No tienes viajes activos.") {
		t.Fatalf("dashboard vacío: %q %v", html, err)
	}
}

func TestRenderArrangementDetailIncludesTypedFields(t *testing.T) {
	a := domain.TravelArrangement{
		ArrangementID: "a1", Type: domain.TypeFlight, Origin: domain.OriginEmail, Sender: "ana@aerolinea.com", Status: domain.StatusConfirmed,
		Detail:    &domain.Flight{Airline: "LATAM", FlightNumber: "LA800", OriginIATA: "EZE", DestinationIATA: "LIM"},
		UpdatedAt: time.Date(2026, 11, 21, 9, 30, 0, 0, time.UTC),
	}
	html, err := NewSSRRenderer().RenderArrangementDetail(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Aerolínea", "LATAM", "N.º de vuelo", "LA800", "Origen (IATA)", "EZE", "ana@aerolinea.com", "21/11/2026 09:30"} {
		if !strings.Contains(string(html), want) {
			t.Errorf("el detalle debe contener %q: %s", want, html)
		}
	}
}
