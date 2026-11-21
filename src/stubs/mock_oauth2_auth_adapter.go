package stubs

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Prefijos determinísticos del flujo OAuth simulado.
const (
	// AuthCodePrefix: los códigos de autorización válidos empiezan así (p. ej. "mock-code-ana").
	AuthCodePrefix = "mock-code-"
	// AccessTokenPrefix: los access tokens emitidos empiezan así; el stub de correo exige este prefijo.
	AccessTokenPrefix = "mock-access-"
	refreshPrefix     = "mock-refresh-"
)

// MockOAuth2AuthAdapter [ADR-15] Simula OAuth 2.0 con códigos y tokens determinísticos:
// código → intercambio → tokens → revocación. [Prototipo] sin proveedor de correo real.
// Cada código se puede canjear una sola vez, como en OAuth real.
type MockOAuth2AuthAdapter struct {
	mu        sync.Mutex
	usedCodes map[string]bool
	refreshes int
	revoked   []string
	now       func() time.Time
}

var _ ports.OAuthEmailPort = (*MockOAuth2AuthAdapter)(nil)

// NewMockOAuth2AuthAdapter crea el adapter simulado.
func NewMockOAuth2AuthAdapter() *MockOAuth2AuthAdapter {
	return &MockOAuth2AuthAdapter{usedCodes: map[string]bool{}, now: time.Now}
}

// ExchangeCode intercambia el código de autorización por tokens. Un código que no empieza con AuthCodePrefix,
// vacío o ya usado → ErrInvalidAuthorizationCode.
func (m *MockOAuth2AuthAdapter) ExchangeCode(_ context.Context, provider, code, redirectURI string) (ports.OAuthTokens, error) {
	suffix, ok := strings.CutPrefix(code, AuthCodePrefix)
	if !ok || suffix == "" || strings.TrimSpace(provider) == "" || strings.TrimSpace(redirectURI) == "" {
		return ports.OAuthTokens{}, ports.ErrInvalidAuthorizationCode
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.usedCodes[code] {
		return ports.OAuthTokens{}, fmt.Errorf("%w: el código ya fue canjeado", ports.ErrInvalidAuthorizationCode)
	}
	m.usedCodes[code] = true
	return ports.OAuthTokens{
		AccessToken:  AccessTokenPrefix + provider + "-" + suffix,
		RefreshToken: refreshPrefix + provider + "-" + suffix,
		ExpiresAt:    m.now().Add(time.Hour).UTC(),
	}, nil
}

// Refresh renueva los tokens de una cuenta; una cuenta revocada no se puede renovar.
func (m *MockOAuth2AuthAdapter) Refresh(_ context.Context, accountID string) (ports.OAuthTokens, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range m.revoked {
		if id == accountID {
			return ports.OAuthTokens{}, ports.ErrOAuthAccountRevoked
		}
	}
	m.refreshes++
	return ports.OAuthTokens{
		AccessToken:  fmt.Sprintf("%sr%d-%s", AccessTokenPrefix, m.refreshes, accountID),
		RefreshToken: fmt.Sprintf("%sr%d-%s", refreshPrefix, m.refreshes, accountID),
		ExpiresAt:    m.now().Add(time.Hour).UTC(),
	}, nil
}

// Revoke revoca los tokens de una cuenta y deja registro de la llamada (las pruebas lo verifican).
func (m *MockOAuth2AuthAdapter) Revoke(_ context.Context, accountID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revoked = append(m.revoked, accountID)
	return nil
}

// RevokedAccounts devuelve los ids de cuenta revocados, en orden.
func (m *MockOAuth2AuthAdapter) RevokedAccounts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.revoked...)
}
