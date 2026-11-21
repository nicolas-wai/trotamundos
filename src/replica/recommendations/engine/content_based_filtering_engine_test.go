package engine

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
	auth "github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
	arrangements "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

func catalog() []domain.CatalogItem {
	mk := func(id string, c domain.Category, provider, dest string, price float64, stars int) domain.CatalogItem {
		m := map[string]any{"destino": dest, "precio": price}
		if stars > 0 {
			m["estrellas"] = stars
		}
		return domain.CatalogItem{ItemID: id, Category: c, Title: id, Provider: provider, Metadata: m}
	}
	return []domain.CatalogItem{
		mk("alo-barato-lisboa", domain.CategoryAccommodation, "StayCo", "Lisboa", 95, 4),
		mk("alo-caro-lisboa", domain.CategoryAccommodation, "LuxStay", "Lisboa", 180, 5),
		mk("alo-roma", domain.CategoryAccommodation, "StayCo", "Roma", 150, 4),
		mk("tra-bus-oporto", domain.CategoryTransport, "BusLine", "Oporto", 25, 0),
		mk("rut-roma", domain.CategoryRoute, "WalkTours", "Roma", 55, 0),
	}
}

func ids(recs []domain.Recommendation) []string {
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = r.RecommendationID
	}
	return out
}

func filter(t *testing.T, h ports.TravelerHistory) []domain.Recommendation {
	t.Helper()
	recs, err := NewContentBasedFilteringEngine().Filter(context.Background(), h, catalog())
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func TestRankingIsDeterministicAndFollowsHistory(t *testing.T) {
	h := ports.TravelerHistory{
		Trips:        []arrangements.Trip{{Destination: "Lisboa"}},
		Arrangements: []arrangements.TravelArrangement{{Type: arrangements.TypeFlight, Title: "Vuelo a Lisboa"}},
	}
	first := ids(filter(t, h))
	// Lisboa (historial: +3, categoría con reservas: +1) va primero; empates por id.
	want := []string{"alo-barato-lisboa", "alo-caro-lisboa", "tra-bus-oporto", "alo-roma", "rut-roma"}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("ranking %v, se esperaba %v", first, want)
	}
	for i := 0; i < 5; i++ {
		if again := ids(filter(t, h)); !reflect.DeepEqual(again, first) {
			t.Fatalf("el ranking no es determinístico: %v vs %v", again, first)
		}
	}
}

// QA18: el presupuesto ajustado excluye los ítems que lo superan.
func TestBudgetAdjustmentExcludesExpensiveItems(t *testing.T) {
	h := ports.TravelerHistory{Preferences: auth.TravelerPreferences{RecommendationParams: map[string]any{"presupuestoMax": 120.0}}}
	for _, r := range filter(t, h) {
		if price := r.ItemMetadata["precio"].(float64); price > 120 {
			t.Fatalf("%s cuesta %v y supera el presupuesto", r.RecommendationID, price)
		}
	}
	if got := ids(filter(t, h)); len(got) != 3 {
		t.Fatalf("debían quedar 3 ítems ≤ 120, quedaron %v", got)
	}
	// Los números decodificados de JSON (float64) y los enteros de Go se tratan igual.
	h.Preferences.RecommendationParams["presupuestoMax"] = 60
	if got := ids(filter(t, h)); len(got) != 2 {
		t.Fatalf("con presupuesto 60 debían quedar 2: %v", got)
	}
}

func TestFeedbackShapesTheNextRanking(t *testing.T) {
	at := time.Now()
	h := ports.TravelerHistory{Feedback: []domain.RecommendationFeedback{
		{RecommendationID: "alo-barato-lisboa", Action: domain.FeedbackReject, At: at},
		{RecommendationID: "rut-roma", Action: domain.FeedbackAccept, At: at},
	}}
	got := ids(filter(t, h))
	for _, id := range got {
		if id == "alo-barato-lisboa" {
			t.Fatal("un ítem rechazado no debe volver a recomendarse")
		}
	}
	if got[0] != "rut-roma" {
		t.Fatalf("el ítem aceptado debe liderar el ranking: %v", got)
	}
	// La última acción de cada ítem gana: aceptar después de rechazar lo recupera.
	h.Feedback = append(h.Feedback, domain.RecommendationFeedback{RecommendationID: "alo-barato-lisboa", Action: domain.FeedbackAccept, At: at.Add(time.Second)})
	if got := ids(filter(t, h)); got[0] != "alo-barato-lisboa" && got[1] != "alo-barato-lisboa" {
		t.Fatalf("aceptar tras rechazar debía recuperar el ítem: %v", got)
	}
}

func TestPreferredProviderAndMinStars(t *testing.T) {
	h := ports.TravelerHistory{Preferences: auth.TravelerPreferences{
		AccommodationPreferences: map[string]any{"proveedorPreferido": "LuxStay"},
		RecommendationParams:     map[string]any{"estrellasMin": 5.0},
	}}
	got := ids(filter(t, h))
	if got[0] != "alo-caro-lisboa" {
		t.Fatalf("el proveedor preferido debía liderar: %v", got)
	}
	for _, id := range got {
		if id == "alo-barato-lisboa" || id == "alo-roma" {
			t.Fatalf("%s tiene menos de 5 estrellas", id)
		}
	}
}
