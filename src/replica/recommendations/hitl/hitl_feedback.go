// Package hitl es el componente "Retroalimentación HITL (Analyzer + Updater)" de Recomendaciones (AD §4.4).
package hitl

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
	"github.com/nicolas-wai/trotamundos/src/ports"
	auth "github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
)

// Errores de la retroalimentación HITL.
var (
	ErrUnknownRecommendation = errors.New("recomendación no encontrada")
	ErrInvalidFeedback       = errors.New("feedback inválido")
)

// Claves de preferencias que escribe el Analyzer (las lee el motor de filtrado).
const (
	paramPreferredDest    = "destinoPreferido"
	prefPreferredProvider = "proveedorPreferido"
)

// PreferencesDelta son los cambios de preferencias derivados de un feedback.
type PreferencesDelta struct {
	Recommendation map[string]any // se mezcla en RecommendationParams
	Accommodation  map[string]any
	Transport      map[string]any
	Route          map[string]any
}

// IsEmpty indica que el feedback no cambia ninguna preferencia.
func (d PreferencesDelta) IsEmpty() bool {
	return len(d.Recommendation)+len(d.Accommodation)+len(d.Transport)+len(d.Route) == 0
}

// FeedbackAnalyzer [ADR-25] Analiza la interacción del viajero (ACEPTAR/RECHAZAR/AJUSTAR) y deriva
// los cambios de preferencias:
//   - AJUSTAR: los parámetros ajustados pasan tal cual a los parámetros de recomendación (ej. presupuestoMax).
//   - ACEPTAR: se favorece el destino y el proveedor del ítem aceptado.
//   - RECHAZAR: no cambia preferencias; el rechazo queda en el historial de feedback y el motor excluye el ítem.
type FeedbackAnalyzer struct{}

// Analyze traduce un feedback sobre un ítem del catálogo en cambios de preferencias.
func (*FeedbackAnalyzer) Analyze(_ context.Context, item domain.CatalogItem, feedback domain.RecommendationFeedback) (PreferencesDelta, error) {
	var delta PreferencesDelta
	switch feedback.Action {
	case domain.FeedbackAdjust:
		if len(feedback.AdjustedParams) == 0 {
			return delta, fmt.Errorf("%w: AJUSTAR requiere parametrosAjustados", ErrInvalidFeedback)
		}
		delta.Recommendation = feedback.AdjustedParams
	case domain.FeedbackAccept:
		if dest, _ := item.Metadata["destino"].(string); dest != "" {
			delta.Recommendation = map[string]any{paramPreferredDest: dest}
		}
		section := map[string]any{prefPreferredProvider: item.Provider}
		switch item.Category {
		case domain.CategoryAccommodation:
			delta.Accommodation = section
		case domain.CategoryTransport:
			delta.Transport = section
		default:
			delta.Route = section
		}
	case domain.FeedbackReject:
		// sin cambios de preferencias
	default:
		return delta, fmt.Errorf("%w: acción %q desconocida", ErrInvalidFeedback, feedback.Action)
	}
	return delta, nil
}

// PreferencesStore es el repositorio de preferencias del perfil (lo implementa el de Autenticación).
type PreferencesStore interface {
	Get(ctx context.Context, travelerID string) (auth.TravelerPreferences, error)
	Save(ctx context.Context, preferences auth.TravelerPreferences) error
}

// PreferencesUpdater [ADR-25][QA-18] Escribe las preferencias actualizadas vía cola de prioridades,
// para que la siguiente recomendación las use.
type PreferencesUpdater struct{ store PreferencesStore }

// Update mezcla el delta en las preferencias del viajero y las persiste (lectura + escritura en cola).
func (u *PreferencesUpdater) Update(ctx context.Context, travelerID string, delta PreferencesDelta) error {
	if delta.IsEmpty() {
		return nil
	}
	prefs, err := u.store.Get(ctx, travelerID)
	if err != nil {
		return err
	}
	prefs.TravelerID = travelerID
	prefs.RecommendationParams = merge(prefs.RecommendationParams, delta.Recommendation)
	prefs.AccommodationPreferences = merge(prefs.AccommodationPreferences, delta.Accommodation)
	prefs.TransportPreferences = merge(prefs.TransportPreferences, delta.Transport)
	prefs.RoutePreferences = merge(prefs.RoutePreferences, delta.Route)
	return u.store.Save(ctx, prefs)
}

func merge(base, delta map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(delta))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range delta {
		out[k] = v
	}
	return out
}

// HITLFeedback agrupa Analyzer y Updater y registra el feedback en el historial (human-in-the-loop). [ADR-25]
type HITLFeedback struct {
	Analyzer *FeedbackAnalyzer
	Updater  *PreferencesUpdater

	data    ports.DataGateway
	catalog ports.ProviderCatalogPort
	now     func() time.Time
}

// NewHITLFeedback crea la retroalimentación HITL.
func NewHITLFeedback(data ports.DataGateway, store PreferencesStore, catalog ports.ProviderCatalogPort) *HITLFeedback {
	return &HITLFeedback{
		Analyzer: &FeedbackAnalyzer{}, Updater: &PreferencesUpdater{store: store},
		data: data, catalog: catalog, now: time.Now,
	}
}

// Register analiza el feedback, lo guarda en el historial y actualiza las preferencias del viajero.
// Ambas escrituras salen por la cola de prioridades con baja prioridad. [QA-18]
func (h *HITLFeedback) Register(ctx context.Context, travelerID, recommendationID string, feedback domain.RecommendationFeedback) error {
	items, err := h.catalog.List(ctx)
	if err != nil {
		return err
	}
	var item domain.CatalogItem
	found := false
	for _, it := range items {
		if it.ItemID == recommendationID {
			item, found = it, true
			break
		}
	}
	if !found {
		return ErrUnknownRecommendation
	}
	feedback.RecommendationID = recommendationID
	delta, err := h.Analyzer.Analyze(ctx, item, feedback)
	if err != nil {
		return err
	}
	params, err := json.Marshal(feedback.AdjustedParams)
	if err != nil || feedback.AdjustedParams == nil {
		params = []byte("{}")
	}
	id, err := newID()
	if err != nil {
		return err
	}
	if _, err := h.data.Write(ctx, travelerID, operations.RecommendationFeedbackSave, map[string]any{
		"idFeedback": id, "idRecomendacion": recommendationID, "idViajero": travelerID,
		"accion": string(feedback.Action), "parametrosAjustados": string(params),
		"fecha": h.now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return err
	}
	return h.Updater.Update(ctx, travelerID, delta)
}

func newID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("fb-%x", b[:]), nil
}
