package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authapp "github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/oauthmanager"
	permapp "github.com/nicolas-wai/trotamundos/src/replica/permissions/application"
	"github.com/nicolas-wai/trotamundos/src/replica/priorityqueueclient"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations"
	"github.com/nicolas-wai/trotamundos/src/replica/ssr"
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
		Recommender:    recommendations.NewRecommender(priorityqueueclient.NewClient()),
		EmailLinking:   oauth,
		Renderer:       ssr.NewRenderer(),
	}))
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

func TestEveryContractRouteRespondsNotImplemented(t *testing.T) {
	if len(contractRoutes) != 23 {
		t.Fatalf("AD §6.2 define 23 rutas, la tabla tiene %d", len(contractRoutes))
	}
	router := newTestRouter()
	for _, rt := range contractRoutes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(rt.method, rt.path, strings.NewReader("{}")))
			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("código %d, se esperaba 501", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("content-type %q", ct)
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

func TestUnknownRouteIsNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, httptest.NewRequest("GET", "/no-existe", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("código %d, se esperaba 404", rec.Code)
	}
}
