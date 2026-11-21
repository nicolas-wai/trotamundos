// Package tokenvalidator es el componente "Validador de token firmado" de la réplica (AD §4.4).
package tokenvalidator

import (
	"net/http"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Validator [ADR-16] Valida firma y vigencia del access token localmente, sin estado compartido,
// e inyecta idViajero en el contexto. Fase 1: aún no se engancha a las rutas.
type Validator struct {
	tokens ports.TokenServicePort
}

// NewValidator crea el validador sobre el servicio de tokens.
func NewValidator(tokens ports.TokenServicePort) *Validator { return &Validator{tokens: tokens} }

// Authenticate extrae y valida el Bearer token de la request.
func (v *Validator) Authenticate(r *http.Request) (ports.Claims, error) {
	return ports.Claims{}, apierror.ErrNotImplemented
}

// Middleware protege un handler exigiendo token válido (401 TOKEN_EXPIRADO / TOKEN_INVALIDO).
func (v *Validator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apierror.WriteNotImplemented(w)
	})
}
