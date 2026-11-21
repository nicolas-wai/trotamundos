// Package oauthmanager es el componente "Gestor OAuth de correo" (AD §4.4).
package oauthmanager

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
)

// OAuthManager [ADR-15] Vinculación, almacenamiento (tokens cifrados) y revocación de permisos de correo
// sobre OAuthEmailPort. Nunca se guarda la contraseña del correo. Lógica en la Fase 4.
type OAuthManager struct {
	oauth ports.OAuthEmailPort
}

// NewOAuthManager crea el gestor.
func NewOAuthManager(oauth ports.OAuthEmailPort) *OAuthManager { return &OAuthManager{oauth: oauth} }

// Link intercambia el código OAuth y persiste la cuenta vinculada.
func (m *OAuthManager) Link(ctx context.Context, travelerID, provider, authorizationCode, redirectURI string) (domain.LinkedEmailAccount, error) {
	return domain.LinkedEmailAccount{}, apierror.ErrNotImplemented
}

// Unlink revoca los tokens y elimina el vínculo.
func (m *OAuthManager) Unlink(ctx context.Context, travelerID, accountID string) error {
	return apierror.ErrNotImplemented
}
