package ports

import (
	"context"
	"errors"

	auth "github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
)

// Errores de la gestión de cuentas de correo vinculadas.
var (
	// ErrInvalidInput: faltan o son inválidos los datos de la solicitud de vinculación.
	ErrInvalidInput = errors.New("datos inválidos")
	// ErrLinkedAccountNotFound: la cuenta de correo no existe o es de otro viajero (no se revela cuál). [ADR-15]
	ErrLinkedAccountNotFound = errors.New("cuenta de correo vinculada no encontrada")
)

// LinkedEmailAccountPort [ADR-15] Persistencia de las cuentas de correo vinculadas por OAuth (tokens ya cifrados).
// Puerto compartido entre Autenticación (que implementa el adapter sobre la cola de prioridades) y el Gestor OAuth
// de Integraciones externas (que lo consume): así ninguno de estos dos módulos importa la capa application del
// otro [ADR-02].
type LinkedEmailAccountPort interface {
	Save(ctx context.Context, account auth.LinkedEmailAccount) error
	// FindByID devuelve ErrLinkedAccountNotFound si no existe o no es del viajero.
	FindByID(ctx context.Context, travelerID, accountID string) (auth.LinkedEmailAccount, error)
	// Delete devuelve ErrLinkedAccountNotFound si no había nada que borrar.
	Delete(ctx context.Context, travelerID, accountID string) error
}
