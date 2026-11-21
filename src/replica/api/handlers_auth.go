package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/httpdto"
	"github.com/nicolas-wai/trotamundos/src/ports"
	authapp "github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
)

// Autenticación [ADR-14][ADR-16].

const maxBodyBytes = 1 << 20

// decodeJSON lee el body JSON; responde 400 y devuelve false si es malformado.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(dst); err != nil {
		apierror.WriteError(w, http.StatusBadRequest, apierror.ValidacionFallida, "El cuerpo de la solicitud no es un JSON válido")
		return false
	}
	return true
}

// writeAuthError traduce los errores del módulo de autenticación al contrato de AD §6.1/§6.2.
func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authapp.ErrValidation):
		apierror.WriteError(w, http.StatusUnprocessableEntity, apierror.ValidacionFallida, err.Error())
	case errors.Is(err, authapp.ErrEmailTaken):
		apierror.WriteError(w, http.StatusConflict, apierror.Conflicto, "El email ya está registrado")
	case errors.Is(err, authapp.ErrInvalidCredentials):
		apierror.WriteError(w, http.StatusUnauthorized, apierror.CredencialesInvalidas, "Email o contraseña incorrectos")
	case errors.Is(err, ports.ErrRefreshRevokedOrExpired):
		apierror.WriteError(w, http.StatusUnauthorized, apierror.RefreshRevocadoOExpirado, "El refresh token fue revocado o expiró")
	case errors.Is(err, ports.ErrDataNodeUnavailable), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		apierror.WriteError(w, http.StatusServiceUnavailable, apierror.ModuloNoDisponible, "Módulo de datos no disponible")
	default:
		apierror.WriteError(w, http.StatusInternalServerError, apierror.ErrorInterno, "Error interno")
	}
}

// Login atiende POST /login.
func (c *Controllers) Login(w http.ResponseWriter, r *http.Request) {
	var req httpdto.LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	pair, err := c.deps.Authentication.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	apierror.WriteJSON(w, http.StatusOK, httpdto.LoginResponse{
		AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresIn: pair.ExpiresIn, TokenType: "Bearer",
	})
}

// RefreshToken atiende POST /token/refresh.
func (c *Controllers) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req httpdto.RefreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	at, err := c.deps.Authentication.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	apierror.WriteJSON(w, http.StatusOK, httpdto.RefreshResponse{AccessToken: at.Token, ExpiresIn: at.ExpiresIn, TokenType: "Bearer"})
}

// Logout atiende POST /logout.
func (c *Controllers) Logout(w http.ResponseWriter, r *http.Request) {
	var req httpdto.LogoutRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := c.deps.Authentication.Logout(r.Context(), req.RefreshToken); err != nil {
		writeAuthError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Register atiende POST /registro [QA-05].
func (c *Controllers) Register(w http.ResponseWriter, r *http.Request) {
	var req httpdto.RegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := c.deps.Authentication.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	apierror.WriteJSON(w, http.StatusCreated, httpdto.RegisterResponse{IDViajero: id})
}
