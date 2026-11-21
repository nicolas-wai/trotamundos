// Package domain (recommendations) modela el agregado "Recomendación" (AD §2.2).
package domain

import "time"

// Category es la categoría de una recomendación.
type Category string

// Categorías del contrato (AD §6.2).
const (
	CategoryAccommodation Category = "ALOJAMIENTO"
	CategoryTransport     Category = "TRANSPORTE"
	CategoryRoute         Category = "RUTA"
)

// Recommendation es la raíz del agregado. Se genera a partir de historial y preferencias;
// el feedback se aplica a la siguiente recomendación (QA-18). [ADR-25]
type Recommendation struct {
	RecommendationID string
	TravelerID       string
	Category         Category
	ItemMetadata     map[string]any
	GeneratedAt      time.Time
	Feedback         []RecommendationFeedback
}

// FeedbackAction es la acción del viajero sobre una recomendación (human-in-the-loop).
type FeedbackAction string

// Acciones HITL del contrato.
const (
	FeedbackAccept FeedbackAction = "ACEPTAR"
	FeedbackReject FeedbackAction = "RECHAZAR"
	FeedbackAdjust FeedbackAction = "AJUSTAR"
)

// RecommendationFeedback es el feedback que actualiza las preferencias del viajero. [QA-18]
type RecommendationFeedback struct {
	Action         FeedbackAction
	AdjustedParams map[string]any
	At             time.Time
}
