// Package application (authentication) contiene los casos de uso de Autenticación y sus puertos propios.
// Solo conoce stdlib, src/ports, src/contracts y su propio domain (concern de tecnologías genéricas).
package application

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
)

// TravelerRepository es el puerto de persistencia del agregado Cuenta del viajero.
// El adapter lo implementa sobre PriorityQueuePort [ADR-09]. El email único se resuelve vía directorio ◇ [ADR-11].
type TravelerRepository interface {
	Create(ctx context.Context, traveler domain.Traveler) error
	FindByEmail(ctx context.Context, email string) (domain.Traveler, error)
	FindByID(ctx context.Context, travelerID string) (domain.Traveler, error)
}

// RefreshTokenRepository persiste los refresh tokens revocables. [ADR-16]
type RefreshTokenRepository interface {
	Save(ctx context.Context, token domain.RefreshToken) error
	FindByID(ctx context.Context, tokenID string) (domain.RefreshToken, error)
	Revoke(ctx context.Context, tokenID string) error
}

// PreferencesRepository persiste las preferencias de recomendación del viajero. [ADR-25]
type PreferencesRepository interface {
	Get(ctx context.Context, travelerID string) (domain.TravelerPreferences, error)
	Save(ctx context.Context, preferences domain.TravelerPreferences) error
}

// PasswordHasher es el puerto de cifrado de contraseñas (adapter bcrypt en Fase 2). [ADR-14]
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}
