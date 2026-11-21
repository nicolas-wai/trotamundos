// Package recommendations es el módulo "Recomendaciones" de la réplica (AD §4.4).
package recommendations

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/collector"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/engine"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/hitl"
)

// Recommender [ADR-23][ADR-24][ADR-25] Compone ConcurrentCollector, ContentBasedFilteringEngine y HITLFeedback
// detrás de RecommenderPort. Lógica en la Fase 5.
type Recommender struct {
	Collector *collector.ConcurrentCollector
	Engine    *engine.ContentBasedFilteringEngine
	HITL      *hitl.HITLFeedback
}

var _ ports.RecommenderPort = (*Recommender)(nil)

// NewRecommender ensambla el módulo sobre la cola de prioridades.
func NewRecommender(queue ports.PriorityQueuePort) *Recommender {
	return &Recommender{
		Collector: collector.NewConcurrentCollector(queue),
		Engine:    engine.NewContentBasedFilteringEngine(),
		HITL:      hitl.NewHITLFeedback(queue),
	}
}

// Generate genera las recomendaciones del viajero (máx. 3 s, QA-17).
func (*Recommender) Generate(context.Context, string) ([]domain.Recommendation, error) {
	return nil, apierror.ErrNotImplemented
}

// RegisterFeedback registra feedback HITL para la próxima recomendación (QA-18).
func (*Recommender) RegisterFeedback(context.Context, string, string, domain.FeedbackAction, map[string]any) error {
	return apierror.ErrNotImplemented
}
