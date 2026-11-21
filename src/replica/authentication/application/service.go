package application

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// AuthenticationService [ADR-14][ADR-16] Registro, login, refresco y logout con tokens de vida corta.
type AuthenticationService interface {
	Register(ctx context.Context, email, password string) (travelerID string, err error)
	Login(ctx context.Context, email, password string) (ports.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (ports.AccessToken, error)
	Logout(ctx context.Context, refreshToken string) error
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
