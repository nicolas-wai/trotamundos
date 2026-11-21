package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/httpdto"
	"github.com/nicolas-wai/trotamundos/src/httpresponse"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Vinculación de correo [ADR-15].

// writeEmailLinkError traduce los errores de la vinculación OAuth al contrato de AD §6.1.
func writeEmailLinkError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ports.ErrInvalidInput):
		httpresponse.WriteError(w, http.StatusUnprocessableEntity, apierror.ValidacionFallida, err.Error())
	case errors.Is(err, ports.ErrInvalidAuthorizationCode):
		httpresponse.WriteError(w, http.StatusBadRequest, apierror.CodigoOAuthInvalido, "El código de autorización es inválido o ya fue usado")
	case errors.Is(err, ports.ErrLinkedAccountNotFound):
		httpresponse.WriteError(w, http.StatusNotFound, apierror.NoEncontrado, "La cuenta de correo no existe")
	case errors.Is(err, ports.ErrDataNodeUnavailable), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		httpresponse.WriteError(w, http.StatusServiceUnavailable, apierror.ModuloNoDisponible, "Módulo de datos no disponible")
	default:
		httpresponse.WriteError(w, http.StatusInternalServerError, apierror.ErrorInterno, "Error interno")
	}
}

// LinkEmail atiende POST /correos: canjea el código OAuth y guarda los tokens cifrados (201).
func (c *Controllers) LinkEmail(w http.ResponseWriter, r *http.Request) {
	var req httpdto.LinkEmailRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	account, err := c.deps.EmailLinking.Link(r.Context(), travelerIDFrom(r.Context()), req.Proveedor, req.AuthorizationCode, req.RedirectURI)
	if err != nil {
		writeEmailLinkError(w, err)
		return
	}
	httpresponse.WriteJSON(w, http.StatusCreated, httpdto.LinkEmailResponse{
		IDCuenta: account.AccountID, Proveedor: account.Provider, Estado: string(account.Status),
	})
}

// UnlinkEmail atiende DELETE /correos/{idCuenta}: revoca los tokens y elimina el vínculo (204).
func (c *Controllers) UnlinkEmail(w http.ResponseWriter, r *http.Request) {
	if err := c.deps.EmailLinking.Unlink(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idCuenta")); err != nil {
		writeEmailLinkError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
