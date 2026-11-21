package httpdto

import (
	"encoding/json"
	"sort"
	"testing"
)

// keysOf serializa v y devuelve las claves JSON de primer nivel ordenadas.
func keysOf(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func assertKeys(t *testing.T, name string, v any, want ...string) {
	t.Helper()
	sort.Strings(want)
	got := keysOf(t, v)
	if len(got) != len(want) {
		t.Fatalf("%s: claves %v, se esperaban %v", name, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s: claves %v, se esperaban %v", name, got, want)
		}
	}
}

func TestAuthDTOs(t *testing.T) {
	assertKeys(t, "LoginRequest", LoginRequest{}, "email", "password")
	assertKeys(t, "LoginResponse", LoginResponse{}, "accessToken", "refreshToken", "expiresIn", "tokenType")
	assertKeys(t, "RefreshRequest", RefreshRequest{}, "refreshToken")
	assertKeys(t, "RefreshResponse", RefreshResponse{}, "accessToken", "expiresIn", "tokenType")
	assertKeys(t, "RegisterRequest", RegisterRequest{}, "email", "password")
	assertKeys(t, "RegisterResponse", RegisterResponse{}, "idViajero")
}

func TestDashboardDTOs(t *testing.T) {
	assertKeys(t, "DashboardResponse", DashboardResponse{}, "viajes", "avisosProveedores")
	assertKeys(t, "TripSummary", TripSummary{}, "idViaje", "nombre", "destino", "fechaInicio", "fechaFin", "arreglos")
	assertKeys(t, "ArrangementSummary", ArrangementSummary{}, "idArreglo", "tipo", "titulo", "fechaInicio", "estado", "origen")
	assertKeys(t, "ProviderNotice", ProviderNotice{}, "proveedor", "mensaje")
	// El resumen del dashboard no incluye detalle (payload reducido, ADR-17).
	for _, k := range keysOf(t, ArrangementSummary{}) {
		if k == "detalle" {
			t.Fatal("ArrangementSummary no debe incluir detalle")
		}
	}
}

func TestArrangementDTOs(t *testing.T) {
	assertKeys(t, "ArrangementDetailResponse", ArrangementDetailResponse{},
		"idArreglo", "idViaje", "tipo", "estado", "origen", "remitente", "detalle", "actualizadoEn")
	assertKeys(t, "ArrangementRequest", ArrangementRequest{}, "idViaje", "tipo", "titulo", "fechaInicio", "fechaFin", "detalle")
	assertKeys(t, "CreateArrangementResponse", CreateArrangementResponse{}, "idArreglo")
}

func TestRemitenteIsNullable(t *testing.T) {
	raw, _ := json.Marshal(ArrangementDetailResponse{})
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if v, ok := m["remitente"]; !ok || v != nil {
		t.Fatalf("remitente debe serializarse como null, obtuvo %v", v)
	}
}

func TestRecommendationDTOs(t *testing.T) {
	assertKeys(t, "RecommendationsResponse", RecommendationsResponse{}, "recomendaciones")
	assertKeys(t, "RecommendationItem", RecommendationItem{}, "idRecomendacion", "categoria", "titulo", "metadatos", "proveedor")
	assertKeys(t, "FeedbackRequest", FeedbackRequest{Accion: "AJUSTAR", ParametrosAjustados: map[string]any{"presupuestoMax": 120}}, "accion", "parametrosAjustados")
	assertKeys(t, "FeedbackResponse", FeedbackResponse{}, "registrado")
	assertKeys(t, "UpdatePreferencesRequest", UpdatePreferencesRequest{}, "parametros")
}

func TestEmailDTOs(t *testing.T) {
	assertKeys(t, "LinkEmailRequest", LinkEmailRequest{}, "proveedor", "authorizationCode", "redirectUri")
	assertKeys(t, "LinkEmailResponse", LinkEmailResponse{}, "idCuenta", "proveedor", "estado")
}

func TestItineraryDTOs(t *testing.T) {
	assertKeys(t, "SharedItineraryResponse", SharedItineraryResponse{}, "idItinerario", "nombre", "arreglos")
	assertKeys(t, "AccessDeniedResponse", AccessDeniedResponse{}, "codigo", "puedeSolicitar")
	assertKeys(t, "CreateItineraryRequest", CreateItineraryRequest{}, "nombre", "idsArreglos")
	assertKeys(t, "CreateItineraryResponse", CreateItineraryResponse{}, "idItinerario", "linkPublico")
	assertKeys(t, "AddArrangementRequest", AddArrangementRequest{}, "idArreglo")
	assertKeys(t, "CreateAccessRequestResponse", CreateAccessRequestResponse{}, "idSolicitud", "estado")
	assertKeys(t, "AccessRequestsResponse", AccessRequestsResponse{}, "solicitudes")
	assertKeys(t, "AccessRequestItem", AccessRequestItem{}, "idSolicitud", "idViajero", "estado")
	assertKeys(t, "DecisionRequest", DecisionRequest{}, "decision")
	assertKeys(t, "DecisionResponse", DecisionResponse{}, "estado")
}
