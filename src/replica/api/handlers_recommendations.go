package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/httpdto"
	"github.com/nicolas-wai/trotamundos/src/httpresponse"
	"github.com/nicolas-wai/trotamundos/src/ports"
	authapp "github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations"
	recdomain "github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
)

// DefaultRecommendationTimeout es el límite de generación de recomendaciones. [QA-17]
const DefaultRecommendationTimeout = 3 * time.Second

// Recomendaciones [ADR-23][ADR-25].

// Recommendations atiende GET /recomendaciones: request independiente del dashboard con límite de 3 s (QA-17).
// Si falla o demora responde 503 y el cliente muestra el estado de error del panel sin afectar a las reservas. [ADR-23]
func (c *Controllers) Recommendations(w http.ResponseWriter, r *http.Request) {
	timeout := c.deps.RecommendationTimeout
	if timeout <= 0 {
		timeout = DefaultRecommendationTimeout
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	recs, err := c.deps.Recommender.Generate(ctx, travelerIDFrom(r.Context()))
	if err != nil {
		httpresponse.WriteError(w, http.StatusServiceUnavailable, apierror.ModuloNoDisponible, "Las recomendaciones no están disponibles por ahora")
		return
	}
	out := httpdto.RecommendationsResponse{Recomendaciones: []httpdto.RecommendationItem{}}
	for _, rec := range recs {
		out.Recomendaciones = append(out.Recomendaciones, httpdto.RecommendationItem{
			IDRecomendacion: rec.RecommendationID, Categoria: string(rec.Category), Titulo: rec.Title,
			Metadatos: rec.ItemMetadata, Proveedor: rec.Provider,
		})
	}
	httpresponse.WriteJSON(w, http.StatusOK, out)
}

// RecommendationFeedback atiende POST /recomendaciones/{idRecomendacion}/feedback (HITL, QA-18).
func (c *Controllers) RecommendationFeedback(w http.ResponseWriter, r *http.Request) {
	var req httpdto.FeedbackRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	action := recdomain.FeedbackAction(req.Accion)
	switch action {
	case recdomain.FeedbackAccept, recdomain.FeedbackReject, recdomain.FeedbackAdjust:
	default:
		httpresponse.WriteError(w, http.StatusUnprocessableEntity, apierror.ValidacionFallida, "accion debe ser ACEPTAR, RECHAZAR o AJUSTAR")
		return
	}
	err := c.deps.Recommender.RegisterFeedback(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idRecomendacion"), action, req.ParametrosAjustados)
	switch {
	case err == nil:
		httpresponse.WriteJSON(w, http.StatusOK, httpdto.FeedbackResponse{Registrado: true})
	case errors.Is(err, recommendations.ErrInvalidFeedback):
		httpresponse.WriteError(w, http.StatusUnprocessableEntity, apierror.ValidacionFallida, err.Error())
	case errors.Is(err, recommendations.ErrUnknownRecommendation):
		httpresponse.WriteError(w, http.StatusNotFound, apierror.NoEncontrado, "La recomendación no existe")
	default:
		writeUnavailableOrInternal(w, err)
	}
}

// UpdatePreferences atiende PUT /perfil/preferencias.
func (c *Controllers) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	var req httpdto.UpdatePreferencesRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	err := c.deps.Profile.UpdatePreferences(r.Context(), travelerIDFrom(r.Context()), req.Parametros)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, authapp.ErrValidation):
		httpresponse.WriteError(w, http.StatusUnprocessableEntity, apierror.ValidacionFallida, err.Error())
	default:
		writeUnavailableOrInternal(w, err)
	}
}

func writeUnavailableOrInternal(w http.ResponseWriter, err error) {
	if errors.Is(err, ports.ErrDataNodeUnavailable) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		httpresponse.WriteError(w, http.StatusServiceUnavailable, apierror.ModuloNoDisponible, "Módulo de datos no disponible")
		return
	}
	httpresponse.WriteError(w, http.StatusInternalServerError, apierror.ErrorInterno, "Error interno")
}
