package oauthmanager

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
	"github.com/nicolas-wai/trotamundos/src/stubs"
)

type memAccounts struct {
	mu sync.Mutex
	m  map[string]domain.LinkedEmailAccount
}

func (a *memAccounts) Save(_ context.Context, acc domain.LinkedEmailAccount) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.m[acc.AccountID] = acc
	return nil
}
func (a *memAccounts) FindByID(_ context.Context, _, id string) (domain.LinkedEmailAccount, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	acc, ok := a.m[id]
	if !ok {
		return domain.LinkedEmailAccount{}, ports.ErrLinkedAccountNotFound
	}
	return acc, nil
}
func (a *memAccounts) Delete(context.Context, string, string) error { return nil }

type plainCipher struct{}

func (plainCipher) Encrypt(s string) ([]byte, error) { return []byte(s), nil }
func (plainCipher) Decrypt(b []byte) (string, error) { return string(b), nil }

// countingOAuth cuenta las llamadas a Refresh y permite hacerlas lentas o fallar.
type countingOAuth struct {
	*stubs.MockOAuth2AuthAdapter
	refreshes atomic.Int32
	err       error
}

func (c *countingOAuth) Refresh(ctx context.Context, id string) (ports.OAuthTokens, error) {
	c.refreshes.Add(1)
	time.Sleep(5 * time.Millisecond) // da ventana a que otras goroutines lleguen a la vez
	if c.err != nil {
		return ports.OAuthTokens{}, c.err
	}
	return c.MockOAuth2AuthAdapter.Refresh(ctx, id)
}

type fixture struct {
	mgr   *OAuthManager
	oauth *countingOAuth
	accts *memAccounts
	now   *atomic.Int64 // reloj inyectado (UnixNano)
	id    string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	var now atomic.Int64
	now.Store(time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).UnixNano())
	clock := func() time.Time { return time.Unix(0, now.Load()).UTC() }
	oauth := &countingOAuth{MockOAuth2AuthAdapter: stubs.NewMockOAuth2AuthAdapter().WithClock(clock)}
	accts := &memAccounts{m: map[string]domain.LinkedEmailAccount{}}
	mgr := NewOAuthManager(oauth, accts, plainCipher{}).WithClock(clock)
	acc, err := mgr.Link(context.Background(), "v1", "gmail", stubs.AuthCodePrefix+"ana", "http://cb")
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{mgr: mgr, oauth: oauth, accts: accts, now: &now, id: acc.AccountID}
}

func (f *fixture) advance(d time.Duration) { f.now.Add(int64(d)) }

// [ADR-15] Con el token vigente no se renueva.
func TestAccessTokenWithinLifetimeDoesNotRefresh(t *testing.T) {
	f := newFixture(t)
	tok, err := f.mgr.AccessToken(context.Background(), "v1", f.id)
	if err != nil || tok != stubs.AccessTokenPrefix+"gmail-ana" || f.oauth.refreshes.Load() != 0 {
		t.Fatalf("token=%q err=%v refreshes=%d", tok, err, f.oauth.refreshes.Load())
	}
}

// [ADR-15] Con el reloj adelantado se renueva una sola vez aunque haya varias goroutines, y el siguiente
// sondeo usa el token nuevo.
func TestExpiredTokenIsRefreshedOnceUnderConcurrency(t *testing.T) {
	f := newFixture(t)
	f.advance(stubs.TokenLifetime + time.Second)

	const n = 20
	tokens := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, err := f.mgr.AccessToken(context.Background(), "v1", f.id)
			if err != nil {
				t.Error(err)
			}
			tokens[i] = tok
		}()
	}
	wg.Wait()
	if got := f.oauth.refreshes.Load(); got != 1 {
		t.Fatalf("debía renovar una sola vez, renovó %d", got)
	}
	for _, tok := range tokens {
		if tok != tokens[0] || tok == stubs.AccessTokenPrefix+"gmail-ana" {
			t.Fatalf("todos debían recibir el token nuevo: %v", tokens)
		}
	}
	// el siguiente sondeo usa el token nuevo y guardado (cifrado) sin volver a renovar
	next, err := f.mgr.AccessToken(context.Background(), "v1", f.id)
	if err != nil || next != tokens[0] || f.oauth.refreshes.Load() != 1 {
		t.Fatalf("siguiente token=%q err=%v refreshes=%d", next, err, f.oauth.refreshes.Load())
	}
	acc, _ := f.accts.FindByID(context.Background(), "v1", f.id)
	if !acc.ExpiresAt.After(time.Unix(0, f.now.Load())) {
		t.Fatalf("el nuevo vencimiento debía quedar guardado: %v", acc.ExpiresAt)
	}
}

// [ADR-15] Falta poco para vencer (margen) también renueva.
func TestTokenNearExpiryIsRefreshed(t *testing.T) {
	f := newFixture(t)
	f.advance(stubs.TokenLifetime - 30*time.Second)
	if _, err := f.mgr.AccessToken(context.Background(), "v1", f.id); err != nil || f.oauth.refreshes.Load() != 1 {
		t.Fatalf("err=%v refreshes=%d", err, f.oauth.refreshes.Load())
	}
}

// [ADR-15][ADR-07] Refresh rechazado → cuenta marcada para re-vincular.
func TestRejectedRefreshMarksAccountForRelink(t *testing.T) {
	f := newFixture(t)
	f.advance(2 * stubs.TokenLifetime)
	f.oauth.err = ports.ErrOAuthAccountRevoked
	if _, err := f.mgr.AccessToken(context.Background(), "v1", f.id); !errors.Is(err, ports.ErrEmailRelinkRequired) {
		t.Fatalf("se esperaba ErrEmailRelinkRequired: %v", err)
	}
	acc, _ := f.accts.FindByID(context.Background(), "v1", f.id)
	if acc.Status != domain.EmailAccountRelinkRequired {
		t.Fatalf("estado: %s", acc.Status)
	}
	// una cuenta marcada no vuelve a llamar al proveedor
	before := f.oauth.refreshes.Load()
	if _, err := f.mgr.AccessToken(context.Background(), "v1", f.id); !errors.Is(err, ports.ErrEmailRelinkRequired) || f.oauth.refreshes.Load() != before {
		t.Fatalf("no debía reintentar: %v", err)
	}
}

// Una falla transitoria no marca la cuenta: es una falla del proveedor y se reintentará.
func TestTransientRefreshFailureKeepsAccountLinked(t *testing.T) {
	f := newFixture(t)
	f.advance(2 * stubs.TokenLifetime)
	f.oauth.err = errors.New("timeout")
	if _, err := f.mgr.AccessToken(context.Background(), "v1", f.id); !errors.Is(err, ports.ErrProviderUnavailable) {
		t.Fatalf("se esperaba ErrProviderUnavailable: %v", err)
	}
	if acc, _ := f.accts.FindByID(context.Background(), "v1", f.id); acc.Status != domain.EmailAccountLinked {
		t.Fatalf("la cuenta debía seguir vinculada: %s", acc.Status)
	}
}
