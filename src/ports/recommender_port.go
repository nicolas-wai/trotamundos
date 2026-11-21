package ports

import (
	"context"

	recommendations "github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
)

// RecommenderPort [ADR-23][ADR-25] Recomendaciones. Generar tiene un límite de 3 s (QA17) y se invoca
// de forma asíncrona. El feedback se usa en la siguiente recomendación (QA18).
type RecommenderPort interface {
	Generate(ctx context.Context, travelerID string) ([]recommendations.Recommendation, error)
	RegisterFeedback(ctx context.Context, travelerID, recommendationID string,
		action recommendations.FeedbackAction, params map[string]any) error
}
