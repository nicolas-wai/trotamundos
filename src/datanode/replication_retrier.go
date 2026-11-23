package datanode

import (
	"context"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
)

// replicaApplier aplica un registro de replicación en la base de lectura (la implementa ReadReplicaStore).
type replicaApplier interface {
	Replicate(ctx context.Context, rec operations.ReplicationRecord) error
}

// ReplicationRetrier [ADR-10] Lista de replicaciones pendientes por partición. Si la copia a la base de lectura
// falla, el registro queda aquí y se reintenta en segundo plano, siempre en el orden original de la partición.
// Mientras una partición tenga pendientes, DataNode.Read la atiende desde la primaria (nunca devuelve datos viejos).
// [Prototipo] La lista vive en memoria: si el proceso muere se pierde (la sincronización completa del pasivo
// o un nuevo snapshot reconstruye la réplica).
type ReplicationRetrier struct {
	mu      sync.Mutex
	applier replicaApplier
	fault   func(operations.ReplicationRecord) error // falla inyectable para pruebas
	pending map[string][]operations.ReplicationRecord
}

// NewReplicationRetrier crea el reintentador sobre la réplica de lectura. fault puede ser nil.
func NewReplicationRetrier(applier replicaApplier, fault func(operations.ReplicationRecord) error) *ReplicationRetrier {
	return &ReplicationRetrier{applier: applier, fault: fault, pending: map[string][]operations.ReplicationRecord{}}
}

func (r *ReplicationRetrier) apply(ctx context.Context, rec operations.ReplicationRecord) error {
	if r.fault != nil {
		if err := r.fault(rec); err != nil {
			return err
		}
	}
	return r.applier.Replicate(ctx, rec)
}

// Replicate replica el registro; si la partición ya tiene pendientes o la copia falla, lo encola (conserva el orden).
// Devuelve el error de la copia (nil si se aplicó o si solo quedó detrás de otros pendientes).
func (r *ReplicationRetrier) Replicate(ctx context.Context, rec operations.ReplicationRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending[rec.Partition]) > 0 {
		r.pending[rec.Partition] = append(r.pending[rec.Partition], rec)
		return nil
	}
	if err := r.apply(ctx, rec); err != nil {
		r.pending[rec.Partition] = append(r.pending[rec.Partition], rec)
		return err
	}
	return nil
}

// Pending indica si la partición tiene replicaciones sin aplicar.
func (r *ReplicationRetrier) Pending(partition string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pending[partition]) > 0
}

// PendingCount devuelve el total de replicaciones pendientes de todas las particiones.
func (r *ReplicationRetrier) PendingCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, l := range r.pending {
		n += len(l)
	}
	return n
}

// Clear descarta los pendientes de la partición (la réplica se reconstruyó desde un snapshot de la primaria).
func (r *ReplicationRetrier) Clear(partition string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.pending, partition)
}

// Flush reintenta una vez todas las particiones, en orden, deteniéndose en el primer fallo de cada una.
func (r *ReplicationRetrier) Flush(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for part, list := range r.pending {
		for len(list) > 0 {
			if err := r.apply(ctx, list[0]); err != nil {
				break
			}
			list = list[1:]
		}
		if len(list) == 0 {
			delete(r.pending, part)
		} else {
			r.pending[part] = list
		}
	}
}

// Run reintenta cada intervalo hasta que el contexto termina (interval <= 0 la desactiva: se usa Flush).
func (r *ReplicationRetrier) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		<-ctx.Done()
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Flush(ctx)
		}
	}
}
