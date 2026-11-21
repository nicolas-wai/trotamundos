package region_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
)

// raw hace una request por el balanceador con el Accept indicado y devuelve el cuerpo sin decodificar.
func (e *env) raw(t *testing.T, method, path, token, accept string, body any) (*http.Response, string) {
	t.Helper()
	var rd io.Reader = http.NoBody
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.lb.URL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp, string(out)
}

// signUp registra y autentica un viajero; devuelve su idViajero y access token.
func (e *env) signUp(t *testing.T, email string) (travelerID, token string) {
	t.Helper()
	resp, body := e.do(t, "POST", "/registro", "", creds(email, "clave-segura-1"))
	if resp.StatusCode != 201 {
		t.Fatalf("registro %s: %d %v", email, resp.StatusCode, body)
	}
	travelerID = body["idViajero"].(string)
	resp, body = e.do(t, "POST", "/login", "", creds(email, "clave-segura-1"))
	if resp.StatusCode != 200 {
		t.Fatalf("login %s: %d %v", email, resp.StatusCode, body)
	}
	return travelerID, body["accessToken"].(string)
}

type dashboardJSON struct {
	Viajes []struct {
		IDViaje  string `json:"idViaje"`
		Nombre   string `json:"nombre"`
		Arreglos []struct {
			IDArreglo string `json:"idArreglo"`
			Tipo      string `json:"tipo"`
			Titulo    string `json:"titulo"`
			Estado    string `json:"estado"`
			Origen    string `json:"origen"`
		} `json:"arreglos"`
	} `json:"viajes"`
	AvisosProveedores []any `json:"avisosProveedores"`
}

func (e *env) dashboardJSON(t *testing.T, token string) dashboardJSON {
	t.Helper()
	resp, body := e.raw(t, "GET", "/dashboard", token, "application/json", nil)
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("GET /dashboard JSON: %d %s %s", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	var d dashboardJSON
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	return d
}

func arrangementBody(tripID, title string, start time.Time, detail map[string]any) map[string]any {
	return map[string]any{
		"idViaje": tripID, "tipo": "VUELO", "titulo": title,
		"fechaInicio": start.UTC().Format(time.RFC3339), "fechaFin": start.Add(3 * time.Hour).UTC().Format(time.RFC3339),
		"detalle": detail,
	}
}

// Flujos AD 5.1 (1ª mitad) y 5.2: dashboard SSR y CRUD manual atravesando LB → réplica → cola → nodo de datos.
func TestFlow52ManualCrudThroughTheLoadBalancer(t *testing.T) {
	e := newEnv(t)
	travelerID, token := e.signUp(t, "ana@example.com")
	start := e.clock.Now().AddDate(0, 0, 10)

	// Sin token → 401; el viaje demo se provisionó al registrarse y el dashboard no trae avisos.
	if resp, body := e.do(t, "GET", "/dashboard", "", nil); resp.StatusCode != 401 || body["codigo"] != "TOKEN_INVALIDO" {
		t.Fatalf("sin token: %d %v", resp.StatusCode, body)
	}
	d := e.dashboardJSON(t, token)
	if len(d.Viajes) != 1 || d.Viajes[0].IDViaje != "demo-"+travelerID || len(d.Viajes[0].Arreglos) != 0 {
		t.Fatalf("dashboard inicial: %+v", d)
	}
	if d.AvisosProveedores == nil || len(d.AvisosProveedores) != 0 {
		t.Fatalf("avisosProveedores debe ser [] en esta fase: %#v", d.AvisosProveedores)
	}
	tripID := d.Viajes[0].IDViaje

	// Crear → 201 y aparece en el dashboard (JSON y HTML) con origen MANUAL, sin datos de detalle en el HTML.
	resp, body := e.do(t, "POST", "/arreglos", token, arrangementBody(tripID, "Vuelo a Lima", start, map[string]any{"aerolinea": "LATAM-X", "numeroVuelo": "LA800"}))
	if resp.StatusCode != 201 || body["idArreglo"] == "" {
		t.Fatalf("crear: %d %v", resp.StatusCode, body)
	}
	arrID := body["idArreglo"].(string)
	d = e.dashboardJSON(t, token)
	if len(d.Viajes[0].Arreglos) != 1 || d.Viajes[0].Arreglos[0].IDArreglo != arrID || d.Viajes[0].Arreglos[0].Origen != "MANUAL" ||
		d.Viajes[0].Arreglos[0].Estado != "CONFIRMADO" || d.Viajes[0].Arreglos[0].Tipo != "VUELO" {
		t.Fatalf("el arreglo debe verse en el dashboard JSON: %+v", d.Viajes[0].Arreglos)
	}
	resp, html := e.raw(t, "GET", "/dashboard", token, "text/html", nil)
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") ||
		!strings.Contains(html, "Vuelo a Lima") || !strings.Contains(html, `data-id="`+arrID+`"`) {
		t.Fatalf("dashboard HTML: %d %s", resp.StatusCode, html)
	}
	if strings.Contains(html, "LATAM-X") || strings.Contains(html, "LA800") {
		t.Fatalf("payload reducido: el dashboard HTML no debe traer el detalle: %s", html)
	}

	// Editar → 200 y se refleja; el detalle (JSON y fragmento HTML) trae los datos completos bajo demanda.
	resp, body = e.do(t, "PUT", "/arreglos/"+arrID, token, arrangementBody(tripID, "Vuelo a Cusco", start, map[string]any{"aerolinea": "SKY", "numeroVuelo": "H2-1"}))
	if resp.StatusCode != 200 {
		t.Fatalf("editar: %d %v", resp.StatusCode, body)
	}
	if d = e.dashboardJSON(t, token); d.Viajes[0].Arreglos[0].Titulo != "Vuelo a Cusco" {
		t.Fatalf("la edición debe verse: %+v", d.Viajes[0].Arreglos)
	}
	resp, body = e.do(t, "GET", "/arreglos/"+arrID, token, nil)
	detail, _ := body["detalle"].(map[string]any)
	if resp.StatusCode != 200 || body["idArreglo"] != arrID || body["idViaje"] != tripID || body["origen"] != "MANUAL" ||
		body["remitente"] != nil || detail["aerolinea"] != "SKY" || detail["numeroVuelo"] != "H2-1" || body["actualizadoEn"] == "" {
		t.Fatalf("detalle JSON: %d %v", resp.StatusCode, body)
	}
	resp, html = e.raw(t, "GET", "/arreglos/"+arrID, token, "text/html", nil)
	if resp.StatusCode != 200 || !strings.Contains(html, "SKY") || !strings.Contains(html, "H2-1") {
		t.Fatalf("fragmento HTML de detalle: %d %s", resp.StatusCode, html)
	}

	// Validaciones → 422 con el formato de error de §6.1.
	for name, bad := range map[string]map[string]any{
		"tipo desconocido":  {"idViaje": tripID, "tipo": "CRUCERO", "titulo": "x", "fechaInicio": start.UTC().Format(time.RFC3339)},
		"sin título":        {"idViaje": tripID, "tipo": "BUS", "titulo": "", "fechaInicio": start.UTC().Format(time.RFC3339)},
		"viaje inexistente": {"idViaje": "no-existe", "tipo": "BUS", "titulo": "x", "fechaInicio": start.UTC().Format(time.RFC3339)},
	} {
		if resp, body = e.do(t, "POST", "/arreglos", token, bad); resp.StatusCode != 422 || body["codigo"] != "VALIDACION_FALLIDA" {
			t.Fatalf("%s: %d %v", name, resp.StatusCode, body)
		}
	}

	// Otro viajero no ve ni alcanza el arreglo (404, no se revela su existencia).
	_, otherToken := e.signUp(t, "beto@example.com")
	if resp, body = e.do(t, "GET", "/arreglos/"+arrID, otherToken, nil); resp.StatusCode != 404 || body["codigo"] != "NO_ENCONTRADO" {
		t.Fatalf("detalle ajeno: %d %v", resp.StatusCode, body)
	}
	if resp, _ = e.do(t, "PUT", "/arreglos/"+arrID, otherToken, arrangementBody(tripID, "Robado", start, nil)); resp.StatusCode != 404 {
		t.Fatalf("edición ajena: %d", resp.StatusCode)
	}
	if resp, _ = e.do(t, "DELETE", "/arreglos/"+arrID, otherToken, nil); resp.StatusCode != 404 {
		t.Fatalf("baja ajena: %d", resp.StatusCode)
	}
	if od := e.dashboardJSON(t, otherToken); len(od.Viajes) != 1 || len(od.Viajes[0].Arreglos) != 0 {
		t.Fatalf("el otro viajero no debe ver arreglos ajenos: %+v", od)
	}
	if resp, body = e.do(t, "POST", "/arreglos", otherToken, arrangementBody(tripID, "En viaje ajeno", start, nil)); resp.StatusCode != 422 {
		t.Fatalf("alta en un viaje ajeno: %d %v", resp.StatusCode, body)
	}
	if d = e.dashboardJSON(t, token); d.Viajes[0].Arreglos[0].Titulo != "Vuelo a Cusco" {
		t.Fatalf("los intentos ajenos no deben alterar los datos: %+v", d.Viajes[0].Arreglos)
	}

	// Eliminar → 204; después el detalle y una segunda baja dan 404, y el dashboard queda sin el arreglo.
	if resp, _ = e.do(t, "DELETE", "/arreglos/"+arrID, token, nil); resp.StatusCode != 204 {
		t.Fatalf("eliminar: %d", resp.StatusCode)
	}
	if resp, body = e.do(t, "GET", "/arreglos/"+arrID, token, nil); resp.StatusCode != 404 || body["codigo"] != "NO_ENCONTRADO" {
		t.Fatalf("detalle tras eliminar: %d %v", resp.StatusCode, body)
	}
	if resp, _ = e.do(t, "DELETE", "/arreglos/"+arrID, token, nil); resp.StatusCode != 404 {
		t.Fatalf("segunda baja: %d", resp.StatusCode)
	}
	if d = e.dashboardJSON(t, token); len(d.Viajes[0].Arreglos) != 0 {
		t.Fatalf("el arreglo eliminado no debe verse: %+v", d.Viajes[0].Arreglos)
	}
}

// [ADR-09] Las escrituras del CRUD entran a la cola como ESCRITURA (prioridad baja) y la lectura del dashboard
// como LECTURA (prioridad alta); el idViajero de cada operación sale del token, no del body. [ADR-11]
func TestCrudWritesAndDashboardReadsAreQueuedWithTheirKind(t *testing.T) {
	e := newEnv(t)
	travelerID, token := e.signUp(t, "ana@example.com")
	tripID := "demo-" + travelerID
	start := e.clock.Now().AddDate(0, 0, 10)

	resources := func(ops []events.QueuedOperation) (kinds map[string]string, owners map[string]bool) {
		kinds, owners = map[string]string{}, map[string]bool{}
		for _, op := range ops {
			kinds[op.Recurso] = op.Tipo
			owners[op.IDViajero] = true
		}
		return
	}

	mark := e.spy.mark()
	e.dashboardJSON(t, token)
	ops := e.spy.since(mark)
	kinds, owners := resources(ops)
	if len(ops) == 0 || len(kinds) != 2 || kinds["trip.list"] != events.OperationRead || kinds["arrangement.list_summaries"] != events.OperationRead {
		t.Fatalf("el dashboard debe leer viajes y arreglos como LECTURA: %+v", ops)
	}
	for _, op := range ops {
		if op.Prioridad != 1 {
			t.Fatalf("las lecturas llevan prioridad alta (1): %+v", op)
		}
	}
	if len(owners) != 1 || !owners[travelerID] {
		t.Fatalf("idViajero debe venir del token: %v", owners)
	}

	mark = e.spy.mark()
	_, body := e.do(t, "POST", "/arreglos", token, arrangementBody(tripID, "Vuelo", start, nil))
	arrID := body["idArreglo"].(string)
	e.do(t, "PUT", "/arreglos/"+arrID, token, arrangementBody(tripID, "Vuelo editado", start, nil))
	e.do(t, "DELETE", "/arreglos/"+arrID, token, nil)
	var writes []string
	for _, op := range e.spy.since(mark) {
		if op.Tipo == events.OperationWrite {
			if op.Prioridad != 2 {
				t.Fatalf("las escrituras llevan prioridad baja (2): %+v", op)
			}
			writes = append(writes, op.Recurso)
		}
	}
	sort.Strings(writes)
	if strings.Join(writes, ",") != "arrangement.delete,arrangement.save,arrangement.save" {
		t.Fatalf("alta, edición y baja deben encolarse como ESCRITURA: %v", writes)
	}
}

// El shell web y sus estáticos se sirven sin autenticación a través del balanceador. [ADR-20]
func TestWebShellIsServedWithoutAuthentication(t *testing.T) {
	e := newEnv(t)
	resp, html := e.raw(t, "GET", "/", "", "text/html", nil)
	if resp.StatusCode != 200 || !strings.Contains(html, `id="login-form"`) || !strings.Contains(html, "visual_preferences.js") {
		t.Fatalf("shell: %d %.200s", resp.StatusCode, html)
	}
	for _, asset := range []string{"runtime.js", "optimistic_state.js", "visual_preferences.js", "styles.css"} {
		if resp, body := e.raw(t, "GET", "/static/"+asset, "", "", nil); resp.StatusCode != 200 || len(body) < 100 {
			t.Fatalf("/static/%s: %d (%d bytes)", asset, resp.StatusCode, len(body))
		}
	}
	if resp, _ := e.raw(t, "GET", "/no-existe", "", "", nil); resp.StatusCode != 404 {
		t.Fatalf("ruta desconocida: %d", resp.StatusCode)
	}
}

// Smoke QA02: 200 lecturas del dashboard por el balanceador con p95 < 800 ms (medida local, no productiva).
func TestQA02DashboardReadLatencySmoke(t *testing.T) {
	e := newEnv(t)
	travelerID, token := e.signUp(t, "ana@example.com")
	tripID := "demo-" + travelerID
	start := e.clock.Now().AddDate(0, 0, 10)
	for i := 0; i < 5; i++ {
		if resp, body := e.do(t, "POST", "/arreglos", token, arrangementBody(tripID, "Reserva", start.Add(time.Duration(i)*time.Hour), nil)); resp.StatusCode != 201 {
			t.Fatalf("alta %d: %d %v", i, resp.StatusCode, body)
		}
	}
	const reads = 200
	durations := make([]time.Duration, 0, reads)
	for i := 0; i < reads; i++ {
		begin := time.Now()
		resp, _ := e.raw(t, "GET", "/dashboard", token, "text/html", nil)
		durations = append(durations, time.Since(begin))
		if resp.StatusCode != 200 {
			t.Fatalf("lectura %d: %d", i, resp.StatusCode)
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[reads*95/100-1]
	t.Logf("QA02 p95=%v max=%v (%d lecturas)", p95, durations[reads-1], reads)
	if p95 >= 800*time.Millisecond {
		t.Fatalf("p95 = %v, el objetivo es < 800 ms", p95)
	}
}
