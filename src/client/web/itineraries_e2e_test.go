//go:build e2e

// Pruebas de navegador real de la pantalla "Mis itinerarios" [ADR-13] y de su actualización optimista [ADR-18].
package web_test

import (
	"strings"
	"testing"
	"time"

	playwright "github.com/mxschmitt/playwright-go"
)

// pollFast acelera la consulta periódica de solicitudes para que la prueba no espere los 5 s por defecto.
func pollFast(t *testing.T, page playwright.Page) {
	t.Helper()
	must(t, page.AddInitScript(playwright.Script{Content: playwright.String("window.TROTAMUNDOS_ITINERARY_POLL_MS = 150;")}))
}

func waitCount(t *testing.T, page playwright.Page, selector string, want int, msg string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for count(t, page, selector) != want {
		if time.Now().After(deadline) {
			t.Fatalf("%s: %q tiene %d, se esperaban %d", msg, selector, count(t, page, selector), want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// shareItinerary: la dueña crea un itinerario con una reserva y devuelve su código.
func shareItinerary(t *testing.T, owner playwright.Page, base string) string {
	t.Helper()
	pollFast(t, owner)
	signUpInBrowser(t, owner, base, "ana@example.com")
	addBooking(t, owner, "Vuelo a Lisboa")
	item := owner.Locator("li.arrangement", playwright.PageLocatorOptions{HasText: "Vuelo a Lisboa"})
	must(t, item.WaitFor())
	for { // esperar el id real (no tmp-…)
		id, err := item.GetAttribute("data-id")
		must(t, err)
		if id != "" && !strings.HasPrefix(id, "tmp-") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	must(t, owner.Fill("form[data-form=create-itinerary] input[name=nombre]", "Lisboa 2030"))
	must(t, owner.Click("form[data-form=create-itinerary] button[type=submit]"))
	article := owner.Locator("article.itinerary")
	must(t, article.WaitFor())
	must(t, article.Locator("[data-action=add-itinerary-arrangement]").Click())
	must(t, article.Locator("li.itinerary-arrangement", playwright.LocatorLocatorOptions{HasText: "Vuelo a Lisboa"}).WaitFor())
	code, err := article.Locator(".itinerary-link code").TextContent()
	must(t, err)
	return code
}

// openAsGuest abre el itinerario desde la vista del invitado.
func openAsGuest(t *testing.T, guest playwright.Page, code string) {
	t.Helper()
	must(t, guest.Fill("form[data-form=open-itinerary] input[name=enlace]", code))
	must(t, guest.Click("form[data-form=open-itinerary] button[type=submit]"))
}

func guestSees(t *testing.T, guest playwright.Page, text string) {
	t.Helper()
	must(t, guest.Locator("[data-role=guest-view]", playwright.PageLocatorOptions{HasText: text}).WaitFor())
}

// El invitado recibe 403 con puedeSolicitar y pide acceso; el dueño ve la solicitud sin recargar, la acepta y el
// invitado pasa a ver el itinerario. Luego el dueño revoca el acceso.
func TestBrowserOwnerAcceptsRequestAndRevokesAccess(t *testing.T) {
	base := startRegion(t)
	owner, guest := newPage(t), newPage(t)
	code := shareItinerary(t, owner, base)
	signUpInBrowser(t, guest, base, "beto@example.com")

	openAsGuest(t, guest, code)
	guestSees(t, guest, "No tienes acceso a este itinerario.")
	must(t, guest.Click("[data-action=request-access]"))
	guestSees(t, guest, "Solicitud enviada")

	// Aparece en la pantalla de la dueña sin recargar (consulta periódica).
	waitCount(t, owner, "article.itinerary li.request", 1, "solicitud pendiente visible")
	must(t, owner.Click("li.request [data-action=accept-request]"))
	// Optimista: sale de pendientes y pasa a accesos al instante.
	waitCount(t, owner, "li.request", 0, "la solicitud sale de pendientes")
	waitCount(t, owner, "li.access", 1, "el viajero pasa a accesos")
	waitCount(t, owner, "li.access.pending", 0, "el acceso queda confirmado")

	must(t, guest.Click("[data-action=reopen-itinerary]"))
	guestSees(t, guest, "Lisboa 2030")
	guestSees(t, guest, "Vuelo a Lisboa")

	must(t, owner.Click("li.access [data-action=revoke-access]"))
	waitCount(t, owner, "li.access", 0, "el acceso se revoca")
	openAsGuest(t, guest, code)
	guestSees(t, guest, "No tienes acceso a este itinerario.")
	if n := count(t, guest, ".shared-itinerary"); n != 0 {
		t.Fatalf("tras revocar el invitado no debe ver el itinerario (%d)", n)
	}
}

// Rama de rechazo: el rechazo bloquea; el invitado no vuelve a ver la opción de solicitar.
func TestBrowserOwnerRejectsRequest(t *testing.T) {
	base := startRegion(t)
	owner, guest := newPage(t), newPage(t)
	code := shareItinerary(t, owner, base)
	signUpInBrowser(t, guest, base, "caro@example.com")

	openAsGuest(t, guest, code)
	must(t, guest.Locator("[data-action=request-access]").WaitFor())
	must(t, guest.Click("[data-action=request-access]"))
	waitCount(t, owner, "li.request", 1, "solicitud pendiente visible")
	must(t, owner.Click("li.request [data-action=reject-request]"))
	waitCount(t, owner, "li.request", 0, "la solicitud sale de pendientes")
	if n := count(t, owner, "li.access"); n != 0 {
		t.Fatalf("rechazar no concede acceso (%d)", n)
	}

	openAsGuest(t, guest, code)
	guestSees(t, guest, "No tienes acceso a este itinerario.")
	if n := count(t, guest, "[data-action=request-access]"); n != 0 {
		t.Fatalf("el rechazo bloquea: no debe ofrecerse solicitar de nuevo (%d)", n)
	}
}

// [ADR-18] Si el servidor rechaza la decisión, la solicitud vuelve a pendientes y se avisa.
func TestBrowserDecisionRollsBackOnServerError(t *testing.T) {
	base := startRegion(t)
	owner, guest := newPage(t), newPage(t)
	code := shareItinerary(t, owner, base)
	signUpInBrowser(t, guest, base, "dani@example.com")
	openAsGuest(t, guest, code)
	must(t, guest.Locator("[data-action=request-access]").WaitFor())
	must(t, guest.Click("[data-action=request-access]"))
	waitCount(t, owner, "li.request", 1, "solicitud pendiente visible")

	must(t, owner.Route("**/decision", func(route playwright.Route) {
		_ = route.Fulfill(playwright.RouteFulfillOptions{
			Status: playwright.Int(500), ContentType: playwright.String("application/json"),
			Body: `{"codigo":"ERROR_INTERNO","mensaje":"falla simulada"}`,
		})
	}))
	must(t, owner.Click("li.request [data-action=accept-request]"))
	banner := owner.Locator("#banner")
	must(t, banner.WaitFor())
	text, err := banner.TextContent()
	must(t, err)
	if !strings.Contains(text, "Se restauró el estado anterior") {
		t.Fatalf("aviso inesperado: %q", text)
	}
	waitCount(t, owner, "li.request", 1, "el rollback devuelve la solicitud a pendientes")
	waitCount(t, owner, "li.access", 0, "el rollback quita el acceso optimista")
}
