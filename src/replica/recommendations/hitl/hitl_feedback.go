// Package hitl es el componente "Retroalimentación HITL (Analyzer + Updater)" de Recomendaciones (AD §4.4).
package hitl

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
)

// FeedbackAnalyzer [ADR-25] Analiza la interacción del viajero (ACEPTAR/RECHAZAR/AJUSTAR) y deriva
// los cambios de preferencias.
type FeedbackAnalyzer struct{}

// Analyze traduce un feedback en parámetros de preferencias actualizados.
func (*FeedbackAnalyzer) Analyze(context.Context, domain.RecommendationFeedback) (map[string]any, error) {
	return nil, apierror.ErrNotImplemented
}

// PreferencesUpdater [ADR-25][QA-18] Escribe las preferencias actualizadas vía cola de prioridades,
// para que la siguiente recomendación las use.
type PreferencesUpdater struct {
	queue ports.PriorityQueuePort
}

// Update persiste las preferencias del viajero.
func (*PreferencesUpdater) Update(ctx context.Context, travelerID string, params map[string]any) error {
	return apierror.ErrNotImplemented
}

// HITLFeedback agrupa Analyzer y Updater. Lógica en la Fase 5.
type HITLFeedback struct {
	Analyzer *FeedbackAnalyzer
	Updater  *PreferencesUpdater
}

// NewHITLFeedback crea la retroalimentación HITL.
func NewHITLFeedback(queue ports.PriorityQueuePort) *HITLFeedback {
	return &HITLFeedback{Analyzer: &FeedbackAnalyzer{}, Updater: &PreferencesUpdater{queue: queue}}
}

// Register analiza el feedback y actualiza las preferencias del viajero.
func (*HITLFeedback) Register(ctx context.Context, travelerID, recommendationID string, feedback domain.RecommendationFeedback) error {
	return apierror.ErrNotImplemented
}
