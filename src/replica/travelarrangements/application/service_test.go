package application

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// fakeTrips y fakeArrangements son repositorios en memoria aislados por viajero.
type fakeTrips struct {
	mu    sync.Mutex
	trips map[string]domain.Trip // clave: idViaje
}

func (f *fakeTrips) ListByTraveler(_ context.Context, travelerID string) ([]domain.Trip, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Trip
	for _, t := range f.trips {
		if t.TravelerID == travelerID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeTrips) FindByID(_ context.Context, travelerID, tripID string) (domain.Trip, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.trips[tripID]; ok && t.TravelerID == travelerID {
		return t, nil
	}
	return domain.Trip{}, ErrNotFound
}

func (f *fakeTrips) Save(_ context.Context, t domain.Trip) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.trips[t.TripID]; !exists {
		f.trips[t.TripID] = t
	}
	return nil
}

type fakeArrangements struct {
	mu   sync.Mutex
	rows map[string]domain.TravelArrangement
}

func (f *fakeArrangements) ListSummariesByTraveler(_ context.Context, travelerID string) ([]domain.TravelArrangement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.TravelArrangement
	for _, a := range f.rows {
		if a.TravelerID == travelerID {
			a.Detail = nil // el listado nunca trae detalle
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeArrangements) FindByID(_ context.Context, travelerID, id string) (domain.TravelArrangement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.rows[id]; ok && a.TravelerID == travelerID {
		return a, nil
	}
	return domain.TravelArrangement{}, ErrNotFound
}

func (f *fakeArrangements) FindByDeduplicationKey(context.Context, string, domain.DeduplicationKey) (domain.TravelArrangement, error) {
	return domain.TravelArrangement{}, ErrNotFound
}

func (f *fakeArrangements) Save(_ context.Context, a domain.TravelArrangement) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[a.ArrangementID] = a
	return nil
}

func (f *fakeArrangements) Delete(_ context.Context, travelerID, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.rows[id]; ok && a.TravelerID == travelerID {
		delete(f.rows, id)
		return nil
	}
	return ErrNotFound
}

var fixedNow = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func newTestService() (*Service, *fakeTrips, *fakeArrangements) {
	trips := &fakeTrips{trips: map[string]domain.Trip{}}
	arrs := &fakeArrangements{rows: map[string]domain.TravelArrangement{}}
	return NewService(trips, arrs, nil, func() time.Time { return fixedNow }), trips, arrs
}

func day(offset int) time.Time {
	return time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC).AddDate(0, 0, offset)
}

func validInput(tripID string) ArrangementInput {
	return ArrangementInput{
		TripID: tripID, Type: domain.TypeFlight, Title: "Vuelo a Lima",
		StartAt: day(10), EndAt: day(10).Add(3 * time.Hour),
		Detail: map[string]any{"aerolinea": "LATAM", "numeroVuelo": "LA800"},
	}
}

func TestDashboardGroupsByTripAndExcludesCompletedTrips(t *testing.T) {
	svc, trips, arrs := newTestService()
	ctx := context.Background()
	trips.trips["t-activo"] = domain.Trip{TripID: "t-activo", TravelerID: "v1", Name: "Perú", StartDate: day(5), EndDate: day(15), Status: domain.TripActive}
	trips.trips["t-proximo"] = domain.Trip{TripID: "t-proximo", TravelerID: "v1", Name: "Chile", StartDate: day(40), EndDate: day(50), Status: domain.TripActive}
	trips.trips["t-completado"] = domain.Trip{TripID: "t-completado", TravelerID: "v1", Name: "Viejo", StartDate: day(-30), EndDate: day(-20), Status: domain.TripCompleted}
	trips.trips["t-vencido"] = domain.Trip{TripID: "t-vencido", TravelerID: "v1", Name: "Terminó por fecha", StartDate: day(-30), EndDate: day(-2), Status: domain.TripActive}
	trips.trips["t-otro"] = domain.Trip{TripID: "t-otro", TravelerID: "v2", Name: "De otro viajero", StartDate: day(5), EndDate: day(9), Status: domain.TripActive}
	for id, a := range map[string]domain.TravelArrangement{
		"a2": {ArrangementID: "a2", TripID: "t-activo", TravelerID: "v1", Title: "Hotel", Type: domain.TypeAccommodation, StartAt: day(6)},
		"a1": {ArrangementID: "a1", TripID: "t-activo", TravelerID: "v1", Title: "Vuelo", Type: domain.TypeFlight, StartAt: day(5)},
		"a3": {ArrangementID: "a3", TripID: "t-completado", TravelerID: "v1", Title: "Oculto", Type: domain.TypeBus, StartAt: day(-25)},
	} {
		arrs.rows[id] = a
	}

	d, err := svc.GetDashboard(ctx, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Trips) != 2 || d.Trips[0].TripID != "t-activo" || d.Trips[1].TripID != "t-proximo" {
		t.Fatalf("viajes inesperados (completados/vencidos/ajenos deben quedar fuera, orden por fecha): %+v", d.Trips)
	}
	got := d.Trips[0].Arrangements
	if len(got) != 2 || got[0].ArrangementID != "a1" || got[1].ArrangementID != "a2" {
		t.Fatalf("los arreglos del viaje deben ir ordenados por fecha: %+v", got)
	}
	for _, a := range got {
		if a.Detail != nil {
			t.Fatalf("el dashboard no debe cargar detalle [ADR-17]: %+v", a)
		}
	}
	if len(d.Trips[1].Arrangements) != 0 {
		t.Fatalf("el viaje sin reservas debe venir vacío")
	}
	if d.ProviderNotices == nil || len(d.ProviderNotices) != 0 {
		t.Fatalf("avisosProveedores debe ser lista vacía en esta fase: %#v", d.ProviderNotices)
	}
}

func TestCreateArrangementPersistsManualOriginWithTypedDetail(t *testing.T) {
	svc, trips, arrs := newTestService()
	ctx := context.Background()
	trips.trips["t1"] = domain.Trip{TripID: "t1", TravelerID: "v1", Status: domain.TripActive}

	id, err := svc.CreateArrangement(ctx, "v1", validInput("t1"))
	if err != nil || id == "" {
		t.Fatalf("alta: id=%q err=%v", id, err)
	}
	saved := arrs.rows[id]
	if saved.Origin != domain.OriginManual || saved.Status != domain.StatusConfirmed || saved.TravelerID != "v1" {
		t.Fatalf("el alta manual debe ser MANUAL/CONFIRMADO del viajero del token: %+v", saved)
	}
	flight, ok := saved.Detail.(*domain.Flight)
	if !ok || flight.Airline != "LATAM" || flight.FlightNumber != "LA800" {
		t.Fatalf("detalle tipado inesperado: %#v", saved.Detail)
	}
}

func TestCreateArrangementValidation(t *testing.T) {
	svc, trips, _ := newTestService()
	ctx := context.Background()
	trips.trips["t1"] = domain.Trip{TripID: "t1", TravelerID: "v1", Status: domain.TripActive}
	trips.trips["t-ajeno"] = domain.Trip{TripID: "t-ajeno", TravelerID: "v2", Status: domain.TripActive}

	cases := map[string]func(*ArrangementInput){
		"sin viaje":              func(in *ArrangementInput) { in.TripID = "" },
		"viaje inexistente":      func(in *ArrangementInput) { in.TripID = "nope" },
		"viaje de otro viajero":  func(in *ArrangementInput) { in.TripID = "t-ajeno" },
		"tipo desconocido":       func(in *ArrangementInput) { in.Type = "CRUCERO" },
		"sin título":             func(in *ArrangementInput) { in.Title = "   " },
		"título demasiado largo": func(in *ArrangementInput) { in.Title = strings.Repeat("x", maxTitleLength+1) },
		"sin fecha de inicio":    func(in *ArrangementInput) { in.StartAt = time.Time{} },
		"fin antes del inicio":   func(in *ArrangementInput) { in.EndAt = in.StartAt.Add(-time.Hour) },
		"detalle con clave ajena": func(in *ArrangementInput) {
			in.Detail = map[string]any{"hotel": "no es de un vuelo"}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := validInput("t1")
			mutate(&in)
			if _, err := svc.CreateArrangement(ctx, "v1", in); !errors.Is(err, ErrValidation) {
				t.Fatalf("se esperaba ErrValidation, vino %v", err)
			}
		})
	}
}

func TestUpdateKeepsOriginAndRejectsOthersArrangements(t *testing.T) {
	svc, trips, arrs := newTestService()
	ctx := context.Background()
	trips.trips["t1"] = domain.Trip{TripID: "t1", TravelerID: "v1", Status: domain.TripActive}
	arrs.rows["a1"] = domain.TravelArrangement{
		ArrangementID: "a1", TripID: "t1", TravelerID: "v1", Type: domain.TypeFlight, Origin: domain.OriginEmail,
		Sender: "ana@aerolinea.com", Status: domain.StatusModified, Title: "Original", DeduplicationKey: "k1", StartAt: day(3),
	}

	in := validInput("t1")
	in.Title = "Editado"
	if err := svc.UpdateArrangement(ctx, "v1", "a1", in); err != nil {
		t.Fatal(err)
	}
	got := arrs.rows["a1"]
	if got.Title != "Editado" || got.Origin != domain.OriginEmail || got.Sender != "ana@aerolinea.com" ||
		got.Status != domain.StatusModified || got.DeduplicationKey != "k1" || !got.UpdatedAt.Equal(fixedNow) {
		t.Fatalf("la edición debe conservar origen/remitente/estado/clave: %+v", got)
	}

	if err := svc.UpdateArrangement(ctx, "v2", "a1", in); !errors.Is(err, ErrNotFound) {
		t.Fatalf("arreglo de otro viajero: se esperaba ErrNotFound, vino %v", err)
	}
	if err := svc.UpdateArrangement(ctx, "v1", "nope", in); !errors.Is(err, ErrNotFound) {
		t.Fatalf("arreglo inexistente: se esperaba ErrNotFound, vino %v", err)
	}
	in.Title = ""
	if err := svc.UpdateArrangement(ctx, "v1", "a1", in); !errors.Is(err, ErrValidation) {
		t.Fatalf("edición inválida: se esperaba ErrValidation, vino %v", err)
	}
}

func TestGetDetailAndDeleteAreScopedToTheTraveler(t *testing.T) {
	svc, _, arrs := newTestService()
	ctx := context.Background()
	arrs.rows["a1"] = domain.TravelArrangement{ArrangementID: "a1", TravelerID: "v1", Type: domain.TypeBus, Detail: &domain.Bus{Operator: "Flecha"}}

	a, err := svc.GetArrangementDetail(ctx, "v1", "a1")
	if err != nil || a.Detail == nil {
		t.Fatalf("detalle propio: %+v %v", a, err)
	}
	if _, err := svc.GetArrangementDetail(ctx, "v2", "a1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("detalle ajeno: %v", err)
	}
	if err := svc.DeleteArrangement(ctx, "v2", "a1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("baja ajena: %v", err)
	}
	if err := svc.DeleteArrangement(ctx, "v1", "a1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteArrangement(ctx, "v1", "a1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("segunda baja: %v", err)
	}
}

func TestProvisionDemoTripIsDeterministicAndIdempotent(t *testing.T) {
	svc, trips, _ := newTestService()
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := svc.ProvisionDemoTrip(ctx, "v1"); err != nil {
			t.Fatal(err)
		}
	}
	if len(trips.trips) != 1 {
		t.Fatalf("debe haber un único viaje demo: %+v", trips.trips)
	}
	trip := trips.trips[DemoTripID("v1")]
	if trip.TravelerID != "v1" || trip.Status != domain.TripActive || !trip.EndDate.After(fixedNow) {
		t.Fatalf("viaje demo inesperado: %+v", trip)
	}
	// El viaje demo es visible en el dashboard y admite altas.
	if _, err := svc.CreateArrangement(ctx, "v1", validInput(DemoTripID("v1"))); err != nil {
		t.Fatal(err)
	}
}
