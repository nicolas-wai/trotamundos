package api

import "net/http"

// Recomendaciones [ADR-23][ADR-25].

// Recommendations atiende GET /recomendaciones (request independiente del dashboard, límite 3 s, QA-17).
func (c *Controllers) Recommendations(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Recommender.Generate(r.Context(), travelerIDFrom(r.Context()))
	respondSkeleton(w, err)
}

// RecommendationFeedback atiende POST /recomendaciones/{idRecomendacion}/feedback (HITL, QA-18).
func (c *Controllers) RecommendationFeedback(w http.ResponseWriter, r *http.Request) {
	err := c.deps.Recommender.RegisterFeedback(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idRecomendacion"), "", nil)
	respondSkeleton(w, err)
}

// UpdatePreferences atiende PUT /perfil/preferencias.
func (c *Controllers) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	respondSkeleton(w, c.deps.Profile.UpdatePreferences(r.Context(), travelerIDFrom(r.Context()), nil))
}
