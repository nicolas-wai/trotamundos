package ports

import (
	"context"
	"errors"
	"time"
)

// Errores del flujo OAuth del correo.
var (
	// ErrInvalidAuthorizationCode: el proveedor rechazó el código de autorización (inválido, usado o vencido).
	ErrInvalidAuthorizationCode = errors.New("código de autorización inválido")
	// ErrOAuthAccountRevoked: la cuenta ya fue revocada en el proveedor.
	ErrOAuthAccountRevoked = errors.New("cuenta OAuth revocada")
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
