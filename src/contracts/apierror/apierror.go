// Package apierror materializa las convenciones de errores de AD §6.1.
package apierror

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ErrorResponse es el cuerpo de error del contrato: { "codigo", "mensaje" }.
type ErrorResponse struct {
	Codigo  string `json:"codigo"`
	Mensaje string `json:"mensaje"`
}

// Catálogo de códigos de error (AD §6.2).
const (
	CredencialesInvalidas    = "CREDENCIALES_INVALIDAS"
	RefreshRevocadoOExpirado = "REFRESH_REVOCADO_O_EXPIRADO"
	AccesoNoAutorizado       = "ACCESO_NO_AUTORIZADO"
	TokenExpirado            = "TOKEN_EXPIRADO"
	TokenInvalido            = "TOKEN_INVALIDO"
	NoImplementado           = "NO_IMPLEMENTADO"
	ValidacionFallida        = "VALIDACION_FALLIDA"
	NoEncontrado             = "NO_ENCONTRADO"
	Conflicto                = "CONFLICTO"
	ModuloNoDisponible       = "MODULO_NO_DISPONIBLE"
	CodigoOAuthInvalido      = "CODIGO_OAUTH_INVALIDO"
	ErrorInterno             = "ERROR_INTERNO"
)

// ErrNotImplemented lo devuelven los componentes del esqueleto de la Fase 1.
var ErrNotImplemented = errors.New("no implementado")

// WriteError escribe el cuerpo de error con el código HTTP indicado.
func WriteError(w http.ResponseWriter, status int, codigo, mensaje string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Codigo: codigo, Mensaje: mensaje})
}

// WriteNotImplemented responde 501 NO_IMPLEMENTADO.
func WriteNotImplemented(w http.ResponseWriter) {
	WriteError(w, http.StatusNotImplemented, NoImplementado, "Funcionalidad no implementada en esta fase del prototipo")
}

// WriteJSON escribe una respuesta JSON con el código HTTP indicado.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
