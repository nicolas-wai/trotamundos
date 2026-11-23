package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/permissions/domain"
)

// fakeRepo guarda los itinerarios en memoria (sin cola ni SQLite): aísla las reglas de autorización.
type fakeRepo struct {
	mu          sync.Mutex
	itineraries map[string]*domain.SharedItinerary
}

func newFakeRepo() *fakeRepo { return &fakeRepo{itineraries: map[string]*domain.SharedItinerary{}} }

func (r *fakeRepo) FindByID(_ context.Context, id string) (domain.SharedItinerary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	it, ok := r.itineraries[id]
	if !ok {
		return domain.SharedItinerary{}, ErrItineraryNotFound
	}
	cp := *it
	cp.Permissions = append([]domain.AccessPermission(nil), it.Permissions...)
	cp.Requests = append([]domain.AccessRequest(nil), it.Requests...)
	return cp, nil
}

func (r *fakeRepo) ListByOwner(_ context.Context, ownerID string) ([]domain.SharedItinerary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.SharedItinerary{}
	for _, it := range r.itineraries {
		if it.OwnerID == ownerID {
			cp := *it
			cp.Permissions = append([]domain.AccessPermission(nil), it.Permissions...)
			cp.Requests = append([]domain.AccessRequest(nil), it.Requests...)
			out = append(out, cp)
		}
	}
	return out, nil
}

func (r *fakeRepo) Create(_ context.Context, it domain.SharedItinerary) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.itineraries[it.ItineraryID] = &it
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, _, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.itineraries, id)
	return nil
}

func (r *fakeRepo) AddArrangement(_ context.Context, _, id, arr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.itineraries[id].ArrangementIDs = append(r.itineraries[id].ArrangementIDs, arr)
	return nil
}

func (r *fakeRepo) RemoveArrangement(_ context.Context, _, id, arr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	it := r.itineraries[id]
	kept := it.ArrangementIDs[:0]
	for _, a := range it.ArrangementIDs {
		if a != arr {
			kept = append(kept, a)
		}
	}
	it.ArrangementIDs = kept
	return nil
}

func (r *fakeRepo) SavePermission(_ context.Context, _, id string, p domain.AccessPermission) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	it := r.itineraries[id]
	for i := range it.Permissions {
		if it.Permissions[i].AuthorizedTravelerID == p.AuthorizedTravelerID {
			it.Permissions[i] = p
			return nil
		}
	}
	it.Permissions = append(it.Permissions, p)
	return nil
}

func (r *fakeRepo) SaveRequest(_ context.Context, _, id string, q domain.AccessRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	it := r.itineraries[id]
	for i := range it.Requests {
		if it.Requests[i].RequestID == q.RequestID {
			it.Requests[i] = q
			return nil
		}
	}
	it.Requests = append(it.Requests, q)
	return nil
}

func (r *fakeRepo) FindItineraryIDByRequest(_ context.Context, ownerID, requestID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, it := range r.itineraries {
		if it.OwnerID != ownerID {
			continue
		}
		for _, q := range it.Requests {
			if q.RequestID == requestID {
				return id, nil
			}
		}
	}
	return "", ErrItineraryNotFound
}

// fakeCatalog: el dueño "ana" tiene los arreglos a1 y a2.
type fakeCatalog struct{}

func (fakeCatalog) Exists(_ context.Context, owner, id string) (bool, error) {
	return owner == "ana" && (id == "a1" || id == "a2"), nil
}

func (fakeCatalog) Summaries(_ context.Context, _ string, ids []string) ([]domain.ArrangementRef, error) {
	refs := []domain.ArrangementRef{}
	for _, id := range ids {
		refs = append(refs, domain.ArrangementRef{ArrangementID: id, Title: "reserva " + id})
	}
	return refs, nil
}

type captureBus struct {
	mu     sync.Mutex
	events []events.AccessRequestCreated
}

func (b *captureBus) Publish(_ context.Context, topic string, payload any) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if topic == events.TopicAccessRequestCreated {
		b.events = append(b.events, payload.(events.AccessRequestCreated))
	}
	return nil
}
func (*captureBus) Subscribe(string, ports.EventHandler) {}

func newService() (*Service, *captureBus) {
	bus := &captureBus{}
	svc := NewService(newFakeRepo(), fakeCatalog{}, func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) })
	return svc.WithEventBus(bus), bus
}

// Matriz de autorización de QA09: solo el dueño y los autorizados vigentes ven el itinerario.
func TestAuthorizationMatrix(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService()
	it, err := svc.CreateItinerary(ctx, "ana", "Lisboa 2027", []string{"a1"})
	if err != nil {
		t.Fatal(err)
	}
	id := it.ItineraryID

	// Cuatro invitados en estados distintos.
	grant := func(traveler string) string {
		q, err := svc.RequestAccess(ctx, traveler, id)
		if err != nil {
			t.Fatal(err)
		}
		return q.RequestID
	}
	autorizado := grant("autorizado")
	if _, err := svc.DecideRequest(ctx, "ana", id, autorizado, ports.DecisionAccept); err != nil {
		t.Fatal(err)
	}
	grant("pendiente")
	rechazado := grant("rechazado")
	if st, err := svc.DecideRequest(ctx, "ana", id, rechazado, ports.DecisionReject); err != nil || st != domain.RequestRejected {
		t.Fatalf("rechazo: %v %v", st, err)
	}
	revocado := grant("revocado")
	if _, err := svc.DecideRequest(ctx, "ana", id, revocado, ports.DecisionAccept); err != nil {
		t.Fatal(err)
	}
	if err := svc.Revoke(ctx, "ana", id, "revocado"); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		traveler    string
		canView     bool
		canRequest  bool
		description string
	}{
		{"ana", true, false, "dueño"},
		{"autorizado", true, false, "autorizado"},
		{"pendiente", false, true, "pendiente (puede volver a solicitar sin duplicar)"},
		{"rechazado", false, false, "rechazado: el rechazo bloquea"},
		{"revocado", false, true, "revocado: puede pedir un acceso nuevo"},
		{"", false, false, "anónimo"},
		{"desconocido", false, true, "viajero sin relación"},
	}
	for _, c := range cases {
		ok, err := svc.CanView(ctx, c.traveler, id)
		if err != nil || ok != c.canView {
			t.Errorf("%s: CanView=%v err=%v, se esperaba %v", c.description, ok, err, c.canView)
		}
		_, err = svc.GetItinerary(ctx, c.traveler, id)
		if c.canView {
			if err != nil {
				t.Errorf("%s: GetItinerary falló: %v", c.description, err)
			}
			continue
		}
		var denied *AccessDeniedError
		if !errors.As(err, &denied) || !errors.Is(err, ErrAccessDenied) {
			t.Fatalf("%s: se esperaba AccessDeniedError, hubo %v", c.description, err)
		}
		if denied.CanRequest != c.canRequest {
			t.Errorf("%s: puedeSolicitar=%v, se esperaba %v", c.description, denied.CanRequest, c.canRequest)
		}
	}
}

func TestOnlyOwnerManagesTheItinerary(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService()
	it, _ := svc.CreateItinerary(ctx, "ana", "Viaje", []string{"a1"})
	q, _ := svc.RequestAccess(ctx, "beto", it.ItineraryID)

	for name, err := range map[string]error{
		"listar": func() error { _, e := svc.ListRequests(ctx, "beto", it.ItineraryID); return e }(),
		"decidir": func() error {
			_, e := svc.DecideRequest(ctx, "beto", it.ItineraryID, q.RequestID, ports.DecisionAccept)
			return e
		}(),
		"decidir (puerto)": svc.Decide(ctx, "beto", q.RequestID, ports.DecisionAccept),
		"revocar":          svc.Revoke(ctx, "beto", it.ItineraryID, "beto"),
		"eliminar":         svc.DeleteItinerary(ctx, "beto", it.ItineraryID),
		"agregar":          svc.AddArrangement(ctx, "beto", it.ItineraryID, "a2"),
		"quitar":           svc.RemoveArrangement(ctx, "beto", it.ItineraryID, "a1"),
	} {
		if name == "decidir (puerto)" {
			// el puerto resuelve la solicitud dentro del shard del llamador: para un no-dueño no existe
			if !errors.Is(err, domain.ErrRequestNotFound) {
				t.Errorf("%s: se esperaba ErrRequestNotFound, hubo %v", name, err)
			}
			continue
		}
		if !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: se esperaba ErrForbidden, hubo %v", name, err)
		}
	}
	// Tras los intentos de beto la solicitud sigue pendiente y beto sin acceso.
	if ok, _ := svc.CanView(ctx, "beto", it.ItineraryID); ok {
		t.Fatal("beto no debía tener acceso")
	}
	// El dueño sí decide por el puerto (solo con el id de la solicitud).
	if err := svc.Decide(ctx, "ana", q.RequestID, ports.DecisionAccept); err != nil {
		t.Fatal(err)
	}
	if ok, _ := svc.CanView(ctx, "beto", it.ItineraryID); !ok {
		t.Fatal("beto debía quedar autorizado")
	}
}

func TestRequestAccessRulesAndNotification(t *testing.T) {
	ctx := context.Background()
	svc, bus := newService()
	it, _ := svc.CreateItinerary(ctx, "ana", "Viaje", nil)

	q1, err := svc.RequestAccess(ctx, "beto", it.ItineraryID)
	if err != nil || q1.Status != domain.RequestPending {
		t.Fatalf("solicitud: %v %v", q1, err)
	}
	// Idempotente: no se duplica ni se vuelve a notificar.
	q2, _ := svc.RequestAccess(ctx, "beto", it.ItineraryID)
	if q2.RequestID != q1.RequestID || len(bus.events) != 1 {
		t.Fatalf("debía devolver la misma solicitud y notificar una vez: %v / %d eventos", q2, len(bus.events))
	}
	e7 := bus.events[0]
	if e7.IDItinerario != it.ItineraryID || e7.IDDueno != "ana" || e7.IDSolicitante != "beto" || e7.IDSolicitud != q1.RequestID {
		t.Fatalf("E7 inesperado: %+v", e7)
	}
	if _, err := svc.RequestAccess(ctx, "ana", it.ItineraryID); !errors.Is(err, domain.ErrOwnerAlreadyHasAccess) {
		t.Fatalf("el dueño no solicita acceso: %v", err)
	}
	// Decidir dos veces la misma solicitud → conflicto.
	if _, err := svc.DecideRequest(ctx, "ana", it.ItineraryID, q1.RequestID, ports.DecisionReject); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DecideRequest(ctx, "ana", it.ItineraryID, q1.RequestID, ports.DecisionAccept); !errors.Is(err, domain.ErrRequestNotPending) {
		t.Fatalf("segunda decisión: %v", err)
	}
	// El rechazo bloquea: no puede volver a solicitar.
	if _, err := svc.RequestAccess(ctx, "beto", it.ItineraryID); !errors.Is(err, domain.ErrAccessBlocked) {
		t.Fatalf("tras el rechazo: %v", err)
	}
	if _, err := svc.DecideRequest(ctx, "ana", it.ItineraryID, "no-existe", ports.DecisionAccept); !errors.Is(err, domain.ErrRequestNotFound) {
		t.Fatalf("solicitud inexistente: %v", err)
	}
	if _, err := svc.DecideRequest(ctx, "ana", it.ItineraryID, q1.RequestID, "TAL_VEZ"); !errors.Is(err, ErrValidation) {
		t.Fatalf("decisión inválida: %v", err)
	}
}

func TestListRequestsReturnsOnlyPending(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService()
	it, _ := svc.CreateItinerary(ctx, "ana", "Viaje", nil)
	q1, _ := svc.RequestAccess(ctx, "beto", it.ItineraryID)
	svc.RequestAccess(ctx, "caro", it.ItineraryID)
	svc.DecideRequest(ctx, "ana", it.ItineraryID, q1.RequestID, ports.DecisionAccept)
	pending, err := svc.ListRequests(ctx, "ana", it.ItineraryID)
	if err != nil || len(pending) != 1 || pending[0].RequesterTravelerID != "caro" {
		t.Fatalf("pendientes: %v %v", pending, err)
	}
}

func TestCreateAndArrangementValidation(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService()
	if _, err := svc.CreateItinerary(ctx, "ana", "  ", nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("nombre vacío: %v", err)
	}
	if _, err := svc.CreateItinerary(ctx, "ana", "X", []string{"a1", "ajeno"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("arreglo inexistente: %v", err)
	}
	it, err := svc.CreateItinerary(ctx, "ana", "X", []string{"a1", "a1", " a2 "})
	if err != nil || len(it.ArrangementIDs) != 2 || it.PublicLink != "/itinerarios/"+it.ItineraryID {
		t.Fatalf("creación: %+v %v", it, err)
	}
	if err := svc.AddArrangement(ctx, "ana", it.ItineraryID, "ajeno"); !errors.Is(err, ErrArrangementNotFound) {
		t.Fatalf("agregar ajeno: %v", err)
	}
	if err := svc.RemoveArrangement(ctx, "ana", it.ItineraryID, "a-que-no-esta"); !errors.Is(err, ErrArrangementNotFound) {
		t.Fatalf("quitar inexistente: %v", err)
	}
	if err := svc.RemoveArrangement(ctx, "ana", it.ItineraryID, "a1"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetItinerary(ctx, "ana", it.ItineraryID)
	if err != nil || len(got.Arrangements) != 1 || got.Arrangements[0].ArrangementID != "a2" {
		t.Fatalf("lectura del dueño: %+v %v", got, err)
	}
	if _, err := svc.GetItinerary(ctx, "ana", "inexistente"); !errors.Is(err, ErrItineraryNotFound) {
		t.Fatalf("itinerario inexistente: %v", err)
	}
}

// Fase 7: "Mis itinerarios" muestra reservas, accesos vigentes y solo solicitudes pendientes; no los de otro dueño.
func TestListOwnedItinerariesShowsPendingRequestsAndActiveAccessOnly(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()
	it, err := svc.CreateItinerary(ctx, "ana", "Europa", []string{"a1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{"beto", "caro", "dani"} {
		if _, err := svc.RequestAccess(ctx, who, it.ItineraryID); err != nil {
			t.Fatal(err)
		}
	}
	reqs, _ := svc.ListRequests(ctx, "ana", it.ItineraryID)
	_, _ = svc.DecideRequest(ctx, "ana", it.ItineraryID, reqs[0].RequestID, ports.DecisionAccept) // beto: acceso
	_, _ = svc.DecideRequest(ctx, "ana", it.ItineraryID, reqs[1].RequestID, ports.DecisionReject) // caro: rechazada
	if _, err := svc.CreateItinerary(ctx, "ana", "Otro", nil); err != nil {
		t.Fatal(err)
	}

	list, err := svc.ListOwnedItineraries(ctx, "ana")
	if err != nil || len(list) != 2 {
		t.Fatalf("itinerarios de ana: %v %v", list, err)
	}
	for _, got := range list {
		if got.ItineraryID != it.ItineraryID {
			continue
		}
		if len(got.Arrangements) != 1 || len(got.Permissions) != 1 || got.Permissions[0].AuthorizedTravelerID != "beto" {
			t.Fatalf("reservas/accesos: %+v", got)
		}
		if len(got.Requests) != 1 || got.Requests[0].RequesterTravelerID != "dani" {
			t.Fatalf("solo la pendiente de dani: %+v", got.Requests)
		}
	}
	if other, _ := svc.ListOwnedItineraries(ctx, "beto"); len(other) != 0 {
		t.Fatalf("beto no es dueño de nada: %v", other)
	}
}
