package adapters

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	"github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
)

// memRefreshRepo es un RefreshTokenRepository en memoria para probar el adapter aislado.
type memRefreshRepo struct {
	tokens map[string]domain.RefreshToken
}

func (m *memRefreshRepo) Save(_ context.Context, t domain.RefreshToken) error {
	m.tokens[t.TravelerID+"/"+t.TokenID] = t
	return nil
}
func (m *memRefreshRepo) FindByID(_ context.Context, traveler, id string) (domain.RefreshToken, error) {
	t, ok := m.tokens[traveler+"/"+id]
	if !ok {
		return domain.RefreshToken{}, application.ErrRefreshNotFound
	}
	return t, nil
}
func (m *memRefreshRepo) Revoke(_ context.Context, traveler, id string) error {
	t := m.tokens[traveler+"/"+id]
	t.Revoked = true
	m.tokens[traveler+"/"+id] = t
	return nil
}

func newService() (*JWTTokenServiceAdapter, *time.Time, *memRefreshRepo) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	repo := &memRefreshRepo{tokens: map[string]domain.RefreshToken{}}
	return NewJWTTokenServiceAdapter("secreto", 900*time.Second, time.Hour, repo, func() time.Time { return now }), &now, repo
}

func TestIssueAndValidate(t *testing.T) {
	svc, _, repo := newService()
	pair, err := svc.Issue(context.Background(), "v1")
	if err != nil || pair.ExpiresIn != 900 || pair.AccessToken == "" {
		t.Fatalf("%+v %v", pair, err)
	}
	claims, err := svc.Validate(pair.AccessToken)
	if err != nil || claims.TravelerID != "v1" {
		t.Fatalf("%+v %v", claims, err)
	}
	if len(repo.tokens) != 1 {
		t.Fatal("el refresh debía persistirse")
	}
	for _, tok := range repo.tokens {
		if strings.Contains(pair.RefreshToken, tok.TokenID) {
			t.Fatal("se persiste el hash del secreto, no el secreto")
		}
	}
}

func TestValidateRejectsExpiredTamperedAndForeignSecretTokens(t *testing.T) {
	svc, now, _ := newService()
	pair, _ := svc.Issue(context.Background(), "v1")

	if _, err := svc.Validate(pair.AccessToken + "x"); !errors.Is(err, ports.ErrTokenInvalid) {
		t.Fatalf("token manipulado: %v", err)
	}
	if _, err := svc.Validate("no-es-un-jwt"); !errors.Is(err, ports.ErrTokenInvalid) {
		t.Fatalf("basura: %v", err)
	}
	other := NewJWTTokenServiceAdapter("otro-secreto", time.Hour, time.Hour, &memRefreshRepo{tokens: map[string]domain.RefreshToken{}}, nil)
	otherPair, _ := other.Issue(context.Background(), "v1")
	if _, err := svc.Validate(otherPair.AccessToken); !errors.Is(err, ports.ErrTokenInvalid) {
		t.Fatalf("firmado con otro secreto: %v", err)
	}
	*now = now.Add(899 * time.Second)
	if _, err := svc.Validate(pair.AccessToken); err != nil {
		t.Fatalf("aún vigente: %v", err)
	}
	*now = now.Add(2 * time.Second)
	if _, err := svc.Validate(pair.AccessToken); !errors.Is(err, ports.ErrTokenExpired) {
		t.Fatalf("debía expirar a los 900 s: %v", err)
	}
}

func TestRefreshRevokedExpiredAndMalformed(t *testing.T) {
	svc, now, _ := newService()
	ctx := context.Background()
	pair, _ := svc.Issue(ctx, "v1")

	at, err := svc.Refresh(ctx, pair.RefreshToken)
	if err != nil || at.Token == "" || at.ExpiresIn != 900 {
		t.Fatalf("%+v %v", at, err)
	}
	if err := svc.RevokeRefresh(ctx, pair.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, pair.RefreshToken); !errors.Is(err, ports.ErrRefreshRevokedOrExpired) {
		t.Fatalf("revocado: %v", err)
	}
	if err := svc.RevokeRefresh(ctx, pair.RefreshToken); err != nil {
		t.Fatalf("revocar es idempotente: %v", err)
	}
	if err := svc.RevokeRefresh(ctx, "basura"); err != nil {
		t.Fatalf("un token desconocido no es error: %v", err)
	}
	for _, bad := range []string{"", "basura", "v1.", ".x", "v1.secreto-falso", "v2." + strings.SplitN(pair.RefreshToken, ".", 2)[1]} {
		if _, err := svc.Refresh(ctx, bad); !errors.Is(err, ports.ErrRefreshRevokedOrExpired) {
			t.Fatalf("%q debía ser inválido: %v", bad, err)
		}
	}
	fresh, _ := svc.Issue(ctx, "v1")
	*now = now.Add(time.Hour + time.Second)
	if _, err := svc.Refresh(ctx, fresh.RefreshToken); !errors.Is(err, ports.ErrRefreshRevokedOrExpired) {
		t.Fatalf("expirado: %v", err)
	}
}

func TestBcryptHasher(t *testing.T) {
	h := NewBcryptPasswordHasher(4)
	hash, err := h.Hash("contraseña-123")
	if err != nil || hash == "contraseña-123" {
		t.Fatalf("%q %v", hash, err)
	}
	if h.Compare(hash, "contraseña-123") != nil || h.Compare(hash, "otra") == nil {
		t.Fatal("comparación incorrecta")
	}
}
