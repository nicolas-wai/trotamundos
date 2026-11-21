package broker

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
)

type recordingQueue struct {
	mu  sync.Mutex
	got []events.ProviderUpdateNormalized
}

func (q *recordingQueue) Publish(_ context.Context, e events.ProviderUpdateNormalized) error {
	q.mu.Lock()
	q.got = append(q.got, e)
	q.mu.Unlock()
	return nil
}
func (q *recordingQueue) Consume(context.Context) (events.ProviderUpdateNormalized, error) {
	return events.ProviderUpdateNormalized{}, errors.New("no usado")
}
func (q *recordingQueue) Ack(context.Context, string) error { return nil }

var received = time.Date(2027, 1, 5, 12, 0, 0, 0, time.UTC)

func flight(provider, number string, start time.Time, sender *string) events.ProviderUpdateNormalized {
	return events.ProviderUpdateNormalized{
		IDViajero: "v1", Proveedor: events.ProviderRef{Tipo: provider, IDProveedor: "x"}, OrigenRemitente: sender,
		GeneradoEnFuente: received.Add(-time.Minute),
		Arreglo: events.NormalizedArrangement{
			Tipo: "vuelo", Estado: "confirmado", FechaInicio: start, Detalle: map[string]any{"numeroVuelo": number},
		},
	}
}

func newBroker(q *recordingQueue) *Broker {
	b := NewBroker(q)
	b.SetClock(func() time.Time { return received })
	return b
}

// El broker produce un E1 completo y con los nombres JSON del contrato (ADR-06).
func TestNormalizeProducesValidE1WithContractNames(t *testing.T) {
	sender := " Reservas@SolAir.example "
	b := newBroker(&recordingQueue{})
	got, err := b.Normalize(flight("EMAIL", "SA 123", time.Date(2027, 3, 10, 7, 0, 0, 0, time.FixedZone("x", -3*3600)), &sender))
	if err != nil {
		t.Fatal(err)
	}
	if got.EventID == "" || got.TipoEvento != "ARREGLO_CREADO" || !got.RecibidoEn.Equal(received) ||
		got.Arreglo.Tipo != "VUELO" || got.Arreglo.Estado != "CONFIRMADO" || got.Arreglo.ClaveDeduplicacion != "VUELO|sa123|2027-03-10" ||
		got.OrigenRemitente == nil || *got.OrigenRemitente != "reservas@solair.example" || !got.Arreglo.FechaFin.Equal(got.Arreglo.FechaInicio) {
		t.Fatalf("E1 mal normalizado: %+v", got)
	}
	raw, _ := json.Marshal(got)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	for _, k := range []string{"eventId", "tipoEvento", "idViajero", "proveedor", "origenRemitente", "arreglo", "generadoEnFuente", "recibidoEn"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("falta %q en E1: %s", k, raw)
		}
	}
	arr := m["arreglo"].(map[string]any)
	for _, k := range []string{"tipo", "claveDeduplicacion", "estado", "fechaInicio", "fechaFin", "detalle"} {
		if _, ok := arr[k]; !ok {
			t.Fatalf("falta arreglo.%q: %s", k, raw)
		}
	}
}

// La misma reserva, venga de la fuente que venga (formato de número y zona horaria distintos), tiene la misma clave (US01).
func TestDeduplicationKeyIsIdenticalAcrossSources(t *testing.T) {
	b := newBroker(&recordingQueue{})
	a, _ := b.Normalize(flight("EMAIL", "SA123", time.Date(2027, 3, 10, 10, 0, 0, 0, time.UTC), nil))
	c, _ := b.Normalize(flight("VIAJES", "sa-123", time.Date(2027, 3, 10, 7, 0, 0, 0, time.FixedZone("x", -3*3600)), nil))
	d, _ := b.Normalize(flight("AGENCIA", "SA123", time.Date(2027, 3, 11, 10, 0, 0, 0, time.UTC), nil))
	if a.Arreglo.ClaveDeduplicacion != c.Arreglo.ClaveDeduplicacion {
		t.Fatalf("claves distintas: %q vs %q", a.Arreglo.ClaveDeduplicacion, c.Arreglo.ClaveDeduplicacion)
	}
	if a.Arreglo.ClaveDeduplicacion == d.Arreglo.ClaveDeduplicacion {
		t.Fatal("otro día = otra reserva")
	}
}

func TestEventTypeFollowsStateAndSenderOnlyForEmail(t *testing.T) {
	b := newBroker(&recordingQueue{})
	sender := "a@b.c"
	for estado, want := range map[string]string{"CONFIRMADO": "ARREGLO_CREADO", "MODIFICADO": "ARREGLO_MODIFICADO", "CANCELADO": "ARREGLO_CANCELADO"} {
		u := flight("AGENCIA", "SA1", received, &sender)
		u.Arreglo.Estado = estado
		got, err := b.Normalize(u)
		if err != nil || got.TipoEvento != want {
			t.Fatalf("%s → %q (%v), se esperaba %q", estado, got.TipoEvento, err, want)
		}
		if got.OrigenRemitente != nil {
			t.Fatal("el remitente solo aplica al correo")
		}
	}
}

// La lógica nunca recibe datos sin formatear: lo no normalizable no llega a la cola, lo válido sí.
func TestIngestDropsNonNormalizableAndPublishesTheRest(t *testing.T) {
	q := &recordingQueue{}
	b := newBroker(q)
	bad := []events.ProviderUpdateNormalized{
		{IDViajero: "", Proveedor: events.ProviderRef{Tipo: "EMAIL"}},
		func() events.ProviderUpdateNormalized {
			u := flight("EMAIL", "SA1", received, nil)
			u.Arreglo.Tipo = "CRUCERO"
			return u
		}(),
		func() events.ProviderUpdateNormalized {
			u := flight("EMAIL", "SA1", received, nil)
			u.Arreglo.Estado = "???"
			return u
		}(),
		func() events.ProviderUpdateNormalized { u := flight("EMAIL", "", received, nil); return u }(), // sin identidad
		func() events.ProviderUpdateNormalized { u := flight("OTRO", "SA1", received, nil); return u }(),
	}
	err := b.Ingest(context.Background(), append(bad, flight("EMAIL", "SA9", received, nil)))
	if !errors.Is(err, ErrNotNormalizable) {
		t.Fatalf("se esperaba ErrNotNormalizable: %v", err)
	}
	if len(q.got) != 1 || q.got[0].Arreglo.ClaveDeduplicacion != "VUELO|sa9|2027-01-05" || q.got[0].EventID == "" {
		t.Fatalf("solo el válido debe publicarse: %+v", q.got)
	}
}
