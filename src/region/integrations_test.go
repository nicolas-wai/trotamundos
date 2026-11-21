package region_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/config"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/stubs"
)

// pollingEnv arma una región con el sondeo activo a intervalos acelerados y reintentos rápidos.
func pollingEnv(t *testing.T, tune func(*config.RegionConfig)) *env {
	t.Helper()
	return newEnvWith(t, func(c *config.RegionConfig) {
		c.PollingEnabled = true
		c.PollInterval = 200 * time.Millisecond
		c.ProviderMaxRetries = 2
		c.ProviderBackoff = time.Millisecond
		c.ProviderCooldown = 300 * time.Millisecond
		c.ProviderPollTimeout = time.Second
		if tune != nil {
			tune(c)
		}
	})
}

func linkBody(provider, code string) map[string]any {
	return map[string]any{"proveedor": provider, "authorizationCode": code, "redirectUri": "http://localhost:8080/oauth/callback"}
}

// eventually repite cond hasta que se cumple o vence el plazo.
func eventually(t *testing.T, d time.Duration, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout esperando: %s", msg)
}

// dbBytes lee todos los archivos de datos (.db y sus journals) de la región.
func dbBytes(t *testing.T, dir string) []byte {
	t.Helper()
	var all []byte
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.Contains(filepath.Base(path), ".db") {
			b, _ := os.ReadFile(path)
			all = append(all, b...)
		}
		return nil
	})
	return all
}

// Flujo AD 5.4: vinculación de correo con OAuth 2.0 simulado, tokens cifrados en reposo y revocación.
func TestFlow54LinkEmailWithOAuthAndRevoke(t *testing.T) {
	e := newEnv(t)
	_, token := e.signUp(t, "ana@example.com")
	_, otherToken := e.signUp(t, "beto@example.com")

	// Sin token → 401.
	if resp, body := e.do(t, "POST", "/correos", "", linkBody("gmail", "mock-code-ana")); resp.StatusCode != 401 || body["codigo"] != "TOKEN_INVALIDO" {
		t.Fatalf("sin token: %d %v", resp.StatusCode, body)
	}
	// Código inválido → 4xx; campos faltantes → 422.
	if resp, body := e.do(t, "POST", "/correos", token, linkBody("gmail", "codigo-falso")); resp.StatusCode != 400 || body["codigo"] != "CODIGO_OAUTH_INVALIDO" {
		t.Fatalf("código inválido: %d %v", resp.StatusCode, body)
	}
	if resp, body := e.do(t, "POST", "/correos", token, map[string]any{"proveedor": "gmail"}); resp.StatusCode != 422 || body["codigo"] != "VALIDACION_FALLIDA" {
		t.Fatalf("faltan campos: %d %v", resp.StatusCode, body)
	}

	// Código válido → 201 { idCuenta, proveedor, estado: "VINCULADA" }.
	resp, body := e.do(t, "POST", "/correos", token, linkBody("gmail", "mock-code-ana"))
	if resp.StatusCode != 201 || body["proveedor"] != "gmail" || body["estado"] != "VINCULADA" || body["idCuenta"] == "" {
		t.Fatalf("vincular: %d %v", resp.StatusCode, body)
	}
	accountID := body["idCuenta"].(string)
	// Un código OAuth se canjea una sola vez.
	if resp, body := e.do(t, "POST", "/correos", token, linkBody("gmail", "mock-code-ana")); resp.StatusCode != 400 {
		t.Fatalf("código reutilizado: %d %v", resp.StatusCode, body)
	}

	// Tokens cifrados en reposo: el valor guardado en SQLite no es el texto plano (QA-08, ADR-15).
	files := dbBytes(t, e.cfg.DataDir)
	if !bytes.Contains(files, []byte(accountID)) {
		t.Fatal("la cuenta vinculada debería estar en los archivos SQLite")
	}
	for _, plain := range []string{"mock-access-gmail-ana", "mock-refresh-gmail-ana", "mock-access", "mock-refresh"} {
		if bytes.Contains(files, []byte(plain)) {
			t.Fatalf("el token OAuth %q está en texto plano en SQLite", plain)
		}
	}

	// Otro viajero no puede desvincular una cuenta ajena (404, no se revela su existencia).
	if resp, body := e.do(t, "DELETE", "/correos/"+accountID, otherToken, nil); resp.StatusCode != 404 {
		t.Fatalf("cuenta ajena: %d %v", resp.StatusCode, body)
	}
	if len(e.reg.ExternalAPIs.OAuth.RevokedAccounts()) != 0 {
		t.Fatal("no debe revocarse nada ante un intento ajeno")
	}

	// Desvincular → 204, se llamó a revocar en el proveedor y se eliminó el vínculo.
	if resp, _ := e.do(t, "DELETE", "/correos/"+accountID, token, nil); resp.StatusCode != 204 {
		t.Fatalf("desvincular: %d", resp.StatusCode)
	}
	if got := e.reg.ExternalAPIs.OAuth.RevokedAccounts(); len(got) != 1 || got[0] != accountID {
		t.Fatalf("revocaciones en el mock: %v", got)
	}
	if resp, body := e.do(t, "DELETE", "/correos/"+accountID, token, nil); resp.StatusCode != 404 {
		t.Fatalf("ya eliminada: %d %v", resp.StatusCode, body)
	}
}

// Flujo AD 5.3: vincular correo → sondeo (correo, agencia y sistema de viajes) → broker → cola P2P → consumidor →
// dashboard. La reserva presente en dos fuentes aparece una sola vez y la latencia cumple QA-01 escalada.
func TestFlow53PollingIngestsAllProvidersWithoutDuplicates(t *testing.T) {
	e := pollingEnv(t, nil)
	apis := e.reg.ExternalAPIs
	// Los datos "se generan en la fuente" ahora: de aquí se mide la latencia hasta que son visibles (QA-01).
	generated := time.Now()
	for _, s := range []interface{ SetGeneratedAt(time.Time) }{apis.Email, apis.Agency, apis.TravelSystem} {
		s.SetGeneratedAt(generated)
	}
	_, token := e.signUp(t, "ana@example.com")
	if resp, body := e.do(t, "POST", "/correos", token, linkBody("gmail", "mock-code-ana")); resp.StatusCode != 201 {
		t.Fatalf("vincular: %d %v", resp.StatusCode, body)
	}

	var d dashboardJSON
	eventually(t, 5*time.Second, "las 4 reservas distintas en el dashboard", func() bool {
		d = e.dashboardJSON(t, token)
		return len(d.Viajes) == 1 && len(d.Viajes[0].Arreglos) == 4
	})
	visibleAfter := time.Since(generated)
	// Umbral escalado: QA-01 pide 5 min con sondeo cada 2 min; con sondeo cada 200 ms → 5 min × (200 ms / 2 min).
	threshold := time.Duration(float64(5*time.Minute) * float64(e.cfg.PollInterval) / float64(config.MaxPollInterval))
	if visibleAfter > threshold {
		t.Fatalf("QA-01: visible a los %s, umbral escalado %s", visibleAfter, threshold)
	}
	t.Logf("QA-01: generadoEnFuente → visible en %s (umbral escalado %s)", visibleAfter, threshold)

	origins := map[string]int{}
	flights := 0
	ids := map[string]string{} // tipo → idArreglo
	for _, a := range d.Viajes[0].Arreglos {
		origins[a.Origen]++
		ids[a.Tipo] = a.IDArreglo
		if a.Tipo == "VUELO" {
			flights++
		}
	}
	if flights != 1 {
		t.Fatalf("la reserva presente en correo y sistema de viajes debe aparecer una sola vez: %+v", d.Viajes[0].Arreglos)
	}
	if origins["EMAIL"] == 0 || origins["AGENCIA"] == 0 || origins["VIAJES"] == 0 || origins["MANUAL"] != 0 {
		t.Fatalf("orígenes esperados EMAIL/AGENCIA/VIAJES: %v", origins)
	}
	for _, a := range d.Viajes[0].Arreglos {
		if a.Estado != "CONFIRMADO" {
			t.Fatalf("estado inesperado: %+v", a)
		}
	}

	// remitente (US03): el auto llegó solo por correo; el vuelo fusionó el remitente del correo aunque
	// el sistema de viajes lo haya registrado primero.
	remitente := func(id string) any {
		resp, body := e.do(t, "GET", "/arreglos/"+id, token, nil)
		if resp.StatusCode != 200 {
			t.Fatalf("detalle %s: %d %v", id, resp.StatusCode, body)
		}
		return body["remitente"]
	}
	if got := remitente(ids["AUTO"]); got != "alquileres@rutacar.example" {
		t.Fatalf("remitente del auto: %v", got)
	}
	if got := remitente(ids["VUELO"]); got != "reservas@solair.example" {
		t.Fatalf("remitente del vuelo fusionado: %v", got)
	}
	if got := remitente(ids["ALOJAMIENTO"]); got != nil {
		t.Fatalf("la agencia no trae remitente: %v", got)
	}
	// El detalle del vuelo fusionó campos de ambas fuentes.
	_, flight := e.do(t, "GET", "/arreglos/"+ids["VUELO"], token, nil)
	det, _ := flight["detalle"].(map[string]any)
	if det["numeroVuelo"] != "SA123" || det["origenIATA"] != "EZE" || det["destinoIATA"] != "MAD" || det["aerolinea"] == "" {
		t.Fatalf("detalle fusionado: %v", det)
	}

	// Sigue sondeando pero no duplica (sondeos idempotentes y cola vacía).
	time.Sleep(3 * e.cfg.PollInterval)
	if d = e.dashboardJSON(t, token); len(d.Viajes[0].Arreglos) != 4 {
		t.Fatalf("sin duplicados tras más sondeos: %+v", d.Viajes[0].Arreglos)
	}
	eventually(t, time.Second, "cola P2P vacía (todo confirmado)", func() bool { return e.reg.MessageQueue.Pending() == 0 })
	if apis.Email.Calls() == 0 || apis.Agency.Calls() == 0 || apis.TravelSystem.Calls() == 0 {
		t.Fatalf("los tres proveedores deben sondearse: %d %d %d", apis.Email.Calls(), apis.Agency.Calls(), apis.TravelSystem.Calls())
	}
}

// Un viajero no ve las reservas de otro, y las cuentas desvinculadas dejan de sondearse.
func TestPollingIsolatesTravelersAndStopsAfterUnlink(t *testing.T) {
	e := pollingEnv(t, nil)
	_, ana := e.signUp(t, "ana@example.com")
	_, beto := e.signUp(t, "beto@example.com")
	_, body := e.do(t, "POST", "/correos", ana, linkBody("gmail", "mock-code-ana"))
	accountID := body["idCuenta"].(string)
	eventually(t, 5*time.Second, "las reservas de Ana", func() bool {
		d := e.dashboardJSON(t, ana)
		return len(d.Viajes) == 1 && len(d.Viajes[0].Arreglos) == 4
	})
	eventually(t, 5*time.Second, "las reservas de Beto", func() bool {
		d := e.dashboardJSON(t, beto)
		return len(d.Viajes) == 1 && len(d.Viajes[0].Arreglos) >= 2
	})
	// Beto no vinculó correo: solo recibe lo de agencia y sistema de viajes (3 reservas), nunca el auto del correo de Ana.
	for _, a := range e.dashboardJSON(t, beto).Viajes[0].Arreglos {
		if a.Tipo == "AUTO" || a.Origen == "EMAIL" {
			t.Fatalf("Beto no debe ver reservas del correo de Ana: %+v", a)
		}
	}
	if resp, _ := e.do(t, "DELETE", "/correos/"+accountID, ana, nil); resp.StatusCode != 204 {
		t.Fatalf("desvincular: %d", resp.StatusCode)
	}
	time.Sleep(e.cfg.PollInterval) // deja terminar un sondeo en curso
	calls := e.reg.ExternalAPIs.Email.Calls()
	time.Sleep(3 * e.cfg.PollInterval)
	if got := e.reg.ExternalAPIs.Email.Calls(); got != calls {
		t.Fatalf("la cuenta desvinculada no debe sondearse: %d → %d llamadas", calls, got)
	}
}

// Degradación (ADR-07): una agencia caída abre su circuito, el dashboard muestra el aviso y el resto sigue en 200;
// al recuperarse el circuito cierra y el aviso desaparece.
func TestProviderFailureOpensCircuitShowsNoticeAndRecovers(t *testing.T) {
	e := pollingEnv(t, nil)
	apis := e.reg.ExternalAPIs
	apis.Agency.SetFailureMode(stubs.FailUnavailable)
	_, token := e.signUp(t, "ana@example.com")

	var d dashboardJSON
	eventually(t, 5*time.Second, "aviso de proveedor degradado", func() bool {
		resp, body := e.raw(t, "GET", "/dashboard", token, "application/json", nil)
		if resp.StatusCode != 200 { // el dashboard nunca falla por un proveedor caído
			t.Fatalf("el dashboard debe seguir en 200: %d %s", resp.StatusCode, body)
		}
		d = e.dashboardJSON(t, token)
		return len(d.AvisosProveedores) == 1
	})
	notice := d.AvisosProveedores[0].(map[string]any)
	if notice["proveedor"] != "Agencia Sol" || notice["mensaje"] != "No se pudo conectar con el proveedor" {
		t.Fatalf("aviso: %v", notice)
	}
	// El HTML SSR también muestra el aviso.
	if _, html := e.raw(t, "GET", "/dashboard", token, "text/html", nil); !strings.Contains(html, "Agencia Sol") {
		t.Fatalf("el HTML debe mostrar el aviso: %s", html)
	}
	// Los demás proveedores siguen entregando sus reservas (el sistema de viajes no está degradado).
	eventually(t, 5*time.Second, "reservas del sistema de viajes pese a la agencia caída", func() bool {
		d = e.dashboardJSON(t, token)
		return len(d.Viajes) == 1 && len(d.Viajes[0].Arreglos) == 2
	})
	for _, a := range d.Viajes[0].Arreglos {
		if a.Origen == "AGENCIA" {
			t.Fatalf("la agencia caída no debe aportar reservas: %+v", a)
		}
	}

	// Recuperación: pasado el cooldown el circuito prueba, cierra y retira el aviso; llega el hotel de la agencia.
	apis.Agency.SetFailureMode(stubs.FailNone)
	eventually(t, 5*time.Second, "aviso retirado y hotel recibido", func() bool {
		d = e.dashboardJSON(t, token)
		return len(d.AvisosProveedores) == 0 && len(d.Viajes[0].Arreglos) == 3
	})
}

// Un proveedor que responde basura (2xx no parseable) también cuenta como falla del proveedor (QA-14).
func TestUnparseableProviderResponseDegradesProvider(t *testing.T) {
	e := pollingEnv(t, nil)
	e.reg.ExternalAPIs.TravelSystem.SetFailureMode(stubs.FailMalformed)
	_, token := e.signUp(t, "ana@example.com")
	eventually(t, 5*time.Second, "aviso del sistema de viajes", func() bool {
		d := e.dashboardJSON(t, token)
		return len(d.AvisosProveedores) == 1 && d.AvisosProveedores[0].(map[string]any)["proveedor"] == "Sistema Viajes"
	})
}

// Consumo asíncrono por la cola P2P (flujo 5.3, mitad inferior): un E1 publicado directamente llega a
// Arreglos de viaje por el consumidor; uno inválido se confirma y descarta sin bloquear la cola.
func TestConsumerAppliesE1FromTheP2PQueue(t *testing.T) {
	e := newEnv(t) // sin sondeo: solo cola + consumidor
	travelerID, token := e.signUp(t, "ana@example.com")
	ctx := context.Background()
	start := time.Now().AddDate(0, 0, 5).UTC().Truncate(time.Second)
	good := events.ProviderUpdateNormalized{
		EventID: "e1-ok", TipoEvento: "ARREGLO_CREADO", IDViajero: travelerID,
		Proveedor:        events.ProviderRef{Tipo: "AGENCIA", IDProveedor: "agencia:otra"},
		GeneradoEnFuente: time.Now().Add(-time.Second), RecibidoEn: time.Now(),
		Arreglo: events.NormalizedArrangement{
			Tipo: "BUS", ClaveDeduplicacion: "BUS|norte|2027-01-01", Estado: "CONFIRMADO", FechaInicio: start, FechaFin: start.Add(time.Hour),
			Detalle: map[string]any{"titulo": "Bus a la costa", "operador": "Norte"},
		},
	}
	bad := good
	bad.EventID, bad.Arreglo.Tipo = "e1-malo", "CRUCERO"
	_ = e.reg.MessageQueue.Publish(ctx, bad)
	_ = e.reg.MessageQueue.Publish(ctx, good)

	eventually(t, 3*time.Second, "el E1 válido en el dashboard", func() bool {
		d := e.dashboardJSON(t, token)
		return len(d.Viajes) == 1 && len(d.Viajes[0].Arreglos) == 1 && d.Viajes[0].Arreglos[0].Titulo == "Bus a la costa" && d.Viajes[0].Arreglos[0].Origen == "AGENCIA"
	})
	eventually(t, time.Second, "ambos mensajes confirmados", func() bool { return e.reg.MessageQueue.Pending() == 0 })
}
