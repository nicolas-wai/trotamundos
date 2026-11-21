package stubs

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// MockOAuth2AuthAdapter [ADR-15] Simula OAuth 2.0 con códigos y tokens determinísticos:
// código → intercambio → tokens → revocación. [Prototipo] sin proveedor de correo real.
type MockOAuth2AuthAdapter struct{}

var _ ports.OAuthEmailPort = (*MockOAuth2AuthAdapter)(nil)

// NewMockOAuth2AuthAdapter crea el adapter simulado.
func NewMockOAuth2AuthAdapter() *MockOAuth2AuthAdapter { return &MockOAuth2AuthAdapter{} }

// ExchangeCode intercambia el código de autorización por tokens.
func (*MockOAuth2AuthAdapter) ExchangeCode(context.Context, string, string, string) (ports.OAuthTokens, error) {
	return ports.OAuthTokens{}, apierror.ErrNotImplemented
}

// Refresh renueva los tokens de una cuenta.
func (*MockOAuth2AuthAdapter) Refresh(context.Context, string) (ports.OAuthTokens, error) {
	return ports.OAuthTokens{}, apierror.ErrNotImplemented
}

// Revoke revoca los tokens de una cuenta.
func (*MockOAuth2AuthAdapter) Revoke(context.Context, string) error {
	return apierror.ErrNotImplemented
}
