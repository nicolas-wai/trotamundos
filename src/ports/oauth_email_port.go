package ports

import (
	"context"
	"time"
)

// OAuthTokens son los tokens OAuth 2.0 obtenidos del proveedor de correo.
type OAuthTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// OAuthEmailPort [ADR-15] Autorización delegada OAuth 2.0 del correo.
// Nunca se maneja la contraseña del correo; los permisos son revocables.
type OAuthEmailPort interface {
	ExchangeCode(ctx context.Context, provider, authorizationCode, redirectURI string) (OAuthTokens, error)
	Refresh(ctx context.Context, accountID string) (OAuthTokens, error)
	Revoke(ctx context.Context, accountID string) error
}
