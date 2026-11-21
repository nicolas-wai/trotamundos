package ports

import (
	"context"
	"errors"
	"time"
)

// Errores del servicio de tokens.
var (
	ErrTokenInvalid            = errors.New("token inválido")
	ErrTokenExpired            = errors.New("token expirado")
	ErrRefreshRevokedOrExpired = errors.New("refresh token revocado o expirado")
)

// TokenPair es el par emitido en el login.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // segundos
}

// AccessToken es un access token renovado.
type AccessToken struct {
	Token     string
	ExpiresIn int // segundos
}

// Claims son los datos del access token validado; TravelerID es la clave de partición [ADR-11].
type Claims struct {
	TravelerID string
	ExpiresAt  time.Time
}

// TokenServicePort [ADR-16] Tokens de vida corta. Validate es local, sin estado compartido,
// de modo que cualquier réplica valida el token. La revocación inmediata del access no es posible.
type TokenServicePort interface {
	Issue(ctx context.Context, travelerID string) (TokenPair, error)
	// Validate devuelve ErrTokenInvalid o ErrTokenExpired.
	Validate(accessToken string) (Claims, error)
	// Refresh devuelve ErrRefreshRevokedOrExpired si el refresh no es válido.
	Refresh(ctx context.Context, refreshToken string) (AccessToken, error)
	RevokeRefresh(ctx context.Context, refreshToken string) error
}
