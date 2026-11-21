package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicolas-wai/trotamundos/src/ports"
	authapp "github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/oauthmanager"
	permapp "github.com/nicolas-wai/trotamundos/src/replica/permissions/application"
	"github.com/nicolas-wai/trotamundos/src/replica/priorityqueueclient"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations"
	"github.com/nicolas-wai/trotamundos/src/replica/ssr"
	"github.com/nicolas-wai/trotamundos/src/replica/tokenvalidator"
	taapp "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
	"github.com/nicolas-wai/trotamundos/src/stubs"
)

func newTestRouter() http.Handler {
	oauth := oauthmanager.NewOAuthManager(stubs.NewMockOAuth2AuthAdapter())
	return NewRouter(NewControllers(Dependencies{
		Authentication: authapp.UnimplementedAuthenticationService{},
		Profile:        authapp.UnimplementedTravelerProfileService{},
		Arrangements:   taapp.UnimplementedTravelArrangementsService{},
		Permissions:    permapp.UnimplementedPermissionsService{},
		Recommender:    recommendations.NewRecommender(priorityqueueclient.NewClient(nil)),
		EmailLinking:   oauth,
		Renderer:       ssr.NewRenderer(),
	}), tokenvalidator.NewValidator(fakeTokens{}))
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

// Las rutas protegidas todavía sin módulo responden 501 NO_IMPLEMENTADO una vez superado el token (Fases 3-5).
func TestEveryProtectedRouteRespondsNotImplementedWithToken(t *testing.T) {
	if len(contractRoutes) != 23 {
		t.Fatalf("AD §6.2 define 23 rutas, la tabla tiene %d", len(contractRoutes))
	}
	router := newTestRouter()
	for _, rt := range contractRoutes {
		if isPublic(rt.path) {
			continue
		}
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader("{}"))
			req.Header.Set("Authorization", "Bearer ok")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("código %d, se esperaba 501", rec.Code)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["codigo"] != "NO_IMPLEMENTADO" || body["mensaje"] == "" {
				t.Fatalf("cuerpo de error inesperado: %v", body)
			}
		})
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
