// Package readreplica es el componente "Réplicas de lectura" del nodo de datos (AD §4.6).
package readreplica

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// ReadReplicaStore [ADR-10] Réplica de lectura por shard (replica-shard-N.db), atiende los cache miss.
type ReadReplicaStore struct{}

// NewReadReplicaStore crea la réplica de lectura.
func NewReadReplicaStore() *ReadReplicaStore { return &ReadReplicaStore{} }

// Read ejecuta una lectura nombrada en el shard.
func (*ReadReplicaStore) Read(ctx context.Context, shard int, query ports.DataQuery) (ports.DataResult, error) {
	return ports.DataResult{}, apierror.ErrNotImplemented
}
