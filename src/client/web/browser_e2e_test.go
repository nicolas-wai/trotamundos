//go:build e2e

// Pruebas de navegador real (Playwright + Chromium headless) del cliente web. Solo se ejecutan con
// `make test-e2e` (tag `e2e`), para que `go test ./...` no requiera navegador instalado.
// Cubren la parte JS que las pruebas Go no alcanzan: [ADR-18] optimistic update con rollback,
// [ADR-19] preferencias visuales en LocalStorage y [ADR-17][ADR-20] dashboard SSR inyectado.
package web_test

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/nicolas-wai/trotamundos/src/config"
	"github.com/nicolas-wai/trotamundos/src/region"
)

// startRegion levanta una región completa (LB → réplicas → cola → nodo de datos) y devuelve la URL del balanceador.
func startRegion(t *testing.T) string {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.ShardCount = 2
	cfg.HeartbeatInterval = 50 * time.Millisecond
	reg, err := region.New(cfg, region.Options{PasswordCost: 4})
	if err != nil {
		t.Fatal(err)
	}
	var urls []*url.URL
	var servers []*httptest.Server
	for _, rep := range reg.Replicas {
		s := httptest.NewServer(rep.Handler())
		servers = append(servers, s)
		u, _ := url.Parse(s.URL)
		urls = append(urls, u)
	}
	lb := httptest.NewServer(reg.NewLoadBalancer(urls))
	ctx, cancel := context.WithCancel(context.Background())
	reg.Start(ctx)
	t.Cleanup(func() {
		cancel()
		lb.Close()
		for _, s := range servers {
			s.Close()
		}
		reg.Close()
	})
	return lb.URL
}

func newPage(t *testing.T) playwright.Page {
	t.Helper()
	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("playwright: %v (¿instalaste el navegador? ver `make e2e-install`)", err)
	}
	browser, err := pw.Chromium.Launch()
	if err != nil {
		t.Fatalf("chromium: %v (¿instalaste el navegador? ver `make e2e-install`)", err)
	}
	t.Cleanup(func() { _ = browser.Close(); _ = pw.Stop() })
	page, err := browser.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	page.SetDefaultTimeout(8000)
	return page
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// signUpInBrowser registra y entra desde la UI; deja el dashboard visible.
func signUpInBrowser(t *testing.T, page playwright.Page, base, email string) {
	t.Helper()
	_, err := page.Goto(base + "/")
	must(t, err)
	must(t, page.Fill("input[name=email]", email))
	must(t, page.Fill("input[name=password]", "clave-segura-1"))
	must(t, page.Click("#register-button"))
	must(t, page.Locator(".trip").WaitFor())
}

func addBooking(t *testing.T, page playwright.Page, title string) {
	t.Helper()
	must(t, page.Click("[data-action=add]"))
	must(t, page.Fill("input[name=titulo]", title))
	must(t, page.Fill("input[name=fechaInicio]", "2030-05-01T10:00"))
	must(t, page.Click("#arrangement-form button[type=submit]"))
}

func count(t *testing.T, page playwright.Page, selector string) int {
	t.Helper()
	n, err := page.Locator(selector).Count()
	must(t, err)
	return n
}

// [ADR-18] Éxito: la reserva aparece al instante y queda confirmada con id real; luego se elimina.
func TestBrowserOptimisticCreateAndDelete(t *testing.T) {
	base := startRegion(t)
	page := newPage(t)
	signUpInBrowser(t, page, base, "ana@example.com")

	addBooking(t, page, "Vuelo a Lima")
	item := page.Locator(".arrangement", playwright.PageLocatorOptions{HasText: "Vuelo a Lima"})
	must(t, item.WaitFor())
	// Tras la confirmación el id deja de ser provisional (tmp-…).
	deadline := time.Now().Add(5 * time.Second)
	for {
		id, err := item.GetAttribute("data-id")
		must(t, err)
		if id != "" && !strings.HasPrefix(id, "tmp-") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("la reserva nunca se confirmó, id=%q", id)
		}
		time.Sleep(50 * time.Millisecond)
	}

	must(t, item.Locator("[data-action=delete]").Click())
	deadline = time.Now().Add(5 * time.Second)
	for count(t, page, ".arrangement") != 0 {
		if time.Now().After(deadline) {
			t.Fatal("la reserva no desapareció tras eliminarla")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// [ADR-18] Fallo: si el servidor rechaza el POST, la reserva optimista se revierte y se avisa al usuario.
func TestBrowserOptimisticRollbackOnServerError(t *testing.T) {
	base := startRegion(t)
	page := newPage(t)
	signUpInBrowser(t, page, base, "beto@example.com")

	must(t, page.Route("**/arreglos", func(route playwright.Route) {
		if route.Request().Method() == "POST" {
			_ = route.Fulfill(playwright.RouteFulfillOptions{
				Status: playwright.Int(500), ContentType: playwright.String("application/json"),
				Body: `{"codigo":"ERROR_INTERNO","mensaje":"falla simulada"}`,
			})
			return
		}
		_ = route.Continue()
	}))

	addBooking(t, page, "Hotel fantasma")
	banner := page.Locator("#banner")
	must(t, banner.WaitFor())
	text, err := banner.TextContent()
	must(t, err)
	if !strings.Contains(text, "Se restauró el estado anterior") {
		t.Fatalf("aviso inesperado: %q", text)
	}
	if n := count(t, page, ".arrangement"); n != 0 {
		t.Fatalf("el rollback debía quitar la reserva optimista, quedan %d", n)
	}
}

// [ADR-19] El tema elegido se guarda en LocalStorage y se aplica en el primer render tras recargar.
func TestBrowserThemePersistsInLocalStorage(t *testing.T) {
	base := startRegion(t)
	page := newPage(t)
	signUpInBrowser(t, page, base, "carla@example.com")

	before, err := page.Locator("html").GetAttribute("data-theme")
	must(t, err)
	must(t, page.Click("#theme-toggle"))
	after, err := page.Locator("html").GetAttribute("data-theme")
	must(t, err)
	if before == after {
		t.Fatalf("el tema no cambió: %q", after)
	}

	_, err = page.Reload()
	must(t, err)
	reloaded, err := page.Locator("html").GetAttribute("data-theme")
	must(t, err)
	if reloaded != after {
		t.Fatalf("el tema no persistió tras recargar: esperado %q, hay %q", after, reloaded)
	}
}

// [ADR-16] Salir borra la sesión: se vuelve al login y el dashboard deja de estar visible.
func TestBrowserLogoutReturnsToLogin(t *testing.T) {
	base := startRegion(t)
	page := newPage(t)
	signUpInBrowser(t, page, base, "dani@example.com")
	must(t, page.Click("#logout-button"))
	must(t, page.Locator("#login-view").WaitFor())
	if n := count(t, page, ".trip"); n != 0 {
		t.Fatalf("el dashboard sigue visible tras salir (%d viajes)", n)
	}
}
