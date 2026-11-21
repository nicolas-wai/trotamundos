package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// dedupArrangements agrega la búsqueda por claveDeduplicacion al repositorio falso.
type dedupArrangements struct{ *fakeArrangements }

func (d dedupArrangements) FindByDeduplicationKey(_ context.Context, travelerID string, key domain.DeduplicationKey) (domain.TravelArrangement, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, a := range d.rows {
		if a.TravelerID == travelerID && a.DeduplicationKey == key {
			return a, nil
		}
	}
	return domain.TravelArrangement{}, ErrNotFound
}

type fakeNotices struct {
	mu   sync.Mutex
	rows map[string]ProviderNoticeRecord
}

func (f *fakeNotices) Save(_ context.Context, n ProviderNoticeRecord) error {
	f.mu.Lock()
	f.rows[n.ProviderID] = n
	f.mu.Unlock()
	return nil
}
func (f *fakeNotices) Clear(_ context.Context, id string) error {
	f.mu.Lock()
	delete(f.rows, id)
	f.mu.Unlock()
	return nil
}
func (f *fakeNotices) List(context.Context) ([]ProviderNoticeRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []ProviderNoticeRecord
	for _, n := range f.rows {
		out = append(out, n)
	}
	return out, nil
}

func ingestionService() (*Service, *fakeTrips, dedupArrangements, *fakeNotices) {
	trips := &fakeTrips{trips: map[string]domain.Trip{}}
	arrs := dedupArrangements{&fakeArrangements{rows: map[string]domain.TravelArrangement{}}}
	notices := &fakeNotices{rows: map[string]ProviderNoticeRecord{}}
	svc := NewService(trips, arrs, nil, func() time.Time { return fixedNow }).WithProviderNotices(notices)
	return svc, trips, arrs, notices
}

func e1(provider, tipoEvento, estado string, sender *string, detail map[string]any) events.ProviderUpdateNormalized {
	return events.ProviderUpdateNormalized{
		EventID: "e-" + provider + tipoEvento, TipoEvento: tipoEvento, IDViajero: "v1",
		Proveedor: events.ProviderRef{Tipo: provider, IDProveedor: provider + ":x"}, OrigenRemitente: sender,
		GeneradoEnFuente: fixedNow.Add(-time.Minute), RecibidoEn: fixedNow,
		Arreglo: events.NormalizedArrangement{
			Tipo: "VUELO", ClaveDeduplicacion: "VUELO|sa123|2026-12-10", Estado: estado,
			FechaInicio: time.Date(2026, 12, 10, 10, 0, 0, 0, time.UTC), FechaFin: time.Date(2026, 12, 10, 14, 0, 0, 0, time.UTC),
			Detalle: detail,
		},
	}
}

func only(t *testing.T, arrs dedupArrangements) domain.TravelArrangement {
	t.Helper()
	if len(arrs.rows) != 1 {
		t.Fatalf("se esperaba 1 arreglo, hay %d", len(arrs.rows))
	}
	for _, a := range arrs.rows {
		return a
	}
	panic("unreachable")
}

// Crear: origen y remitente marcados, recibidoEn guardado (QA-01), viaje demo provisionado si no hay.
func TestApplyProviderUpdateCreatesWithOriginSenderAndReceivedAt(t *testing.T) {
	svc, trips, arrs, _ := ingestionService()
	sender := "reservas@solair.example"
	err := svc.ApplyProviderUpdate(context.Background(), e1("EMAIL", "ARREGLO_CREADO", "CONFIRMADO", &sender,
		map[string]any{"titulo": "Vuelo a Madrid", "numeroVuelo": "SA123", "aerolinea": "Sol Air", "xtra": "ignorado"}))
	if err != nil {
		t.Fatal(err)
	}
	a := only(t, arrs)
	if a.Origin != domain.OriginEmail || a.Sender != "reservas@solair.example" || a.Status != domain.StatusConfirmed ||
		a.Title != "Vuelo a Madrid" || a.DeduplicationKey != "VUELO|sa123|2026-12-10" || !a.ReceivedAt.Equal(fixedNow) ||
		!a.GeneratedAtSource.Equal(fixedNow.Add(-time.Minute)) || a.TravelerID != "v1" {
		t.Fatalf("arreglo inesperado: %+v", a)
	}
	if f, ok := a.Detail.(*domain.Flight); !ok || f.FlightNumber != "SA123" || f.Airline != "Sol Air" {
		t.Fatalf("detalle tipado: %#v", a.Detail)
	}
	if _, ok := trips.trips[DemoTripID("v1")]; !ok || a.TripID != DemoTripID("v1") {
		t.Fatalf("sin viajes se provisiona el demo: %v / %s", trips.trips, a.TripID)
	}
}

// La misma reserva recibida de otra fuente se fusiona (US01): no se duplica, conserva el origen y completa remitente y detalle.
func TestApplyProviderUpdateMergesDuplicateFromAnotherSource(t *testing.T) {
	svc, _, arrs, _ := ingestionService()
	ctx := context.Background()
	_ = svc.ApplyProviderUpdate(ctx, e1("VIAJES", "ARREGLO_CREADO", "CONFIRMADO", nil, map[string]any{"titulo": "Vuelo SA123 EZE-MAD", "numeroVuelo": "SA123", "origenIATA": "EZE"}))
	first := only(t, arrs)
	sender := "reservas@solair.example"
	if err := svc.ApplyProviderUpdate(ctx, e1("EMAIL", "ARREGLO_CREADO", "CONFIRMADO", &sender, map[string]any{"titulo": "Confirmación", "numeroVuelo": "SA123", "origenIATA": "XXX", "destinoIATA": "MAD"})); err != nil {
		t.Fatal(err)
	}
	a := only(t, arrs) // sigue habiendo uno solo
	f := a.Detail.(*domain.Flight)
	if a.ArrangementID != first.ArrangementID || a.Origin != domain.OriginTravelSystem || a.Sender != "reservas@solair.example" ||
		a.Title != "Vuelo SA123 EZE-MAD" || f.OriginIATA != "EZE" || f.DestinationIATA != "MAD" {
		t.Fatalf("fusión inesperada: %+v %+v", a, f)
	}
}

// Modificar y cancelar sobre una reserva existente.
func TestApplyProviderUpdateModifiesAndCancels(t *testing.T) {
	svc, _, arrs, _ := ingestionService()
	ctx := context.Background()
	_ = svc.ApplyProviderUpdate(ctx, e1("AGENCIA", "ARREGLO_CREADO", "CONFIRMADO", nil, map[string]any{"numeroVuelo": "SA123", "origenIATA": "EZE"}))

	mod := e1("AGENCIA", "ARREGLO_MODIFICADO", "MODIFICADO", nil, map[string]any{"numeroVuelo": "SA123", "origenIATA": "COR"})
	mod.Arreglo.FechaInicio = mod.Arreglo.FechaInicio.Add(2 * time.Hour)
	if err := svc.ApplyProviderUpdate(ctx, mod); err != nil {
		t.Fatal(err)
	}
	a := only(t, arrs)
	if a.Status != domain.StatusModified || a.Detail.(*domain.Flight).OriginIATA != "COR" || !a.StartAt.Equal(mod.Arreglo.FechaInicio) {
		t.Fatalf("modificación: %+v", a)
	}
	if err := svc.ApplyProviderUpdate(ctx, e1("AGENCIA", "ARREGLO_CANCELADO", "CANCELADO", nil, map[string]any{"numeroVuelo": "SA123"})); err != nil {
		t.Fatal(err)
	}
	if a = only(t, arrs); a.Status != domain.StatusCancelled {
		t.Fatalf("cancelación: %+v", a)
	}
}

// El viaje elegido es el vigente que contiene la fecha; si ninguno la contiene, el primero vigente.
func TestApplyProviderUpdatePicksTheTripContainingTheDate(t *testing.T) {
	svc, trips, arrs, _ := ingestionService()
	trips.trips["t-nov"] = domain.Trip{TripID: "t-nov", TravelerID: "v1", StartDate: day(0), EndDate: day(5), Status: domain.TripActive}
	trips.trips["t-dic"] = domain.Trip{TripID: "t-dic", TravelerID: "v1", StartDate: time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC), Status: domain.TripActive}
	trips.trips["t-viejo"] = domain.Trip{TripID: "t-viejo", TravelerID: "v1", StartDate: day(-40), EndDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), Status: domain.TripCompleted}
	_ = svc.ApplyProviderUpdate(context.Background(), e1("AGENCIA", "ARREGLO_CREADO", "CONFIRMADO", nil, map[string]any{"numeroVuelo": "SA123"}))
	if a := only(t, arrs); a.TripID != "t-dic" {
		t.Fatalf("viaje %s, se esperaba t-dic", a.TripID)
	}
	u := e1("AGENCIA", "ARREGLO_CREADO", "CONFIRMADO", nil, map[string]any{"numeroVuelo": "ZZ9"})
	u.Arreglo.ClaveDeduplicacion = "VUELO|zz9|2027-06-01"
	u.Arreglo.FechaInicio = time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	_ = svc.ApplyProviderUpdate(context.Background(), u)
	for _, a := range arrs.rows {
		if a.DeduplicationKey == "VUELO|zz9|2027-06-01" && a.TripID != "t-nov" {
			t.Fatalf("fuera de rango → primer viaje vigente (t-nov), fue %s", a.TripID)
		}
	}
}

func TestApplyProviderUpdateRejectsInvalid(t *testing.T) {
	svc, _, arrs, _ := ingestionService()
	cases := map[string]func(*events.ProviderUpdateNormalized){
		"sin viajero": func(u *events.ProviderUpdateNormalized) { u.IDViajero = "" },
		"tipo":        func(u *events.ProviderUpdateNormalized) { u.Arreglo.Tipo = "CRUCERO" },
		"sin clave":   func(u *events.ProviderUpdateNormalized) { u.Arreglo.ClaveDeduplicacion = "" },
		"sin fecha":   func(u *events.ProviderUpdateNormalized) { u.Arreglo.FechaInicio = time.Time{} },
		"tipo evento": func(u *events.ProviderUpdateNormalized) { u.TipoEvento = "X" },
		"estado":      func(u *events.ProviderUpdateNormalized) { u.Arreglo.Estado = "X" },
	}
	for name, mutate := range cases {
		u := e1("EMAIL", "ARREGLO_CREADO", "CONFIRMADO", nil, map[string]any{"numeroVuelo": "SA123"})
		mutate(&u)
		if err := svc.ApplyProviderUpdate(context.Background(), u); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(arrs.rows) != 0 {
		t.Fatal("nada inválido debe persistirse")
	}
}

// E2 → avisosProveedores en el dashboard; el cierre del circuito retira el aviso; los avisos propios no se filtran a otros.
func TestProviderDegradedBecomesDashboardNoticeAndClears(t *testing.T) {
	svc, trips, _, _ := ingestionService()
	_ = svc.ProvisionDemoTrip(context.Background(), "v1")
	_ = trips
	ctx := context.Background()
	open := events.ProviderDegraded{IDProveedor: "agencia:agencia-sol", Tipo: "AGENCIA", EstadoCircuito: "ABIERTO", Reintentos: 3, ProximoIntento: fixedNow.Add(time.Minute)}
	if err := svc.ReportProviderDegraded(ctx, open); err != nil {
		t.Fatal(err)
	}
	mine := "v1"
	own := events.ProviderDegraded{IDProveedor: "email:gmail", Tipo: "EMAIL", IDViajero: &mine, EstadoCircuito: "ABIERTO"}
	_ = svc.ReportProviderDegraded(ctx, own)

	d, err := svc.GetDashboard(ctx, "v1")
	if err != nil || len(d.ProviderNotices) != 2 {
		t.Fatalf("%v %+v", err, d.ProviderNotices)
	}
	got := map[string]string{}
	for _, n := range d.ProviderNotices {
		got[n.Provider] = n.Message
	}
	if got["Agencia Sol"] != "No se pudo conectar con el proveedor" || got["Gmail"] == "" {
		t.Fatalf("avisos: %v", got)
	}
	if other, _ := svc.GetDashboard(ctx, "v2"); len(other.ProviderNotices) != 1 {
		t.Fatalf("v2 solo ve el aviso global: %+v", other.ProviderNotices)
	}

	closed := open
	closed.EstadoCircuito = "CERRADO"
	_ = svc.ReportProviderDegraded(ctx, closed)
	if d, _ = svc.GetDashboard(ctx, "v1"); len(d.ProviderNotices) != 1 || d.ProviderNotices[0].Provider != "Gmail" {
		t.Fatalf("el cierre retira el aviso: %+v", d.ProviderNotices)
	}
}
