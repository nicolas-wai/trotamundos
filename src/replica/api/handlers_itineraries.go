package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/httpdto"
	"github.com/nicolas-wai/trotamundos/src/httpresponse"
	"github.com/nicolas-wai/trotamundos/src/ports"
	permapp "github.com/nicolas-wai/trotamundos/src/replica/permissions/application"
	permdomain "github.com/nicolas-wai/trotamundos/src/replica/permissions/domain"
)

// Itinerarios compartidos [ADR-13]. idViajero sale siempre del token [ADR-11].

// writeItineraryError traduce los errores de Permisos al contrato de AD §6.1/§6.2.
func writeItineraryError(w http.ResponseWriter, err error) {
	var denied *permapp.AccessDeniedError
	switch {
	case errors.As(err, &denied):
		// 403 con la opción de solicitar acceso (AD §6.2). El rechazo del dueño bloquea: no se ofrece solicitar.
		httpresponse.WriteJSON(w, http.StatusForbidden, httpdto.AccessDeniedResponse{
			Codigo: apierror.AccesoNoAutorizado, Mensaje: "No tienes acceso a este itinerario", PuedeSolicitar: denied.CanRequest,
		})
	case errors.Is(err, permapp.ErrValidation):
		httpresponse.WriteError(w, http.StatusUnprocessableEntity, apierror.ValidacionFallida, err.Error())
	case errors.Is(err, permapp.ErrForbidden), errors.Is(err, permdomain.ErrAccessBlocked):
		httpresponse.WriteError(w, http.StatusForbidden, apierror.AccesoNoAutorizado, err.Error())
	case errors.Is(err, permapp.ErrItineraryNotFound):
		httpresponse.WriteError(w, http.StatusNotFound, apierror.NoEncontrado, "El itinerario no existe")
	case errors.Is(err, permapp.ErrArrangementNotFound):
		httpresponse.WriteError(w, http.StatusNotFound, apierror.NoEncontrado, "El arreglo de viaje no existe")
	case errors.Is(err, permdomain.ErrRequestNotFound), errors.Is(err, permdomain.ErrPermissionNotFound):
		httpresponse.WriteError(w, http.StatusNotFound, apierror.NoEncontrado, err.Error())
	case errors.Is(err, permdomain.ErrRequestNotPending), errors.Is(err, permdomain.ErrAlreadyAuthorized),
		errors.Is(err, permdomain.ErrOwnerAlreadyHasAccess):
		httpresponse.WriteError(w, http.StatusConflict, apierror.Conflicto, err.Error())
	case errors.Is(err, ports.ErrDataNodeUnavailable), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		httpresponse.WriteError(w, http.StatusServiceUnavailable, apierror.ModuloNoDisponible, "Módulo de datos no disponible")
	default:
		httpresponse.WriteError(w, http.StatusInternalServerError, apierror.ErrorInterno, "Error interno")
	}
}

// GetItinerary atiende GET /itinerarios/{idItinerario}: 200 al dueño y a los autorizados, 403 al resto. [QA-09]
func (c *Controllers) GetItinerary(w http.ResponseWriter, r *http.Request) {
	it, err := c.deps.Permissions.GetItinerary(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idItinerario"))
	if err != nil {
		writeItineraryError(w, err)
		return
	}
	out := httpdto.SharedItineraryResponse{IDItinerario: it.ItineraryID, Nombre: it.Name, Arreglos: []httpdto.ArrangementSummary{}}
	for _, a := range it.Arrangements {
		out.Arreglos = append(out.Arreglos, httpdto.ArrangementSummary{
			IDArreglo: a.ArrangementID, Tipo: a.Type, Titulo: a.Title, FechaInicio: a.StartAt, Estado: a.Status, Origen: a.Origin,
		})
	}
	httpresponse.WriteJSON(w, http.StatusOK, out)
}

// CreateItinerary atiende POST /itinerarios.
func (c *Controllers) CreateItinerary(w http.ResponseWriter, r *http.Request) {
	var req httpdto.CreateItineraryRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	it, err := c.deps.Permissions.CreateItinerary(r.Context(), travelerIDFrom(r.Context()), req.Nombre, req.IDsArreglos)
	if err != nil {
		writeItineraryError(w, err)
		return
	}
	httpresponse.WriteJSON(w, http.StatusCreated, httpdto.CreateItineraryResponse{IDItinerario: it.ItineraryID, LinkPublico: it.PublicLink})
}

// DeleteItinerary atiende DELETE /itinerarios/{id}.
func (c *Controllers) DeleteItinerary(w http.ResponseWriter, r *http.Request) {
	respondNoContent(w, c.deps.Permissions.DeleteItinerary(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id")))
}

// AddItineraryArrangement atiende POST /itinerarios/{id}/arreglos.
func (c *Controllers) AddItineraryArrangement(w http.ResponseWriter, r *http.Request) {
	var req httpdto.AddArrangementRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	respondNoContent(w, c.deps.Permissions.AddArrangement(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id"), req.IDArreglo))
}

// RemoveItineraryArrangement atiende DELETE /itinerarios/{id}/arreglos/{idArreglo}.
func (c *Controllers) RemoveItineraryArrangement(w http.ResponseWriter, r *http.Request) {
	respondNoContent(w, c.deps.Permissions.RemoveArrangement(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id"), r.PathValue("idArreglo")))
}

// RequestItineraryAccess atiende POST /itinerarios/{id}/solicitudes (queda PENDIENTE hasta que decida el dueño).
func (c *Controllers) RequestItineraryAccess(w http.ResponseWriter, r *http.Request) {
	req, err := c.deps.Permissions.RequestAccess(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id"))
	if err != nil {
		writeItineraryError(w, err)
		return
	}
	httpresponse.WriteJSON(w, http.StatusCreated, httpdto.CreateAccessRequestResponse{IDSolicitud: req.RequestID, Estado: string(req.Status)})
}

// ListItineraryRequests atiende GET /itinerarios/{id}/solicitudes (solo el dueño).
func (c *Controllers) ListItineraryRequests(w http.ResponseWriter, r *http.Request) {
	reqs, err := c.deps.Permissions.ListRequests(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id"))
	if err != nil {
		writeItineraryError(w, err)
		return
	}
	out := httpdto.AccessRequestsResponse{Solicitudes: []httpdto.AccessRequestItem{}}
	for _, q := range reqs {
		out.Solicitudes = append(out.Solicitudes, httpdto.AccessRequestItem{
			IDSolicitud: q.RequestID, IDViajero: q.RequesterTravelerID, Estado: string(q.Status),
		})
	}
	httpresponse.WriteJSON(w, http.StatusOK, out)
}

// DecideItineraryRequest atiende POST /itinerarios/{id}/solicitudes/{idSolicitud}/decision (solo el dueño).
func (c *Controllers) DecideItineraryRequest(w http.ResponseWriter, r *http.Request) {
	var req httpdto.DecisionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	status, err := c.deps.Permissions.DecideRequest(r.Context(), travelerIDFrom(r.Context()),
		r.PathValue("id"), r.PathValue("idSolicitud"), ports.AccessDecision(req.Decision))
	if err != nil {
		writeItineraryError(w, err)
		return
	}
	httpresponse.WriteJSON(w, http.StatusOK, httpdto.DecisionResponse{Estado: string(status)})
}

// RevokeItineraryAccess atiende DELETE /itinerarios/{id}/accesos/{idViajero}.
func (c *Controllers) RevokeItineraryAccess(w http.ResponseWriter, r *http.Request) {
	respondNoContent(w, c.deps.Permissions.Revoke(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id"), r.PathValue("idViajero")))
}

func respondNoContent(w http.ResponseWriter, err error) {
	if err != nil {
		writeItineraryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
