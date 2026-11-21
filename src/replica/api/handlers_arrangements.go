package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/httpdto"
	"github.com/nicolas-wai/trotamundos/src/httpresponse"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// Dashboard y arreglos [ADR-17][ADR-18][ADR-20].

// writeArrangementError traduce los errores de Arreglos de viaje al contrato de AD §6.1.
func writeArrangementError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrValidation):
		httpresponse.WriteError(w, http.StatusUnprocessableEntity, apierror.ValidacionFallida, err.Error())
	case errors.Is(err, application.ErrNotFound):
		httpresponse.WriteError(w, http.StatusNotFound, apierror.NoEncontrado, "El arreglo de viaje no existe")
	case errors.Is(err, ports.ErrDataNodeUnavailable), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		httpresponse.WriteError(w, http.StatusServiceUnavailable, apierror.ModuloNoDisponible, "Módulo de datos no disponible")
	default:
		httpresponse.WriteError(w, http.StatusInternalServerError, apierror.ErrorInterno, "Error interno")
	}
}

// wantsJSON: el dashboard responde JSON solo si el cliente lo pide (móvil ◇); la web recibe HTML SSR. [ADR-20]
func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

// wantsHTML: el detalle responde JSON salvo que el cliente pida el fragmento HTML (recarga parcial).
func wantsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func writeHTML(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// Dashboard atiende GET /dashboard (HTML en web, JSON con Accept: application/json).
func (c *Controllers) Dashboard(w http.ResponseWriter, r *http.Request) {
	dashboard, err := c.deps.Arrangements.GetDashboard(r.Context(), travelerIDFrom(r.Context()))
	if err != nil {
		writeArrangementError(w, err)
		return
	}
	if wantsJSON(r) {
		httpresponse.WriteJSON(w, http.StatusOK, dashboardToDTO(dashboard))
		return
	}
	body, err := c.deps.Renderer.RenderDashboard(r.Context(), dashboard)
	if err != nil {
		writeArrangementError(w, err)
		return
	}
	writeHTML(w, body)
}

// GetArrangement atiende GET /arreglos/{idArreglo} (JSON o fragmento HTML).
func (c *Controllers) GetArrangement(w http.ResponseWriter, r *http.Request) {
	a, err := c.deps.Arrangements.GetArrangementDetail(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idArreglo"))
	if err != nil {
		writeArrangementError(w, err)
		return
	}
	if wantsHTML(r) {
		body, err := c.deps.Renderer.RenderArrangementDetail(r.Context(), a)
		if err != nil {
			writeArrangementError(w, err)
			return
		}
		writeHTML(w, body)
		return
	}
	httpresponse.WriteJSON(w, http.StatusOK, detailToDTO(a))
}

// CreateArrangement atiende POST /arreglos (escritura por la cola de prioridades).
func (c *Controllers) CreateArrangement(w http.ResponseWriter, r *http.Request) {
	var req httpdto.ArrangementRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := c.deps.Arrangements.CreateArrangement(r.Context(), travelerIDFrom(r.Context()), inputFromDTO(req))
	if err != nil {
		writeArrangementError(w, err)
		return
	}
	httpresponse.WriteJSON(w, http.StatusCreated, httpdto.CreateArrangementResponse{IDArreglo: id})
}

// UpdateArrangement atiende PUT /arreglos/{idArreglo}.
func (c *Controllers) UpdateArrangement(w http.ResponseWriter, r *http.Request) {
	var req httpdto.ArrangementRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	err := c.deps.Arrangements.UpdateArrangement(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idArreglo"), inputFromDTO(req))
	if err != nil {
		writeArrangementError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// DeleteArrangement atiende DELETE /arreglos/{idArreglo}.
func (c *Controllers) DeleteArrangement(w http.ResponseWriter, r *http.Request) {
	if err := c.deps.Arrangements.DeleteArrangement(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idArreglo")); err != nil {
		writeArrangementError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func inputFromDTO(req httpdto.ArrangementRequest) application.ArrangementInput {
	return application.ArrangementInput{
		TripID: req.IDViaje, Type: domain.ArrangementType(req.Tipo), Title: req.Titulo,
		StartAt: req.FechaInicio, EndAt: req.FechaFin, Detail: req.Detalle,
	}
}

// dashboardToDTO arma la representación JSON de §6.2 (móvil ◇), siempre con listas (nunca null).
func dashboardToDTO(d application.Dashboard) httpdto.DashboardResponse {
	out := httpdto.DashboardResponse{Viajes: []httpdto.TripSummary{}, AvisosProveedores: []httpdto.ProviderNotice{}}
	for _, t := range d.Trips {
		ts := httpdto.TripSummary{
			IDViaje: t.TripID, Nombre: t.Name, Destino: t.Destination,
			FechaInicio: t.StartDate.UTC().Format(time.DateOnly), FechaFin: t.EndDate.UTC().Format(time.DateOnly),
			Arreglos: []httpdto.ArrangementSummary{},
		}
		for _, a := range t.Arrangements {
			ts.Arreglos = append(ts.Arreglos, httpdto.ArrangementSummary{
				IDArreglo: a.ArrangementID, Tipo: string(a.Type), Titulo: a.Title, FechaInicio: a.StartAt,
				Estado: string(a.Status), Origen: string(a.Origin),
			})
		}
		out.Viajes = append(out.Viajes, ts)
	}
	for _, n := range d.ProviderNotices {
		out.AvisosProveedores = append(out.AvisosProveedores, httpdto.ProviderNotice{Proveedor: n.Provider, Mensaje: n.Message})
	}
	return out
}

func detailToDTO(a domain.TravelArrangement) httpdto.ArrangementDetailResponse {
	detalle := map[string]any{}
	if a.Detail != nil {
		if raw, err := json.Marshal(a.Detail); err == nil {
			_ = json.Unmarshal(raw, &detalle)
		}
	}
	var sender *string
	if a.Sender != "" {
		s := string(a.Sender)
		sender = &s
	}
	return httpdto.ArrangementDetailResponse{
		IDArreglo: a.ArrangementID, IDViaje: a.TripID, Tipo: string(a.Type), Estado: string(a.Status),
		Origen: string(a.Origin), Remitente: sender, Detalle: detalle, ActualizadoEn: a.UpdatedAt,
	}
}
