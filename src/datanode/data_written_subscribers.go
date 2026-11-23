package datanode

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/datanode/cache"
	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
)

// dataWrittenSubscriber [ADR-10] Suscriptor interno de E6 DatosEscritos. Se entrega de forma síncrona,
// en orden de suscripción, antes de responder la escritura (read-your-writes).
// rec es el registro de replicación de la escritura confirmada por la primaria.
type dataWrittenSubscriber interface {
	OnDataWritten(ctx context.Context, e6 events.DataWritten, rec operations.ReplicationRecord)
}

// cacheInvalidator [ADR-10] Suscriptor de E6 que invalida las claves de la caché del viajero.
type cacheInvalidator struct{ cache *cache.InMemoryCacheAdapter }

func (c cacheInvalidator) OnDataWritten(ctx context.Context, e6 events.DataWritten, _ operations.ReplicationRecord) {
	_ = c.cache.Invalidate(ctx, e6.Claves...)
}

// readReplicaReplicator [ADR-10] Suscriptor de E6 que copia la escritura a la base de lectura.
// Si la copia falla, el ReplicationRetrier la deja pendiente y la reintenta; la escritura no se revierte.
type readReplicaReplicator struct{ retrier *ReplicationRetrier }

func (r readReplicaReplicator) OnDataWritten(ctx context.Context, _ events.DataWritten, rec operations.ReplicationRecord) {
	_ = r.retrier.Replicate(ctx, rec)
}
