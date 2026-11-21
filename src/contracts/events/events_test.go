package events

import (
	"encoding/json"
	"sort"
	"testing"
)

func keys(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func assertKeys(t *testing.T, name string, v any, want ...string) {
	t.Helper()
	sort.Strings(want)
	got := keys(t, v)
	if len(got) != len(want) {
		t.Fatalf("%s: claves %v, se esperaban %v", name, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s: claves %v, se esperaban %v", name, got, want)
		}
	}
}

func TestE1ProviderUpdateNormalized(t *testing.T) {
	assertKeys(t, "E1", ProviderUpdateNormalized{}, "eventId", "tipoEvento", "idViajero", "proveedor",
		"origenRemitente", "arreglo", "generadoEnFuente", "recibidoEn")
	assertKeys(t, "E1.proveedor", ProviderRef{}, "tipo", "idProveedor")
	assertKeys(t, "E1.arreglo", NormalizedArrangement{}, "tipo", "claveDeduplicacion", "estado", "fechaInicio", "fechaFin", "detalle")
}

func TestE2ToE8(t *testing.T) {
	assertKeys(t, "E2", ProviderDegraded{}, "idProveedor", "tipo", "idViajero", "estadoCircuito", "reintentos", "proximoIntento")
	assertKeys(t, "E3", Heartbeat{}, "nodoId", "tipoNodo", "rol", "secuencia", "emitidoEn")
	assertKeys(t, "E4", ResponsibilitySwap{}, "regionId", "nuevoPrimario", "nuevoPasivo", "motivo", "ocurridoEn")
	assertKeys(t, "E5", StateSync{}, "origen", "destino", "versionEstado", "emitidoEn")
	assertKeys(t, "E6", DataWritten{}, "idViajero", "entidad", "claves", "version")
	assertKeys(t, "E7", AccessRequestCreated{}, "idItinerario", "idDueno", "idSolicitante", "idSolicitud", "creadaEn")
	assertKeys(t, "E8", QueuedOperation{}, "operacionId", "tipo", "prioridad", "idViajero", "recurso", "payload", "encoladaEn")
}
