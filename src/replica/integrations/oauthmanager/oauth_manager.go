// Package oauthmanager es el componente "Gestor OAuth de correo" (AD §4.4). [ADR-15]
// Depende solo de puertos (src/ports) y del domain de Autenticación; no importa su capa application.
package oauthmanager

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
)

// OAuthManager [ADR-15] Vinculación, almacenamiento (tokens cifrados) y revocación de permisos de correo
// sobre OAuthEmailPort. Nunca se guarda la contraseña del correo, solo tokens OAuth cifrados en reposo [QA-08].
type OAuthManager struct {
	oauth    ports.OAuthEmailPort
	accounts ports.LinkedEmailAccountPort
	cipher   ports.TokenCipherPort
}

// NewOAuthManager crea el gestor.
func NewOAuthManager(oauth ports.OAuthEmailPort, accounts ports.LinkedEmailAccountPort, cipher ports.TokenCipherPort) *OAuthManager {
	return &OAuthManager{oauth: oauth, accounts: accounts, cipher: cipher}
}

// Link intercambia el código OAuth (flujo 5.4), cifra los tokens y persiste la cuenta vinculada.
// Campos vacíos → ports.ErrInvalidInput; código rechazado → ports.ErrInvalidAuthorizationCode.
func (m *OAuthManager) Link(ctx context.Context, travelerID, provider, authorizationCode, redirectURI string) (domain.LinkedEmailAccount, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch {
	case provider == "":
		return domain.LinkedEmailAccount{}, fmt.Errorf("%w: falta proveedor", ports.ErrInvalidInput)
	case strings.TrimSpace(authorizationCode) == "":
		return domain.LinkedEmailAccount{}, fmt.Errorf("%w: falta authorizationCode", ports.ErrInvalidInput)
	case strings.TrimSpace(redirectURI) == "":
		return domain.LinkedEmailAccount{}, fmt.Errorf("%w: falta redirectUri", ports.ErrInvalidInput)
	}
	tokens, err := m.oauth.ExchangeCode(ctx, provider, authorizationCode, redirectURI)
	if err != nil {
		return domain.LinkedEmailAccount{}, err
	}
	access, err := m.cipher.Encrypt(tokens.AccessToken)
	if err != nil {
		return domain.LinkedEmailAccount{}, err
	}
	refresh, err := m.cipher.Encrypt(tokens.RefreshToken)
	if err != nil {
		return domain.LinkedEmailAccount{}, err
	}
	id, err := newAccountID()
	if err != nil {
		return domain.LinkedEmailAccount{}, err
	}
	account := domain.LinkedEmailAccount{
		AccountID: id, TravelerID: travelerID, Provider: provider,
		EncryptedAccessToken: access, EncryptedRefreshToken: refresh, Status: domain.EmailAccountLinked,
	}
	if err := m.accounts.Save(ctx, account); err != nil {
		return domain.LinkedEmailAccount{}, err
	}
	return account, nil
}

// Unlink revoca los tokens en el proveedor y elimina el vínculo (flujo 5.4, rama opcional).
// ports.ErrLinkedAccountNotFound si la cuenta no existe o es de otro viajero.
func (m *OAuthManager) Unlink(ctx context.Context, travelerID, accountID string) error {
	if _, err := m.accounts.FindByID(ctx, travelerID, accountID); err != nil {
		return err
	}
	if err := m.oauth.Revoke(ctx, accountID); err != nil {
		return fmt.Errorf("revocando permisos en el proveedor: %w", err)
	}
	return m.accounts.Delete(ctx, travelerID, accountID)
}

// AccessToken entrega el access token descifrado de una cuenta vinculada (lo usa el Adapter Email al sondear).
func (m *OAuthManager) AccessToken(ctx context.Context, travelerID, accountID string) (string, error) {
	account, err := m.accounts.FindByID(ctx, travelerID, accountID)
	if err != nil {
		return "", err
	}
	if account.Status != domain.EmailAccountLinked {
		return "", errors.New("la cuenta de correo no está vinculada")
	}
	return m.cipher.Decrypt(account.EncryptedAccessToken)
}

func newAccountID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("cta-%x", b), nil
}
