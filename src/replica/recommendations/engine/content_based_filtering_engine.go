// Package engine es el componente "Motor de filtrado basado en contenido" de Recomendaciones (AD §4.4).
package engine

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
	auth "github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
	arrangements "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// Claves de las preferencias que el motor interpreta (RecommendationParams y secciones por categoría).
const (
	ParamMaxBudget        = "presupuestoMax"     // tope de precio: los ítems más caros se excluyen
	ParamPreferredDest    = "destinoPreferido"   // destino que se favorece
	PrefPreferredProvider = "proveedorPreferido" // en la sección de la categoría del ítem
	ParamMinStars         = "estrellasMin"       // solo alojamiento
)

// Pesos del puntaje por similitud de metadatos. Determinístico: mismos datos → mismo ranking.
const (
	weightHistoryDestination = 3 // el destino aparece en un viaje o reserva del historial
	weightPreferredDest      = 2
	weightPreferredProvider  = 2
	weightAccepted           = 4 // el viajero aceptó este ítem
	weightSameAsAccepted     = 1 // comparte destino o proveedor con un ítem aceptado
	weightCategoryHistory    = 1 // la categoría coincide con lo que el viajero ya reservó
)

// ContentBasedFilteringEngine [ADR-25] Filtra y puntúa el catálogo de proveedores por similitud de metadatos
// con el historial y las preferencias del viajero (filtrado basado en contenido, sin datos de otros viajeros).
// El feedback del viajero (QA-18) entra por las preferencias y por el historial de feedback: se aplica a la
// SIGUIENTE recomendación.
type ContentBasedFilteringEngine struct{ now func() time.Time }

// NewContentBasedFilteringEngine crea el motor.
func NewContentBasedFilteringEngine() *ContentBasedFilteringEngine {
	return &ContentBasedFilteringEngine{now: time.Now}
}

// Filter devuelve las recomendaciones ordenadas (mayor puntaje primero; empates por id del ítem).
func (e *ContentBasedFilteringEngine) Filter(_ context.Context, history ports.TravelerHistory, catalog []domain.CatalogItem) ([]domain.Recommendation, error) {
	rejected, accepted := feedbackSets(history.Feedback)
	acceptedItems := itemsByID(catalog, accepted)
	histDest := historyText(history)
	histCategories := historyCategories(history.Arrangements)
	params := history.Preferences.RecommendationParams
	maxBudget, hasBudget := number(params[ParamMaxBudget])
	minStars, hasStars := number(params[ParamMinStars])
	preferredDest, _ := params[ParamPreferredDest].(string)

	type scored struct {
		item  domain.CatalogItem
		score int
	}
	var ranked []scored
	for _, it := range catalog {
		if rejected[it.ItemID] {
			continue // el viajero lo rechazó
		}
		if price, ok := number(it.Metadata["precio"]); hasBudget && ok && price > maxBudget {
			continue // supera el presupuesto ajustado por el viajero [QA-18]
		}
		if stars, ok := number(it.Metadata["estrellas"]); hasStars && ok && stars < minStars {
			continue
		}
		dest, _ := it.Metadata["destino"].(string)
		score := 0
		if dest != "" && strings.Contains(histDest, strings.ToLower(dest)) {
			score += weightHistoryDestination
		}
		if preferredDest != "" && strings.EqualFold(dest, preferredDest) {
			score += weightPreferredDest
		}
		if pp, _ := sectionOf(history.Preferences, it.Category)[PrefPreferredProvider].(string); pp != "" && strings.EqualFold(pp, it.Provider) {
			score += weightPreferredProvider
		}
		if accepted[it.ItemID] {
			score += weightAccepted
		}
		for _, a := range acceptedItems {
			if a.ItemID == it.ItemID {
				continue
			}
			if adest, _ := a.Metadata["destino"].(string); (adest != "" && adest == dest) || a.Provider == it.Provider {
				score += weightSameAsAccepted
				break
			}
		}
		if histCategories[it.Category] {
			score += weightCategoryHistory
		}
		ranked = append(ranked, scored{item: it, score: score})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].item.ItemID < ranked[j].item.ItemID
	})

	now := e.now().UTC()
	out := make([]domain.Recommendation, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, domain.Recommendation{
			RecommendationID: r.item.ItemID, TravelerID: history.Preferences.TravelerID, Category: r.item.Category,
			Title: r.item.Title, Provider: r.item.Provider, ItemMetadata: r.item.Metadata, GeneratedAt: now,
		})
	}
	return out, nil
}

// feedbackSets deriva los ítems rechazados y aceptados: gana la última acción de cada ítem; AJUSTAR no cambia el
// estado de un ítem (solo actualiza las preferencias).
func feedbackSets(feedback []domain.RecommendationFeedback) (rejected, accepted map[string]bool) {
	rejected, accepted = map[string]bool{}, map[string]bool{}
	ordered := append([]domain.RecommendationFeedback(nil), feedback...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].At.Before(ordered[j].At) })
	for _, f := range ordered {
		switch f.Action {
		case domain.FeedbackAccept:
			accepted[f.RecommendationID], rejected[f.RecommendationID] = true, false
		case domain.FeedbackReject:
			rejected[f.RecommendationID], accepted[f.RecommendationID] = true, false
		}
	}
	return rejected, accepted
}

func itemsByID(catalog []domain.CatalogItem, ids map[string]bool) []domain.CatalogItem {
	var out []domain.CatalogItem
	for _, it := range catalog {
		if ids[it.ItemID] {
			out = append(out, it)
		}
	}
	return out
}

// historyText junta destinos de viajes y títulos de reservas en minúsculas para comparar destinos por contenido.
func historyText(h ports.TravelerHistory) string {
	var b strings.Builder
	for _, t := range h.Trips {
		b.WriteString(strings.ToLower(t.Destination) + " ")
	}
	for _, a := range h.Arrangements {
		b.WriteString(strings.ToLower(a.Title) + " ")
	}
	return b.String()
}

// historyCategories indica en qué categorías del catálogo el viajero ya tiene reservas.
func historyCategories(as []arrangements.TravelArrangement) map[domain.Category]bool {
	out := map[domain.Category]bool{}
	for _, a := range as {
		if a.Type == arrangements.TypeAccommodation {
			out[domain.CategoryAccommodation] = true
		} else {
			out[domain.CategoryTransport] = true
		}
	}
	return out
}

// sectionOf devuelve la sección de preferencias de la categoría del ítem.
func sectionOf(p auth.TravelerPreferences, c domain.Category) map[string]any {
	switch c {
	case domain.CategoryAccommodation:
		return p.AccommodationPreferences
	case domain.CategoryTransport:
		return p.TransportPreferences
	default:
		return p.RoutePreferences
	}
}

// number lee un número de un mapa decodificado de JSON (float64) o armado en Go (int).
func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
