package datanode

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
)

// SyncEmitter [ADR-21] Emisor de sincronización periódica de estado hacia el nodo pasivo (E5).
// En el prototipo usará VACUUM INTO sobre los archivos SQLite (Fase 2).
type SyncEmitter struct{}

// NewSyncEmitter crea el emisor.
func NewSyncEmitter() *SyncEmitter { return &SyncEmitter{} }

// SyncTo sincroniza el estado hacia el destino y devuelve E5.
func (e *SyncEmitter) SyncTo(ctx context.Context, destination string) (events.StateSync, error) {
	return events.StateSync{}, apierror.ErrNotImplemented
}
