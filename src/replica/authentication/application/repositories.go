// Package application (authentication) contiene los casos de uso de Autenticación y sus puertos propios.
// Solo conoce stdlib, src/ports, src/contracts y su propio domain (concern de tecnologías genéricas).
package application

import (
	"context"
	"errors"

	"github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
)

// Errores de la capa de aplicación de Autenticación.
var (
	ErrEmailTaken         = errors.New("el email ya está registrado")
	ErrTravelerNotFound   = errors.New("viajero no encontrado")
	ErrRefreshNotFound    = errors.New("refresh token no encontrado")
	ErrInvalidCredentials = errors.New("credenciales inválidas")
	ErrValidation         = errors.New("datos inválidos")
	// ErrLinkedAccountNotFound: la cuenta de correo no existe o es de otro viajero (no se revela cuál). [ADR-15]
	ErrLinkedAccountNotFound = errors.New("cuenta de correo vinculada no encontrada")
)

// TravelerRepository es el puerto de persistencia del agregado Cuenta del viajero.
// El adapter lo implementa sobre PriorityQueuePort [ADR-09]. El email único se resuelve vía directorio ◇ [ADR-11].
type TravelerRepository interface {
	// Create reserva el email en el directorio (ErrEmailTaken si existe) y crea el viajero en su shard.
	Create(ctx context.Context, traveler domain.Traveler) error
	// FindByEmail devuelve ErrTravelerNotFound si no existe.
	FindByEmail(ctx context.Context, email string) (domain.Traveler, error)
	// FindByID devuelve ErrTravelerNotFound si no existe.
	FindByID(ctx context.Context, travelerID string) (domain.Traveler, error)
}

// RefreshTokenRepository persiste los refresh tokens revocables. [ADR-16]
// Recibe travelerID porque es la clave de partición [ADR-11].
type RefreshTokenRepository interface {
	Save(ctx context.Context, token domain.RefreshToken) error
	// FindByID devuelve ErrRefreshNotFound si no existe.
	FindByID(ctx context.Context, travelerID, tokenID string) (domain.RefreshToken, error)
	Revoke(ctx context.Context, travelerID, tokenID string) error
}

// PreferencesRepository persiste las preferencias de recomendación del viajero. [ADR-25]
type PreferencesRepository interface {
	Get(ctx context.Context, travelerID string) (domain.TravelerPreferences, error)
	Save(ctx context.Context, preferences domain.TravelerPreferences) error
}

// PasswordHasher es el puerto de cifrado de contraseñas (adapter bcrypt). [ADR-14]
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

// LinkedEmailAccountRepository persiste las cuentas de correo vinculadas por OAuth (tokens ya cifrados). [ADR-15]
// El adapter lo implementa sobre PriorityQueuePort y mantiene el directorio que usa el planificador de sondeo.
type LinkedEmailAccountRepository interface {
	Save(ctx context.Context, account domain.LinkedEmailAccount) error
	// FindByID devuelve ErrLinkedAccountNotFound si no existe o no es del viajero.
	FindByID(ctx context.Context, travelerID, accountID string) (domain.LinkedEmailAccount, error)
	// Delete devuelve ErrLinkedAccountNotFound si no había nada que borrar.
	Delete(ctx context.Context, travelerID, accountID string) error
}
