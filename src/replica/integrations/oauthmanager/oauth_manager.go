// Package oauthmanager es el componente "Gestor OAuth de correo" (AD §4.4). [ADR-15]
// Depende solo de puertos (src/ports) y del domain de Autenticación; no importa su capa application.
package oauthmanager

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
)

// OAuthManager [ADR-15] Vinculación, almacenamiento (tokens cifrados) y revocación de permisos de correo
// sobre OAuthEmailPort. Nunca se guarda la contraseña del correo, solo tokens OAuth cifrados en reposo [QA-08].
type OAuthManager struct {
	oauth    ports.OAuthEmailPort
	accounts ports.LinkedEmailAccountPort
	cipher   ports.TokenCipherPort
	now      func() time.Time

	mu    sync.Mutex
	locks map[string]*sync.Mutex // un candado por cuenta: evita renovaciones simultáneas [ADR-15]
}

// refreshMargin: se renueva el token cuando le falta menos que esto para vencer. [ADR-15]
const refreshMargin = time.Minute

// NewOAuthManager crea el gestor.
func NewOAuthManager(oauth ports.OAuthEmailPort, accounts ports.LinkedEmailAccountPort, cipher ports.TokenCipherPort) *OAuthManager {
	return &OAuthManager{oauth: oauth, accounts: accounts, cipher: cipher, now: time.Now, locks: map[string]*sync.Mutex{}}
}

// WithClock inyecta el reloj con el que se decide si el token venció (nil deja time.Now).
func (m *OAuthManager) WithClock(now func() time.Time) *OAuthManager {
	if now != nil {
		m.now = now
	}
	return m
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
		EncryptedAccessToken: access, EncryptedRefreshToken: refresh, ExpiresAt: tokens.ExpiresAt, Status: domain.EmailAccountLinked,
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
// [ADR-15] Si el token venció (o falta poco) lo renueva con OAuthEmailPort.Refresh, guarda los tokens nuevos
// cifrados y devuelve el nuevo. Si el proveedor rechaza la renovación, la cuenta queda como "requiere volver a
// vincular" (el planificador deja de sondearla) y se devuelve ports.ErrEmailRelinkRequired, que no es una falla
// del proveedor [ADR-07]. Una falla transitoria de la renovación devuelve ports.ErrProviderUnavailable.
func (m *OAuthManager) AccessToken(ctx context.Context, travelerID, accountID string) (string, error) {
	account, err := m.usableAccount(ctx, travelerID, accountID)
	if err != nil {
		return "", err
	}
	if !m.needsRefresh(account) {
		return m.cipher.Decrypt(account.EncryptedAccessToken)
	}

	lock := m.accountLock(accountID)
	lock.Lock()
	defer lock.Unlock()
	// Otra goroutine pudo renovar mientras esperábamos el candado: se vuelve a leer.
	if account, err = m.usableAccount(ctx, travelerID, accountID); err != nil {
		return "", err
	}
	if !m.needsRefresh(account) {
		return m.cipher.Decrypt(account.EncryptedAccessToken)
	}
	tokens, err := m.oauth.Refresh(ctx, accountID)
	if err != nil {
		if errors.Is(err, ports.ErrOAuthAccountRevoked) || errors.Is(err, ports.ErrInvalidAuthorizationCode) || errors.Is(err, ports.ErrInvalidCredential) {
			account.Status = domain.EmailAccountRelinkRequired
			if saveErr := m.accounts.Save(ctx, account); saveErr != nil {
				return "", saveErr
			}
			return "", fmt.Errorf("%w: %v", ports.ErrEmailRelinkRequired, err)
		}
		return "", fmt.Errorf("%w: renovando el token: %v", ports.ErrProviderUnavailable, err)
	}
	access, err := m.cipher.Encrypt(tokens.AccessToken)
	if err != nil {
		return "", err
	}
	account.EncryptedAccessToken = access
	if tokens.RefreshToken != "" {
		if account.EncryptedRefreshToken, err = m.cipher.Encrypt(tokens.RefreshToken); err != nil {
			return "", err
		}
	}
	account.ExpiresAt = tokens.ExpiresAt
	if err := m.accounts.Save(ctx, account); err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

func (m *OAuthManager) usableAccount(ctx context.Context, travelerID, accountID string) (domain.LinkedEmailAccount, error) {
	account, err := m.accounts.FindByID(ctx, travelerID, accountID)
	if err != nil {
		return domain.LinkedEmailAccount{}, err
	}
	switch account.Status {
	case domain.EmailAccountLinked:
		return account, nil
	case domain.EmailAccountRelinkRequired:
		return domain.LinkedEmailAccount{}, ports.ErrEmailRelinkRequired
	}
	return domain.LinkedEmailAccount{}, errors.New("la cuenta de correo no está vinculada")
}

// needsRefresh: sin vencimiento conocido (cuentas anteriores a la columna) también se renueva.
func (m *OAuthManager) needsRefresh(a domain.LinkedEmailAccount) bool {
	return a.ExpiresAt.IsZero() || !m.now().Add(refreshMargin).Before(a.ExpiresAt)
}

func (m *OAuthManager) accountLock(accountID string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.locks[accountID]
	if !ok {
		l = &sync.Mutex{}
		m.locks[accountID] = l
	}
	return l
}

func newAccountID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("cta-%x", b), nil
}
