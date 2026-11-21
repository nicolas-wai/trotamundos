package hitl

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/nicolas-wai/trotamundos/src/contracts/resources"
	"github.com/nicolas-wai/trotamundos/src/ports"
	auth "github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
)

type recordingGateway struct {
	mu     sync.Mutex
	writes []string
	params []map[string]any
}

func (g *recordingGateway) Read(context.Context, string, string, map[string]any) (ports.DataResult, error) {
	return ports.DataResult{}, nil
}

func (g *recordingGateway) Write(_ context.Context, _ string, resource string, params map[string]any) (ports.DataResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.writes = append(g.writes, resource)
	g.params = append(g.params, params)
	return ports.DataResult{RowsAffected: 1}, nil
}

type memStore struct{ saved []auth.TravelerPreferences }

func (s *memStore) Get(context.Context, string) (auth.TravelerPreferences, error) {
	return auth.TravelerPreferences{RecommendationParams: map[string]any{"otro": 1}}, nil
}
func (s *memStore) Save(_ context.Context, p auth.TravelerPreferences) error {
	s.saved = append(s.saved, p)
	return nil
}

type fixedCatalog struct{}

func (fixedCatalog) List(context.Context) ([]domain.CatalogItem, error) {
	return []domain.CatalogItem{{
		ItemID: "alo-1", Category: domain.CategoryAccommodation, Provider: "StayCo",
		Metadata: map[string]any{"destino": "Lisboa", "precio": 95.0},
	}}, nil
}

func TestAnalyzerDerivesPreferenceDeltas(t *testing.T) {
	a := &FeedbackAnalyzer{}
	item, _ := fixedCatalog{}.List(context.Background())
	ctx := context.Background()

	d, err := a.Analyze(ctx, item[0], domain.RecommendationFeedback{Action: domain.FeedbackAdjust, AdjustedParams: map[string]any{"presupuestoMax": 120.0}})
	if err != nil || !reflect.DeepEqual(d.Recommendation, map[string]any{"presupuestoMax": 120.0}) {
		t.Fatalf("AJUSTAR: %+v %v", d, err)
	}
	d, err = a.Analyze(ctx, item[0], domain.RecommendationFeedback{Action: domain.FeedbackAccept})
	if err != nil || d.Recommendation["destinoPreferido"] != "Lisboa" || d.Accommodation["proveedorPreferido"] != "StayCo" {
		t.Fatalf("ACEPTAR: %+v %v", d, err)
	}
	d, err = a.Analyze(ctx, item[0], domain.RecommendationFeedback{Action: domain.FeedbackReject})
	if err != nil || !d.IsEmpty() {
		t.Fatalf("RECHAZAR no cambia preferencias: %+v %v", d, err)
	}
	if _, err = a.Analyze(ctx, item[0], domain.RecommendationFeedback{Action: domain.FeedbackAdjust}); !errors.Is(err, ErrInvalidFeedback) {
		t.Fatalf("AJUSTAR sin parámetros: %v", err)
	}
	if _, err = a.Analyze(ctx, item[0], domain.RecommendationFeedback{Action: "TAL_VEZ"}); !errors.Is(err, ErrInvalidFeedback) {
		t.Fatalf("acción desconocida: %v", err)
	}
}

func TestRegisterWritesFeedbackAndMergesPreferencesThroughTheQueue(t *testing.T) {
	gw, store := &recordingGateway{}, &memStore{}
	h := NewHITLFeedback(gw, store, fixedCatalog{})

	err := h.Register(context.Background(), "v1", "alo-1", domain.RecommendationFeedback{
		Action: domain.FeedbackAdjust, AdjustedParams: map[string]any{"presupuestoMax": 120.0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gw.writes, []string{resources.RecommendationFeedbackSave}) {
		t.Fatalf("escrituras: %v", gw.writes)
	}
	if gw.params[0]["idRecomendacion"] != "alo-1" || gw.params[0]["accion"] != "AJUSTAR" {
		t.Fatalf("feedback guardado: %v", gw.params[0])
	}
	if len(store.saved) != 1 {
		t.Fatalf("las preferencias debían guardarse una vez: %v", store.saved)
	}
	rp := store.saved[0].RecommendationParams
	if rp["presupuestoMax"] != 120.0 || rp["otro"] != 1 {
		t.Fatalf("el delta debe mezclarse sin perder lo anterior: %v", rp)
	}

	// RECHAZAR solo registra el feedback.
	store.saved = nil
	if err := h.Register(context.Background(), "v1", "alo-1", domain.RecommendationFeedback{Action: domain.FeedbackReject}); err != nil {
		t.Fatal(err)
	}
	if len(store.saved) != 0 {
		t.Fatal("RECHAZAR no debe reescribir preferencias")
	}
	if err := h.Register(context.Background(), "v1", "no-existe", domain.RecommendationFeedback{Action: domain.FeedbackAccept}); !errors.Is(err, ErrUnknownRecommendation) {
		t.Fatalf("recomendación inexistente: %v", err)
	}
}
