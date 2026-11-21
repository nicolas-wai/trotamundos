// Package tokenvalidator es el componente "Validador de token firmado" de la réplica (AD §4.4).
package tokenvalidator

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/httpresponse"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

type ctxKey struct{}

// SignedTokenValidator [ADR-16] Valida firma y vigencia del access token localmente, sin estado compartido,
// e inyecta idViajero en el contexto (clave de partición [ADR-11]: siempre sale del token, nunca del body).
type SignedTokenValidator struct {
	tokens ports.TokenServicePort
}

// NewSignedTokenValidator crea el validador sobre el servicio de tokens.
func NewSignedTokenValidator(tokens ports.TokenServicePort) *SignedTokenValidator {
	return &SignedTokenValidator{tokens: tokens}
}

// Authenticate extrae y valida el Bearer token de la request.
func (v *SignedTokenValidator) Authenticate(r *http.Request) (ports.Claims, error) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return ports.Claims{}, ports.ErrTokenInvalid
	}
	return v.tokens.Validate(strings.TrimSpace(token))
}

// Middleware protege un handler exigiendo token válido (401 TOKEN_EXPIRADO / TOKEN_INVALIDO).
func (v *SignedTokenValidator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := v.Authenticate(r)
		switch {
		case errors.Is(err, ports.ErrTokenExpired):
			httpresponse.WriteError(w, http.StatusUnauthorized, apierror.TokenExpirado, "El access token expiró")
			return
		case err != nil:
			httpresponse.WriteError(w, http.StatusUnauthorized, apierror.TokenInvalido, "Token inválido o ausente")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims.TravelerID)))
	})
}

// TravelerIDFrom devuelve el idViajero que el middleware inyectó en el contexto ("" si no hay).
func TravelerIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
