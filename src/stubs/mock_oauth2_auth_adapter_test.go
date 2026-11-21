package stubs

import (
	"context"
	"errors"
	"testing"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

func TestMockOAuthFlowCodeExchangeRefreshRevoke(t *testing.T) {
	m := NewMockOAuth2AuthAdapter()
	ctx := context.Background()
	tok, err := m.ExchangeCode(ctx, "gmail", "mock-code-ana", "http://localhost/cb")
	if err != nil || tok.AccessToken != "mock-access-gmail-ana" || tok.RefreshToken != "mock-refresh-gmail-ana" || tok.ExpiresAt.IsZero() {
		t.Fatalf("%+v %v", tok, err)
	}
	for _, code := range []string{"", "otro", "mock-code-", "mock-code-ana"} { // inválidos y ya usado
		if _, err := m.ExchangeCode(ctx, "gmail", code, "http://localhost/cb"); !errors.Is(err, ports.ErrInvalidAuthorizationCode) {
			t.Fatalf("código %q: %v", code, err)
		}
	}
	if _, err := m.ExchangeCode(ctx, "gmail", "mock-code-b", ""); !errors.Is(err, ports.ErrInvalidAuthorizationCode) {
		t.Fatalf("sin redirectUri: %v", err)
	}
	if r, err := m.Refresh(ctx, "cta-1"); err != nil || r.AccessToken == "" || r.AccessToken == tok.AccessToken {
		t.Fatalf("refresh: %+v %v", r, err)
	}
	if err := m.Revoke(ctx, "cta-1"); err != nil {
		t.Fatal(err)
	}
	if got := m.RevokedAccounts(); len(got) != 1 || got[0] != "cta-1" {
		t.Fatalf("revocaciones registradas: %v", got)
	}
	if _, err := m.Refresh(ctx, "cta-1"); !errors.Is(err, ports.ErrOAuthAccountRevoked) {
		t.Fatalf("una cuenta revocada no se renueva: %v", err)
	}
}
