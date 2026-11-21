package application

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
)

// AuthenticationService [ADR-14][ADR-16] Registro, login, refresco y logout con tokens de vida corta.
type AuthenticationService interface {
	Register(ctx context.Context, email, password string) (travelerID string, err error)
	Login(ctx context.Context, email, password string) (ports.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (ports.AccessToken, error)
	Logout(ctx context.Context, refreshToken string) error
}

// MinPasswordLength es el largo mínimo de contraseña aceptado.
const MinPasswordLength = 8

// Service implementa AuthenticationService sobre puertos propios (sin librerías concretas).
type Service struct {
	travelers TravelerRepository
	hasher    PasswordHasher
	tokens    ports.TokenServicePort
	now       func() time.Time
}

var _ AuthenticationService = (*Service)(nil)

// NewService crea el servicio de autenticación.
func NewService(travelers TravelerRepository, hasher PasswordHasher, tokens ports.TokenServicePort, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{travelers: travelers, hasher: hasher, tokens: tokens, now: now}
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// Register crea la cuenta: email único (vía directorio) y contraseña cifrada. [ADR-14][QA-05]
func (s *Service) Register(ctx context.Context, email, password string) (string, error) {
	email = normalizeEmail(email)
	if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@") {
		return "", fmt.Errorf("%w: email con formato inválido", ErrValidation)
	}
	if len(password) < MinPasswordLength {
		return "", fmt.Errorf("%w: la contraseña debe tener al menos %d caracteres", ErrValidation, MinPasswordLength)
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	err = s.travelers.Create(ctx, domain.Traveler{
		TravelerID: id, Email: email, EncryptedPassword: hash, CreatedAt: s.now().UTC(),
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// Login valida credenciales y emite el par de tokens. Email desconocido y contraseña errónea
// producen el mismo error para no revelar qué cuentas existen.
func (s *Service) Login(ctx context.Context, email, password string) (ports.TokenPair, error) {
	t, err := s.travelers.FindByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, ErrTravelerNotFound) {
		return ports.TokenPair{}, ErrInvalidCredentials
	}
	if err != nil {
		return ports.TokenPair{}, err
	}
	if s.hasher.Compare(t.EncryptedPassword, password) != nil {
		return ports.TokenPair{}, ErrInvalidCredentials
	}
	return s.tokens.Issue(ctx, t.TravelerID)
}

// Refresh renueva el access token con un refresh vigente y no revocado. [ADR-16]
func (s *Service) Refresh(ctx context.Context, refreshToken string) (ports.AccessToken, error) {
	return s.tokens.Refresh(ctx, refreshToken)
}

// Logout revoca el refresh token: única vía de revocación (ADR-16). Es idempotente.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.tokens.RevokeRefresh(ctx, refreshToken)
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

// TravelerProfileService gestiona las preferencias de recomendación del perfil (PUT /perfil/preferencias).
// [ADR-25] Distintas de las preferencias visuales en LocalStorage [ADR-19].
type TravelerProfileService interface {
	UpdatePreferences(ctx context.Context, travelerID string, params map[string]any) error
}

// UnimplementedAuthenticationService es el stub de la Fase 1.
type UnimplementedAuthenticationService struct{}

// Register no implementado.
func (UnimplementedAuthenticationService) Register(context.Context, string, string) (string, error) {
	return "", apierror.ErrNotImplemented
}

// Login no implementado.
func (UnimplementedAuthenticationService) Login(context.Context, string, string) (ports.TokenPair, error) {
	return ports.TokenPair{}, apierror.ErrNotImplemented
}

// Refresh no implementado.
func (UnimplementedAuthenticationService) Refresh(context.Context, string) (ports.AccessToken, error) {
	return ports.AccessToken{}, apierror.ErrNotImplemented
}

// Logout no implementado.
func (UnimplementedAuthenticationService) Logout(context.Context, string) error {
	return apierror.ErrNotImplemented
}

// UnimplementedTravelerProfileService es el stub de la Fase 1.
type UnimplementedTravelerProfileService struct{}

// UpdatePreferences no implementado.
func (UnimplementedTravelerProfileService) UpdatePreferences(context.Context, string, map[string]any) error {
	return apierror.ErrNotImplemented
}
