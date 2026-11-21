package region_test

import (
	"testing"
	"time"
)

// itineraryEnv prepara a la dueña con dos reservas y un itinerario compartido.
type itineraryEnv struct {
	*env
	ownerID, ownerToken string
	itineraryID         string
	arrangementIDs      []string
}

func newItineraryEnv(t *testing.T) *itineraryEnv {
	t.Helper()
	e := newEnv(t)
	ownerID, token := e.signUp(t, "ana@example.com")
	tripID := e.dashboardJSON(t, token).Viajes[0].IDViaje
	start := e.clock.Now().AddDate(0, 0, 10)
	var ids []string
	for _, title := range []string{"Vuelo a Lisboa", "Vuelo de regreso"} {
		resp, body := e.do(t, "POST", "/arreglos", token, arrangementBody(tripID, title, start, map[string]any{"aerolinea": "Sol Air"}))
		if resp.StatusCode != 201 {
			t.Fatalf("alta de arreglo: %d %v", resp.StatusCode, body)
		}
		ids = append(ids, body["idArreglo"].(string))
		start = start.Add(48 * time.Hour)
	}
	resp, body := e.do(t, "POST", "/itinerarios", token, map[string]any{"nombre": "Lisboa 2027", "idsArreglos": ids[:1]})
	if resp.StatusCode != 201 || body["idItinerario"] == "" || body["linkPublico"] == "" {
		t.Fatalf("crear itinerario: %d %v", resp.StatusCode, body)
	}
	return &itineraryEnv{env: e, ownerID: ownerID, ownerToken: token, itineraryID: body["idItinerario"].(string), arrangementIDs: ids}
}

func (e *itineraryEnv) path(suffix string) string { return "/itinerarios/" + e.itineraryID + suffix }

// Flujo AD 5.6 (QA09): acceso a un itinerario compartido solo con aprobación explícita del dueño.
func TestFlow56SharedItineraryAccessRequiresOwnerApproval(t *testing.T) {
	e := newItineraryEnv(t)
	betoID, beto := e.signUp(t, "beto@example.com")

	// El dueño ve su itinerario; sin token → 401; un invitado sin permiso → 403 con la opción de solicitar.
	resp, body := e.do(t, "GET", e.path(""), e.ownerToken, nil)
	if resp.StatusCode != 200 || body["nombre"] != "Lisboa 2027" || len(body["arreglos"].([]any)) != 1 {
		t.Fatalf("el dueño ve su itinerario: %d %v", resp.StatusCode, body)
	}
	if resp, body = e.do(t, "GET", e.path(""), "", nil); resp.StatusCode != 401 {
		t.Fatalf("sin token: %d %v", resp.StatusCode, body)
	}
	resp, body = e.do(t, "GET", e.path(""), beto, nil)
	if resp.StatusCode != 403 || body["codigo"] != "ACCESO_NO_AUTORIZADO" || body["puedeSolicitar"] != true {
		t.Fatalf("invitado sin permiso: %d %v", resp.StatusCode, body)
	}
	if resp, body = e.do(t, "GET", "/itinerarios/no-existe", beto, nil); resp.StatusCode != 404 {
		t.Fatalf("itinerario inexistente: %d %v", resp.StatusCode, body)
	}

	// El invitado solicita acceso → 201 PENDIENTE y se notifica al dueño (E7); sigue sin acceso.
	resp, body = e.do(t, "POST", e.path("/solicitudes"), beto, nil)
	if resp.StatusCode != 201 || body["estado"] != "PENDIENTE" || body["idSolicitud"] == "" {
		t.Fatalf("solicitar acceso: %d %v", resp.StatusCode, body)
	}
	requestID := body["idSolicitud"].(string)
	notes := e.reg.OwnerNotifier.NotificationsFor(e.ownerID)
	if len(notes) != 1 || notes[0].IDSolicitud != requestID || notes[0].IDSolicitante != betoID || notes[0].IDItinerario != e.itineraryID {
		t.Fatalf("notificación E7 al dueño: %+v", notes)
	}
	if resp, _ = e.do(t, "GET", e.path(""), beto, nil); resp.StatusCode != 403 {
		t.Fatalf("pendiente no ve el itinerario: %d", resp.StatusCode)
	}
	// Solicitar de nuevo no duplica ni vuelve a notificar.
	if resp, body = e.do(t, "POST", e.path("/solicitudes"), beto, nil); resp.StatusCode != 201 || body["idSolicitud"] != requestID {
		t.Fatalf("solicitud repetida: %d %v", resp.StatusCode, body)
	}
	if n := len(e.reg.OwnerNotifier.Notifications()); n != 1 {
		t.Fatalf("debía haber una sola notificación, hay %d", n)
	}

	// Un no-dueño no puede listar ni decidir (aunque conozca el id de la solicitud).
	if resp, _ = e.do(t, "GET", e.path("/solicitudes"), beto, nil); resp.StatusCode != 403 {
		t.Fatalf("no-dueño lista solicitudes: %d", resp.StatusCode)
	}
	decision := e.path("/solicitudes/" + requestID + "/decision")
	if resp, _ = e.do(t, "POST", decision, beto, map[string]string{"decision": "ACEPTAR"}); resp.StatusCode != 403 {
		t.Fatalf("no-dueño decide: %d", resp.StatusCode)
	}
	if resp, _ = e.do(t, "GET", e.path(""), beto, nil); resp.StatusCode != 403 {
		t.Fatalf("el intento de auto-aprobarse no debe dar acceso: %d", resp.StatusCode)
	}

	// El dueño lista las pendientes y acepta → ACEPTADA; el invitado ya ve el itinerario con datos críticos.
	resp, body = e.do(t, "GET", e.path("/solicitudes"), e.ownerToken, nil)
	pending, _ := body["solicitudes"].([]any)
	if resp.StatusCode != 200 || len(pending) != 1 || pending[0].(map[string]any)["idViajero"] != betoID {
		t.Fatalf("el dueño lista pendientes: %d %v", resp.StatusCode, body)
	}
	if resp, body = e.do(t, "POST", decision, e.ownerToken, map[string]string{"decision": "TAL_VEZ"}); resp.StatusCode != 422 {
		t.Fatalf("decisión inválida: %d %v", resp.StatusCode, body)
	}
	if resp, body = e.do(t, "POST", decision, e.ownerToken, map[string]string{"decision": "ACEPTAR"}); resp.StatusCode != 200 || body["estado"] != "ACEPTADA" {
		t.Fatalf("aceptar: %d %v", resp.StatusCode, body)
	}
	if resp, body = e.do(t, "POST", decision, e.ownerToken, map[string]string{"decision": "RECHAZAR"}); resp.StatusCode != 409 {
		t.Fatalf("decidir dos veces: %d %v", resp.StatusCode, body)
	}
	resp, body = e.do(t, "GET", e.path(""), beto, nil)
	if resp.StatusCode != 200 || body["idItinerario"] != e.itineraryID {
		t.Fatalf("autorizado ve el itinerario: %d %v", resp.StatusCode, body)
	}
	arr := body["arreglos"].([]any)[0].(map[string]any)
	if arr["titulo"] != "Vuelo a Lisboa" || arr["idArreglo"] != e.arrangementIDs[0] {
		t.Fatalf("arreglos del itinerario: %v", arr)
	}
	if _, leaked := arr["detalle"]; leaked {
		t.Fatalf("el itinerario compartido no debe exponer el detalle: %v", arr)
	}
	// Un autorizado ve, pero no gestiona.
	if resp, _ = e.do(t, "POST", e.path("/arreglos"), beto, map[string]string{"idArreglo": e.arrangementIDs[1]}); resp.StatusCode != 403 {
		t.Fatalf("el invitado no agrega arreglos: %d", resp.StatusCode)
	}
	if resp, _ = e.do(t, "DELETE", e.path(""), beto, nil); resp.StatusCode != 403 {
		t.Fatalf("el invitado no elimina el itinerario: %d", resp.StatusCode)
	}

	// El dueño desautoriza → 204 y el invitado vuelve a 403; revocar de nuevo → 404.
	if resp, _ = e.do(t, "DELETE", e.path("/accesos/"+betoID), e.ownerToken, nil); resp.StatusCode != 204 {
		t.Fatalf("desautorizar: %d", resp.StatusCode)
	}
	if resp, body = e.do(t, "GET", e.path(""), beto, nil); resp.StatusCode != 403 || body["puedeSolicitar"] != true {
		t.Fatalf("revocado: %d %v", resp.StatusCode, body)
	}
	if resp, _ = e.do(t, "DELETE", e.path("/accesos/"+betoID), e.ownerToken, nil); resp.StatusCode != 404 {
		t.Fatalf("revocar sin acceso vigente: %d", resp.StatusCode)
	}
}

func TestFlow56RejectionBlocksAccess(t *testing.T) {
	e := newItineraryEnv(t)
	_, caro := e.signUp(t, "caro@example.com")

	_, body := e.do(t, "POST", e.path("/solicitudes"), caro, nil)
	requestID := body["idSolicitud"].(string)
	resp, body := e.do(t, "POST", e.path("/solicitudes/"+requestID+"/decision"), e.ownerToken, map[string]string{"decision": "RECHAZAR"})
	if resp.StatusCode != 200 || body["estado"] != "RECHAZADA" {
		t.Fatalf("rechazar: %d %v", resp.StatusCode, body)
	}
	resp, body = e.do(t, "GET", e.path(""), caro, nil)
	if resp.StatusCode != 403 || body["codigo"] != "ACCESO_NO_AUTORIZADO" || body["puedeSolicitar"] != false {
		t.Fatalf("rechazada sigue sin acceso y sin opción de solicitar: %d %v", resp.StatusCode, body)
	}
	if resp, _ = e.do(t, "POST", e.path("/solicitudes"), caro, nil); resp.StatusCode != 403 {
		t.Fatalf("el rechazo bloquea nuevas solicitudes: %d", resp.StatusCode)
	}
	// La solicitud ya decidida deja de aparecer entre las pendientes.
	if _, body = e.do(t, "GET", e.path("/solicitudes"), e.ownerToken, nil); len(body["solicitudes"].([]any)) != 0 {
		t.Fatalf("no debían quedar pendientes: %v", body)
	}
	// El dueño no puede solicitarse acceso a sí mismo.
	if resp, _ = e.do(t, "POST", e.path("/solicitudes"), e.ownerToken, nil); resp.StatusCode != 409 {
		t.Fatalf("el dueño ya tiene acceso: %d", resp.StatusCode)
	}
}

func TestItineraryArrangementsAndDeletion(t *testing.T) {
	e := newItineraryEnv(t)
	betoID, beto := e.signUp(t, "beto@example.com")
	_, betoArr := e.do(t, "POST", "/arreglos", beto, arrangementBody(e.dashboardJSON(t, beto).Viajes[0].IDViaje, "Reserva de Beto", e.clock.Now().AddDate(0, 0, 5), nil))
	betoArrangementID := betoArr["idArreglo"].(string)

	// Agregar una reserva propia y no una ajena ni inexistente.
	if resp, _ := e.do(t, "POST", e.path("/arreglos"), e.ownerToken, map[string]string{"idArreglo": e.arrangementIDs[1]}); resp.StatusCode != 204 {
		t.Fatalf("agregar: %d", resp.StatusCode)
	}
	for _, id := range []string{betoArrangementID, "no-existe"} {
		if resp, _ := e.do(t, "POST", e.path("/arreglos"), e.ownerToken, map[string]string{"idArreglo": id}); resp.StatusCode != 404 {
			t.Fatalf("agregar %q: %d", id, resp.StatusCode)
		}
	}
	_, body := e.do(t, "GET", e.path(""), e.ownerToken, nil)
	if len(body["arreglos"].([]any)) != 2 {
		t.Fatalf("debían ser 2 reservas: %v", body)
	}
	// Quitar una reserva.
	if resp, _ := e.do(t, "DELETE", e.path("/arreglos/"+e.arrangementIDs[0]), e.ownerToken, nil); resp.StatusCode != 204 {
		t.Fatalf("quitar: %d", resp.StatusCode)
	}
	if resp, _ := e.do(t, "DELETE", e.path("/arreglos/"+e.arrangementIDs[0]), e.ownerToken, nil); resp.StatusCode != 404 {
		t.Fatalf("quitar dos veces: %d", resp.StatusCode)
	}
	_, body = e.do(t, "GET", e.path(""), e.ownerToken, nil)
	if got := body["arreglos"].([]any); len(got) != 1 || got[0].(map[string]any)["idArreglo"] != e.arrangementIDs[1] {
		t.Fatalf("reservas tras quitar: %v", body)
	}
	// Una reserva eliminada del dashboard desaparece del itinerario sin romperlo.
	if resp, _ := e.do(t, "DELETE", "/arreglos/"+e.arrangementIDs[1], e.ownerToken, nil); resp.StatusCode != 204 {
		t.Fatalf("eliminar reserva: %d", resp.StatusCode)
	}
	if resp, body := e.do(t, "GET", e.path(""), e.ownerToken, nil); resp.StatusCode != 200 || len(body["arreglos"].([]any)) != 0 {
		t.Fatalf("itinerario con referencia colgante: %d %v", resp.StatusCode, body)
	}

	// Validaciones de creación.
	if resp, _ := e.do(t, "POST", "/itinerarios", e.ownerToken, map[string]any{"nombre": ""}); resp.StatusCode != 422 {
		t.Fatalf("nombre vacío: %d", resp.StatusCode)
	}
	if resp, _ := e.do(t, "POST", "/itinerarios", e.ownerToken, map[string]any{"nombre": "X", "idsArreglos": []string{betoArrangementID}}); resp.StatusCode != 422 {
		t.Fatalf("arreglo ajeno: %d", resp.StatusCode)
	}

	// Eliminar: el dueño sí, y el itinerario deja de existir también para los autorizados.
	_, body = e.do(t, "POST", e.path("/solicitudes"), beto, nil)
	e.do(t, "POST", e.path("/solicitudes/"+body["idSolicitud"].(string)+"/decision"), e.ownerToken, map[string]string{"decision": "ACEPTAR"})
	if resp, _ := e.do(t, "GET", e.path(""), beto, nil); resp.StatusCode != 200 {
		t.Fatalf("beto autorizado: %d", resp.StatusCode)
	}
	if resp, _ := e.do(t, "DELETE", e.path(""), e.ownerToken, nil); resp.StatusCode != 204 {
		t.Fatalf("eliminar: %d", resp.StatusCode)
	}
	if resp, _ := e.do(t, "GET", e.path(""), beto, nil); resp.StatusCode != 404 {
		t.Fatalf("tras eliminar: %d", resp.StatusCode)
	}
	if resp, _ := e.do(t, "DELETE", e.path("/accesos/"+betoID), e.ownerToken, nil); resp.StatusCode != 404 {
		t.Fatalf("revocar en itinerario eliminado: %d", resp.StatusCode)
	}
}
