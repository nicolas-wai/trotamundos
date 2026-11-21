// Package primary es el componente "Base primaria" del nodo de datos: escrituras en SQLite plano (AD §4.6).
package primary

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// PrimaryStore [ADR-10] Base primaria por shard (primary-shard-N.db). Tras escribir emite E6 DatosEscritos.
type PrimaryStore struct{}

// NewPrimaryStore crea la base primaria.
func NewPrimaryStore() *PrimaryStore { return &PrimaryStore{} }

// Write ejecuta una escritura nombrada en el shard.
func (*PrimaryStore) Write(ctx context.Context, shard int, command ports.DataCommand) (ports.DataResult, error) {
	return ports.DataResult{}, apierror.ErrNotImplemented
}
