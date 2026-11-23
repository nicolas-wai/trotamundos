package region_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/stubs"
)

// leaderIndex devuelve qué réplicas se consideran líder hoy.
func (e *env) leaders() []int {
	var out []int
	for i, r := range e.reg.Replicas {
		if r.Leader.IsLeader() {
			out = append(out, i)
		}
	}
	return out
}

// ADR-04/21: si cae la réplica 0, la 1 se vuelve líder y sigue ingiriendo; los avisos E2 siguen llegando.
func TestIntegrationsLeaderTakesOverWhenReplicaZeroFalls(t *testing.T) {
	e := pollingEnv(t, nil)
	_, ana := e.signUp(t, "ana@example.com")
	if got := e.leaders(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("al arrancar lidera la réplica 0: %v", got)
	}
	eventually(t, 5*time.Second, "agencia y sistema de viajes ingeridos por la réplica 0", func() bool {
		d := e.dashboardJSON(t, ana)
		return len(d.Viajes) == 1 && len(d.Viajes[0].Arreglos) == 3
	})

	e.reg.StopReplica(0)
	eventually(t, 5*time.Second, "la réplica 1 pasa a ser líder", func() bool { l := e.leaders(); return len(l) == 1 && l[0] == 1 })

	// Un correo nuevo (cuenta vinculada tras el relevo) lo ingiere la nueva líder, sin duplicar lo ya existente.
	if resp, body := e.do(t, "POST", "/correos", ana, linkBody("gmail", "mock-code-ana")); resp.StatusCode != 201 {
		t.Fatalf("vincular: %d %v", resp.StatusCode, body)
	}
	eventually(t, 5*time.Second, "los 4 arreglos tras el relevo", func() bool {
		d := e.dashboardJSON(t, ana)
		return len(d.Viajes) == 1 && len(d.Viajes[0].Arreglos) == 4
	})
	time.Sleep(3 * e.cfg.PollInterval) // la nueva líder vuelve a leer todo (since vacío): la deduplicación lo hace inofensivo
	if d := e.dashboardJSON(t, ana); len(d.Viajes[0].Arreglos) != 4 {
		t.Fatalf("el segundo sondeo completo no debe duplicar: %+v", d.Viajes[0].Arreglos)
	}

	// E2 después del relevo: el aviso del proveedor degradado llega al dashboard (una sola vez).
	e.reg.ExternalAPIs.Agency.SetFailureMode(stubs.FailUnavailable)
	eventually(t, 5*time.Second, "aviso E2 tras el relevo", func() bool { return len(e.dashboardJSON(t, ana).AvisosProveedores) == 1 })
	time.Sleep(3 * e.cfg.PollInterval)
	if n := len(e.dashboardJSON(t, ana).AvisosProveedores); n != 1 {
		t.Fatalf("un solo aviso por proveedor: %d", n)
	}
}

// Si la réplica 0 vuelve retoma el liderazgo y la 1 lo cede: nunca hay dos líderes ni se duplican arreglos.
func TestIntegrationsLeaderReturnsWithoutDuplicates(t *testing.T) {
	e := pollingEnv(t, nil)
	e.reg.MessageQueue.SetRedeliveryTimeout(300 * time.Millisecond) // un mensaje en vuelo al ceder se re-entrega pronto
	_, ana := e.signUp(t, "ana@example.com")
	if resp, body := e.do(t, "POST", "/correos", ana, linkBody("gmail", "mock-code-ana")); resp.StatusCode != 201 {
		t.Fatalf("vincular: %d %v", resp.StatusCode, body)
	}
	eventually(t, 5*time.Second, "los 4 arreglos", func() bool {
		d := e.dashboardJSON(t, ana)
		return len(d.Viajes) == 1 && len(d.Viajes[0].Arreglos) == 4
	})
	e.reg.StopReplica(0)
	eventually(t, 5*time.Second, "lidera la réplica 1", func() bool { l := e.leaders(); return len(l) == 1 && l[0] == 1 })
	e.reg.RecoverReplica(0)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if l := e.leaders(); len(l) > 1 {
			t.Fatalf("dos líderes a la vez: %v", l)
		} else if len(l) == 1 && l[0] == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if l := e.leaders(); len(l) != 1 || l[0] != 0 {
		t.Fatalf("la réplica 0 debía retomar el liderazgo: %v", l)
	}
	time.Sleep(4 * e.cfg.PollInterval)
	if d := e.dashboardJSON(t, ana); len(d.Viajes[0].Arreglos) != 4 {
		t.Fatalf("sin duplicados tras el regreso: %+v", d.Viajes[0].Arreglos)
	}
	eventually(t, 2*time.Second, "cola vacía", func() bool { return e.reg.MessageQueue.Pending() == 0 })
}

// Los mensajes publicados mientras no hay líder quedan en la cola y los procesa la líder que llega después.
func TestMessagesPublishedWithoutLeaderAreProcessedByNewLeader(t *testing.T) {
	e := newEnv(t) // sin sondeo: solo consumidor
	travelerID, ana := e.signUp(t, "ana@example.com")
	e.reg.StopReplica(0)
	e.reg.StopReplica(1)
	eventually(t, 5*time.Second, "sin líder", func() bool { return len(e.leaders()) == 0 })

	u, err := e.reg.Replicas[0].Integrations.Broker.Normalize(events.ProviderUpdateNormalized{
		IDViajero: travelerID, Proveedor: events.ProviderRef{Tipo: "AGENCIA", IDProveedor: "agencia-sol"},
		Arreglo: events.NormalizedArrangement{
			Tipo: "VUELO", Estado: "CONFIRMADO", FechaInicio: time.Date(2027, 5, 1, 10, 0, 0, 0, time.UTC),
			Detalle: map[string]any{"numeroVuelo": "XY9", "aerolinea": "Sol Air"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.reg.MessageQueue.Publish(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * e.cfg.HeartbeatInterval)
	if e.reg.MessageQueue.Pending() != 1 {
		t.Fatalf("la cola debe retener el mensaje sin líder: %d", e.reg.MessageQueue.Pending())
	}

	e.reg.RecoverReplica(1)
	eventually(t, 5*time.Second, "mensaje retenido procesado", func() bool { return e.reg.MessageQueue.Pending() == 0 })
	eventually(t, 5*time.Second, "arreglo visible", func() bool {
		// el LB tarda en reincorporar a la réplica recuperada: 503 mientras tanto
		resp, raw := e.raw(t, "GET", "/dashboard", ana, "application/json", nil)
		if resp.StatusCode != 200 {
			return false
		}
		var d dashboardJSON
		if json.Unmarshal([]byte(raw), &d) != nil {
			return false
		}
		return len(d.Viajes) >= 1 && len(d.Viajes[0].Arreglos) == 1 && d.Viajes[0].Arreglos[0].Origen == "AGENCIA"
	})
}
