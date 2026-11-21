package adapters

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	"github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
)

// JWTTokenServiceAdapter [ADR-16] Implementa TokenServicePort: access token JWT HS256 de vida corta,
// validado localmente por cualquier réplica (solo comparte el secreto de firma, ningún estado), y refresh
// token persistido y revocable. La revocación inmediata del access token no es posible (decisión de ADR-16).
//
// Formato del refresh token: "<idViajero>.<secreto>". El idViajero (clave de partición, no secreto)
// permite ubicar el shard sin access token; solo se persiste sha256(secreto).
type JWTTokenServiceAdapter struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	refresh    application.RefreshTokenRepository
	now        func() time.Time
}

var _ ports.TokenServicePort = (*JWTTokenServiceAdapter)(nil)

// NewJWTTokenServiceAdapter crea el adaptador; now == nil usa time.Now (reloj inyectable para pruebas).
func NewJWTTokenServiceAdapter(secret string, accessTTL, refreshTTL time.Duration, refresh application.RefreshTokenRepository, now func() time.Time) *JWTTokenServiceAdapter {
	if now == nil {
		now = time.Now
	}
	return &JWTTokenServiceAdapter{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL, refresh: refresh, now: now}
}

func (a *JWTTokenServiceAdapter) signAccess(travelerID string) (string, error) {
	now := a.now()
	claims := jwt.RegisteredClaims{Subject: travelerID, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(a.accessTTL))}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
}

// Issue emite el access token y persiste un refresh token nuevo.
func (a *JWTTokenServiceAdapter) Issue(ctx context.Context, travelerID string) (ports.TokenPair, error) {
	access, err := a.signAccess(travelerID)
	if err != nil {
		return ports.TokenPair{}, err
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return ports.TokenPair{}, err
	}
	secret := hex.EncodeToString(raw[:])
	err = a.refresh.Save(ctx, domain.RefreshToken{TokenID: hashSecret(secret), TravelerID: travelerID, ExpiresAt: a.now().Add(a.refreshTTL)})
	if err != nil {
		return ports.TokenPair{}, err
	}
	return ports.TokenPair{AccessToken: access, RefreshToken: travelerID + "." + secret, ExpiresIn: int(a.accessTTL.Seconds())}, nil
}

// Validate verifica firma y vigencia sin consultar estado compartido.
func (a *JWTTokenServiceAdapter) Validate(accessToken string) (ports.Claims, error) {
	var rc jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(accessToken, &rc, func(*jwt.Token) (any, error) { return a.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(a.now), jwt.WithExpirationRequired())
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return ports.Claims{}, ports.ErrTokenExpired
	case err != nil || rc.Subject == "":
		return ports.Claims{}, ports.ErrTokenInvalid
	}
	return ports.Claims{TravelerID: rc.Subject, ExpiresAt: rc.ExpiresAt.Time}, nil
}

// lookup valida un refresh token y devuelve su registro persistido.
func (a *JWTTokenServiceAdapter) lookup(ctx context.Context, refreshToken string) (domain.RefreshToken, error) {
	travelerID, secret, ok := strings.Cut(refreshToken, ".")
	if !ok || travelerID == "" || secret == "" {
		return domain.RefreshToken{}, ports.ErrRefreshRevokedOrExpired
	}
	t, err := a.refresh.FindByID(ctx, travelerID, hashSecret(secret))
	if errors.Is(err, application.ErrRefreshNotFound) {
		return domain.RefreshToken{}, ports.ErrRefreshRevokedOrExpired
	}
	return t, err
}

// Refresh emite un access token nuevo si el refresh está vigente y no revocado.
func (a *JWTTokenServiceAdapter) Refresh(ctx context.Context, refreshToken string) (ports.AccessToken, error) {
	t, err := a.lookup(ctx, refreshToken)
	if err != nil {
		return ports.AccessToken{}, err
	}
	if t.Revoked || !a.now().Before(t.ExpiresAt) {
		return ports.AccessToken{}, ports.ErrRefreshRevokedOrExpired
	}
	access, err := a.signAccess(t.TravelerID)
	if err != nil {
		return ports.AccessToken{}, err
	}
	return ports.AccessToken{Token: access, ExpiresIn: int(a.accessTTL.Seconds())}, nil
}

// RevokeRefresh revoca el refresh token. Idempotente: un token desconocido no es error.
func (a *JWTTokenServiceAdapter) RevokeRefresh(ctx context.Context, refreshToken string) error {
	t, err := a.lookup(ctx, refreshToken)
	if errors.Is(err, ports.ErrRefreshRevokedOrExpired) {
		return nil
	}
	if err != nil {
		return err
	}
	return a.refresh.Revoke(ctx, t.TravelerID, t.TokenID)
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
