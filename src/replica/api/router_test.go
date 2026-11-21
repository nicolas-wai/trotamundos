package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
	authapp "github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	authdomain "github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
	permapp "github.com/nicolas-wai/trotamundos/src/replica/permissions/application"
	recdomain "github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/ssr"
	"github.com/nicolas-wai/trotamundos/src/replica/tokenvalidator"
	taapp "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
)

// unimplementedEmailLinking responde ErrNotImplemented: la vinculación real se prueba en src/region.
type unimplementedEmailLinking struct{}

func (unimplementedEmailLinking) Link(context.Context, string, string, string, string) (authdomain.LinkedEmailAccount, error) {
	return authdomain.LinkedEmailAccount{}, apierror.ErrNotImplemented
}

func (unimplementedEmailLinking) Unlink(context.Context, string, string) error {
	return apierror.ErrNotImplemented
}

// unimplementedRecommender responde ErrNotImplemented: el recomendador real se prueba en src/region.
type unimplementedRecommender struct{}

func (unimplementedRecommender) Generate(context.Context, string) ([]recdomain.Recommendation, error) {
	return nil, apierror.ErrNotImplemented
}

func (unimplementedRecommender) RegisterFeedback(context.Context, string, string, recdomain.FeedbackAction, map[string]any) error {
	return apierror.ErrNotImplemented
}

// bodyFor devuelve un cuerpo válido para la ruta: los handlers validan el JSON antes de llamar al módulo.
func bodyFor(path string) string {
	if strings.HasSuffix(path, "/feedback") {
		return `{"accion":"ACEPTAR"}`
	}
	return "{}"
}

func newTestRouter() http.Handler {
	return NewRouter(NewControllers(Dependencies{
		Authentication: authapp.UnimplementedAuthenticationService{},
		Profile:        authapp.UnimplementedTravelerProfileService{},
		Arrangements:   taapp.UnimplementedTravelArrangementsService{},
		Permissions:    permapp.UnimplementedPermissionsService{},
		Recommender:    unimplementedRecommender{},
		EmailLinking:   unimplementedEmailLinking{},
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

// Con los módulos sustituidos por stubs de la Fase 1, toda ruta protegida responde 501 NO_IMPLEMENTADO una vez
// superado el token: verifica que cada ruta de AD §6.2 esté registrada y traduzca el error al formato de §6.1.
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
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(bodyFor(rt.path)))
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
