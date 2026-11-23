package datanode

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/datanode/primary"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// SyncTarget es el nodo destino de la sincronización (el nodo de datos pasivo). [ADR-21]
type SyncTarget = ports.SyncTarget

// SyncEmitter [ADR-21] Emisor de sincronización periódica de estado hacia el nodo pasivo (E5).
// [Prototipo] Usa VACUUM INTO sobre los archivos SQLite de la primaria.
type SyncEmitter struct {
	mu         sync.Mutex // serializa sincronizaciones: comparten los archivos temporales de snapshot
	nodeID     string
	primary    *primary.PrimaryStore
	partitions []string
	tmpDir     string
}

// NewSyncEmitter crea el emisor sobre la primaria del nodo.
func NewSyncEmitter(nodeID string, p *primary.PrimaryStore, partitions []string, tmpDir string) *SyncEmitter {
	return &SyncEmitter{nodeID: nodeID, primary: p, partitions: partitions, tmpDir: tmpDir}
}

// SyncTo sincroniza todas las particiones hacia el destino y devuelve E5.
func (e *SyncEmitter) SyncTo(ctx context.Context, target SyncTarget) (events.StateSync, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, part := range e.partitions {
		tmp := filepath.Join(e.tmpDir, fmt.Sprintf("sync-%s-%s.tmp", e.nodeID, part))
		if err := e.primary.Snapshot(ctx, part, tmp); err != nil {
			return events.StateSync{}, fmt.Errorf("snapshot de %s: %w", part, err)
		}
		if err := target.InstallSnapshot(ctx, part, tmp); err != nil {
			return events.StateSync{}, fmt.Errorf("instalando %s en %s: %w", part, target.ID(), err)
		}
	}
	return events.StateSync{
		Origen: e.nodeID, Destino: target.ID(), VersionEstado: e.primary.Version(), EmitidoEn: time.Now().UTC(),
	}, nil
}
