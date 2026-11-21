// Package adapters (authentication) contiene BcryptPasswordHasher [ADR-14], JWTTokenServiceAdapter [ADR-16]
// y los repositorios sobre PriorityQueuePort [ADR-09].
package adapters

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
)

// BcryptPasswordHasher [ADR-14] Implementa application.PasswordHasher con bcrypt.
type BcryptPasswordHasher struct{ cost int }

var _ application.PasswordHasher = (*BcryptPasswordHasher)(nil)

// NewBcryptPasswordHasher crea el hasher; cost <= 0 usa bcrypt.DefaultCost.
func NewBcryptPasswordHasher(cost int) *BcryptPasswordHasher {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptPasswordHasher{cost: cost}
}

// Hash cifra la contraseña.
func (h *BcryptPasswordHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	return string(b), err
}

// Compare verifica la contraseña contra el hash.
func (h *BcryptPasswordHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
