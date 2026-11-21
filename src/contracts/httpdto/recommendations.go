package httpdto

// --- Recomendaciones [ADR-23, ADR-25] ---

// RecommendationsResponse es la respuesta 200 de GET /recomendaciones.
type RecommendationsResponse struct {
	Recomendaciones []RecommendationItem `json:"recomendaciones"`
}

// RecommendationItem es una recomendación individual.
type RecommendationItem struct {
	IDRecomendacion string         `json:"idRecomendacion"`
	Categoria       string         `json:"categoria"`
	Titulo          string         `json:"titulo"`
	Metadatos       map[string]any `json:"metadatos"`
	Proveedor       string         `json:"proveedor"`
}

// FeedbackRequest es el body de POST /recomendaciones/{idRecomendacion}/feedback [QA-18].
type FeedbackRequest struct {
	Accion              string         `json:"accion"`
	ParametrosAjustados map[string]any `json:"parametrosAjustados,omitempty"`
}

// FeedbackResponse es la respuesta 200 del feedback.
type FeedbackResponse struct {
	Registrado bool `json:"registrado"`
}

// UpdatePreferencesRequest es el body de PUT /perfil/preferencias.
type UpdatePreferencesRequest struct {
	Parametros map[string]any `json:"parametros"`
}
