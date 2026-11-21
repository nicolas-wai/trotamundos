// Package apierror materializa las convenciones de errores de AD §6.1 (códigos y cuerpo de error).
// Es un contrato sin tecnología: solo biblioteca estándar. La escritura HTTP vive en httpresponse. [ADR-02]
package apierror

import "errors"

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
