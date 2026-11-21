package ports

import (
	"context"

	auth "github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
	recommendations "github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
	arrangements "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// TravelerHistory es el historial unificado del viajero tras las consultas concurrentes.
type TravelerHistory struct {
	Trips        []arrangements.Trip
	Arrangements []arrangements.TravelArrangement
	Feedback     []recommendations.RecommendationFeedback
	Preferences  auth.TravelerPreferences
}

// HistoryPort [ADR-24] Consultas concurrentes del historial por tabla, que luego se unifican.
type HistoryPort interface {
	Get(ctx context.Context, travelerID string) (TravelerHistory, error)
}
