package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicolas-wai/trotamundos/src/ports"
	authapp "github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	"github.com/nicolas-wai/trotamundos/src/replica/ssr"
	"github.com/nicolas-wai/trotamundos/src/replica/tokenvalidator"
)

// failingAuthentication es el único servicio que las pruebas de ruta pueden invocar (las rutas de autenticación son
// públicas): siempre falla con un error genérico. Los flujos reales se prueban en src/region.
type failingAuthentication struct{ authapp.AuthenticationService }

func (failingAuthentication) Register(context.Context, string, string) (string, error) {
	return "", errors.New("fallo de prueba")
}
func (failingAuthentication) Login(context.Context, string, string) (ports.TokenPair, error) {
	return ports.TokenPair{}, errors.New("fallo de prueba")
}
func (failingAuthentication) Refresh(context.Context, string) (ports.AccessToken, error) {
	return ports.AccessToken{}, errors.New("fallo de prueba")
}
func (failingAuthentication) Logout(context.Context, string) error {
	return errors.New("fallo de prueba")
}

// newTestRouter arma el router sin más módulos: las rutas protegidas se rechazan por token antes de llegar a un
// servicio, o solo se consulta el patrón registrado.
func newTestRouter() http.Handler {
	return NewRouter(NewControllers(Dependencies{Authentication: failingAuthentication{}, Renderer: ssr.NewSSRRenderer()}),
		tokenvalidator.NewSignedTokenValidator(fakeTokens{}))
}

// fakeTokens acepta únicamente el token "ok" (la validación real se prueba en tokenvalidator y en la integración).
type fakeTokens struct{ ports.TokenServicePort }

func (fakeTokens) Validate(token string) (ports.Claims, error) {
	if token == "ok" {
		return ports.Claims{TravelerID: "v1"}, nil
	}
	return ports.Claims{}, ports.ErrTokenInvalid
}

func isPublic(path string) bool {
	return path == "/login" || path == "/token/refresh" || path == "/logout" || path == "/registro"
}

// contractRoutes es la lista independiente de las rutas de AD §6.2 (método y ruta concreta).
var contractRoutes = []struct{ method, path string }{
	{"POST", "/login"},
	{"POST", "/token/refresh"},
	{"POST", "/logout"},
	{"POST", "/registro"},
	{"GET", "/dashboard"},
	{"GET", "/arreglos/a1"},
	{"POST", "/arreglos"},
	{"PUT", "/arreglos/a1"},
	{"DELETE", "/arreglos/a1"},
	{"GET", "/recomendaciones"},
	{"POST", "/recomendaciones/r1/feedback"},
	{"PUT", "/perfil/preferencias"},
	{"POST", "/correos"},
	{"DELETE", "/correos/c1"},
	{"GET", "/itinerarios/i1"},
	{"POST", "/itinerarios"},
	{"DELETE", "/itinerarios/i1"},
	{"POST", "/itinerarios/i1/arreglos"},
	{"DELETE", "/itinerarios/i1/arreglos/a1"},
	{"POST", "/itinerarios/i1/solicitudes"},
	{"GET", "/itinerarios/i1/solicitudes"},
	{"POST", "/itinerarios/i1/solicitudes/s1/decision"},
	{"DELETE", "/itinerarios/i1/accesos/v1"},
}

// Cada ruta de AD §6.2 debe estar registrada en el router: se consulta el mux sin invocar el handler.
func TestEveryContractRouteIsRegistered(t *testing.T) {
	if len(contractRoutes) != 23 {
		t.Fatalf("AD §6.2 define 23 rutas, la tabla tiene %d", len(contractRoutes))
	}
	mux, ok := newTestRouter().(*http.ServeMux)
	if !ok {
		t.Fatal("NewRouter debe devolver un *http.ServeMux")
	}
	for _, rt := range contractRoutes {
		req := httptest.NewRequest(rt.method, rt.path, nil)
		if _, pattern := mux.Handler(req); pattern == "" {
			t.Errorf("%s %s no está registrada", rt.method, rt.path)
		}
	}
}

// Sin token (o con token inválido) toda ruta protegida responde 401 TOKEN_INVALIDO; las de autenticación son públicas.
func TestProtectedRoutesRequireToken(t *testing.T) {
	router := newTestRouter()
	for _, rt := range contractRoutes {
		for _, auth := range []string{"", "Bearer basura", "Basic ok"} {
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader("{}"))
			if auth != "" {
				req.Header.Set("Authorization", auth)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if isPublic(rt.path) {
				if rec.Code == http.StatusUnauthorized && strings.Contains(rec.Body.String(), "TOKEN_") {
					t.Fatalf("%s %s debe ser pública", rt.method, rt.path)
				}
				continue
			}
			var body map[string]string
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			if rec.Code != http.StatusUnauthorized || body["codigo"] != "TOKEN_INVALIDO" {
				t.Fatalf("%s %s con %q: %d %v, se esperaba 401 TOKEN_INVALIDO", rt.method, rt.path, auth, rec.Code, body)
			}
		}
	}
}

func TestUnknownRouteIsNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, httptest.NewRequest("GET", "/no-existe", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("código %d, se esperaba 404", rec.Code)
	}
}
