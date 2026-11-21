// Package recommendations es el módulo "Recomendaciones" de la réplica (AD §4.4).
package recommendations

import (
	"context"
	"fmt"

	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/collector"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/engine"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/hitl"
)

// ErrInvalidFeedback: el feedback no es válido (acción desconocida o AJUSTAR sin parámetros) → 422.
var ErrInvalidFeedback = hitl.ErrInvalidFeedback

// ErrUnknownRecommendation: la recomendación no existe → 404.
var ErrUnknownRecommendation = hitl.ErrUnknownRecommendation

// Recommender [ADR-23][ADR-24][ADR-25] Compone ConcurrentCollector, ContentBasedFilteringEngine y HITLFeedback
// detrás de RecommenderPort. El tiempo máximo (3 s, QA-17) lo impone quien lo invoca mediante el contexto.
type Recommender struct {
	Collector *collector.ConcurrentCollector
	Engine    *engine.ContentBasedFilteringEngine
	HITL      *hitl.HITLFeedback

	catalog ports.ProviderCatalogPort
}

var _ ports.RecommenderPort = (*Recommender)(nil)

// NewRecommender ensambla el módulo: toda lectura/escritura de datos sale por la cola de prioridades [ADR-09].
func NewRecommender(data ports.DataGateway, preferences PreferencesStore, catalog ports.ProviderCatalogPort) *Recommender {
	return &Recommender{
		Collector: collector.NewConcurrentCollector(data, preferences),
		Engine:    engine.NewContentBasedFilteringEngine(),
		HITL:      hitl.NewHITLFeedback(data, preferences, catalog),
		catalog:   catalog,
	}
}

// PreferencesStore es el repositorio de preferencias del perfil que usan recolector y HITL.
type PreferencesStore interface {
	collector.PreferencesReader
	hitl.PreferencesStore
}

// Generate recolecta historial y perfil en paralelo y consulta el catálogo de proveedores a la vez, y luego
// filtra por contenido. Cada paso respeta la cancelación del contexto (límite de 3 s, QA-17).
func (r *Recommender) Generate(ctx context.Context, travelerID string) ([]domain.Recommendation, error) {
	type catalogResult struct {
		items []domain.CatalogItem
		err   error
	}
	catalogCh := make(chan catalogResult, 1)
	go func() {
		items, err := r.catalog.List(ctx)
		catalogCh <- catalogResult{items, err}
	}()

	history, err := r.Collector.Get(ctx, travelerID)
	if err != nil {
		return nil, fmt.Errorf("recolectando historial: %w", err)
	}
	var cat catalogResult
	select {
	case cat = <-catalogCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if cat.err != nil {
		return nil, fmt.Errorf("consultando catálogo de proveedores: %w", cat.err)
	}
	history.Preferences.TravelerID = travelerID
	return r.Engine.Filter(ctx, history, cat.items)
}

// RegisterFeedback registra feedback HITL para la próxima recomendación (QA-18).
func (r *Recommender) RegisterFeedback(ctx context.Context, travelerID, recommendationID string, action domain.FeedbackAction, params map[string]any) error {
	return r.HITL.Register(ctx, travelerID, recommendationID, domain.RecommendationFeedback{Action: action, AdjustedParams: params})
}
