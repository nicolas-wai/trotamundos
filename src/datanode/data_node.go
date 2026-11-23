// Package datanode es el contenedor "Nodo de datos primario / pasivo" (AD §4.6).
// El nodo pasivo es una instancia del mismo módulo con la misma estructura interna [ADR-21].
package datanode

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/datanode/cache"
	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
	"github.com/nicolas-wai/trotamundos/src/datanode/primary"
	"github.com/nicolas-wai/trotamundos/src/datanode/readreplica"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Role es el rol del nodo dentro de la región. [ADR-21]
type Role string

// Roles del nodo de datos.
const (
	RoleActive  Role = "ACTIVO"
	RolePassive Role = "PASIVO"
)

// Config parametriza un nodo de datos.
type Config struct {
	ID                string
	Role              Role
	Dir               string // directorio propio del nodo (sus .db)
	Shards            int
	Heartbeat         ports.HeartbeatPort
	HeartbeatInterval time.Duration
	SyncInterval      time.Duration // 0 desactiva el bucle periódico (SyncNow sigue disponible)
	// ReplicationRetryInterval: cada cuánto se reintentan las replicaciones pendientes (0: solo manual con FlushReplication). [ADR-10]
	ReplicationRetryInterval time.Duration
	// ReplicationFault inyecta fallas en la copia a la base de lectura (solo pruebas). [ADR-10]
	ReplicationFault func(operations.ReplicationRecord) error
	Catalog          *operations.Catalog
}

// DataNode [ADR-10][ADR-11][ADR-21] Caché + base primaria + réplica de lectura, particionado por idViajero.
type DataNode struct {
	id           string
	router       *PartitionRouter
	cache        *cache.InMemoryCacheAdapter
	primary      *primary.PrimaryStore
	replica      *readreplica.ReadReplicaStore
	catalog      *operations.Catalog
	syncEmitter  *SyncEmitter
	heartbeat    *DataNodeHeartbeatEmitter
	syncInterval time.Duration
	retrier      *ReplicationRetrier
	retryEvery   time.Duration
	recoverCh    chan struct{}
	guards       map[string]*partitionGuard // [ADR-09] evita cachear datos viejos cuando lecturas y escrituras se solapan
	internalSubs []dataWrittenSubscriber    // suscriptores internos de E6: caché y réplica de lectura [ADR-10]

	mu          sync.RWMutex
	role        Role
	down        bool
	peer        SyncTarget
	cancel      context.CancelFunc
	subscribers []func(events.DataWritten)
}

var (
	_ ports.DataPort          = (*DataNode)(nil)
	_ ports.PartitionResolver = (*DataNode)(nil)
	_ SyncTarget              = (*DataNode)(nil)
)

// NewDataNode ensambla un nodo de datos y abre sus archivos SQLite.
func NewDataNode(cfg Config) (*DataNode, error) {
	router := NewPartitionRouter(cfg.Shards)
	parts := router.Partitions()
	prim, err := primary.NewPrimaryStore(cfg.Dir, parts)
	if err != nil {
		return nil, err
	}
	repl, err := readreplica.NewReadReplicaStore(cfg.Dir, parts)
	if err != nil {
		prim.Close()
		return nil, err
	}
	catalog := cfg.Catalog
	if catalog == nil {
		catalog = operations.DefaultCatalog()
	}
	n := &DataNode{
		id: cfg.ID, role: cfg.Role, router: router, cache: cache.NewInMemoryCacheAdapter(),
		primary: prim, replica: repl, catalog: catalog, syncInterval: cfg.SyncInterval,
		syncEmitter: NewSyncEmitter(cfg.ID, prim, parts, filepath.Join(cfg.Dir, "sync")),
	}
	n.guards = map[string]*partitionGuard{}
	for _, p := range parts {
		n.guards[p] = &partitionGuard{}
	}
	n.retrier = NewReplicationRetrier(repl, cfg.ReplicationFault)
	n.retryEvery = cfg.ReplicationRetryInterval
	n.recoverCh = make(chan struct{}, 1)
	// Orden de suscripción = orden de entrega: primero se invalida la caché, después se copia a lectura. [ADR-10]
	n.internalSubs = []dataWrittenSubscriber{cacheInvalidator{n.cache}, readReplicaReplicator{n.retrier}}
	n.heartbeat = NewDataNodeHeartbeatEmitter(cfg.ID, cfg.Heartbeat, cfg.HeartbeatInterval, n.Role)
	return n, nil
}

// ID devuelve el identificador del nodo.
func (n *DataNode) ID() string { return n.id }

// PartitionOf [ADR-11] Partición que atiende una operación E8; la cola de prioridades la usa para repartir el
// trabajo entre sus workers.
func (n *DataNode) PartitionOf(op events.QueuedOperation) (string, error) {
	kind := operations.Read
	if op.Tipo == events.OperationWrite {
		kind = operations.Write
	}
	resolved, err := n.catalog.Resolve(op.Recurso, kind)
	if err != nil {
		return "", err
	}
	return n.router.PartitionFor(op.IDViajero, resolved.Scope)
}

// Partitions lista las particiones del nodo (shards + directory).
func (n *DataNode) Partitions() []string { return n.router.Partitions() }

// Router expone el enrutador de partición.
func (n *DataNode) Router() *PartitionRouter { return n.router }

// Cache expone la caché (para pruebas y métricas).
func (n *DataNode) Cache() *cache.InMemoryCacheAdapter { return n.cache }

// Role devuelve el rol actual.
func (n *DataNode) Role() Role {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.role
}

// Promote pasa el nodo a primario: activa su heartbeat como ACTIVO y sus sincronizaciones. [ADR-22]
func (n *DataNode) Promote() { n.setRole(RoleActive) }

// Demote pasa el nodo a pasivo.
func (n *DataNode) Demote() { n.setRole(RolePassive) }

func (n *DataNode) setRole(r Role) {
	n.mu.Lock()
	n.role = r
	n.mu.Unlock()
}

// SetPeer fija el nodo hacia el que sincroniza cuando es primario.
func (n *DataNode) SetPeer(p SyncTarget) {
	n.mu.Lock()
	n.peer = p
	n.mu.Unlock()
}

// Subscribe registra un observador de E6 DatosEscritos.
func (n *DataNode) Subscribe(fn func(events.DataWritten)) {
	n.mu.Lock()
	n.subscribers = append(n.subscribers, fn)
	n.mu.Unlock()
}

func (n *DataNode) isDown() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.down
}

// Stop simula la caída del nodo: deja de latir, de sincronizar y de atender. [QA-04]
func (n *DataNode) Stop() {
	n.mu.Lock()
	n.down = true
	cancel := n.cancel
	n.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Recover simula la vuelta del nodo caído (en el mismo proceso, no hay reinicio de máquina): limpia down y
// hace que Run reinicie su heartbeat y su contexto. El nodo vuelve como pasivo hasta recibir una imagen completa. [ADR-21]
func (n *DataNode) Recover() {
	n.mu.Lock()
	wasDown := n.down
	n.down = false
	n.mu.Unlock()
	if wasDown {
		select {
		case n.recoverCh <- struct{}{}:
		default:
		}
	}
}

// FlushReplication reintenta ahora las replicaciones pendientes (pruebas y recuperación manual). [ADR-10]
func (n *DataNode) FlushReplication(ctx context.Context) { n.retrier.Flush(ctx) }

// PendingReplications devuelve cuántas copias a lectura están pendientes. [ADR-10]
func (n *DataNode) PendingReplications() int { return n.retrier.PendingCount() }

// Close cierra los archivos del nodo.
func (n *DataNode) Close() {
	n.Stop()
	n.primary.Close()
	n.replica.Close()
}

func cacheKey(partition, travelerID, resource string, params map[string]any) string {
	raw, _ := json.Marshal(params) // json ordena las claves de los mapas
	return cacheKeyPrefix(partition, travelerID) + resource + "|" + string(raw)
}

func cacheKeyPrefix(partition, travelerID string) string { return partition + "|" + travelerID + "|" }

// Read lee con caché primero y réplica de lectura ante miss. [ADR-10]
func (n *DataNode) Read(ctx context.Context, travelerID string, q ports.DataQuery) (ports.DataResult, error) {
	if n.isDown() {
		return ports.DataResult{}, ports.ErrDataNodeUnavailable
	}
	op, err := n.catalog.Resolve(q.Resource, operations.Read)
	if err != nil {
		return ports.DataResult{}, err
	}
	partition, err := n.router.PartitionFor(travelerID, op.Scope)
	if err != nil {
		return ports.DataResult{}, err
	}
	key := cacheKey(partition, travelerID, q.Resource, q.Params)
	if res, ok, _ := n.cache.Get(ctx, key); ok {
		return res, nil
	}
	gen := n.guards[partition].generation()
	var res ports.DataResult
	if n.retrier.Pending(partition) {
		// Réplica atrasada en esta partición: se lee de la primaria para no devolver datos viejos. [ADR-10]
		res, err = n.primary.Read(ctx, partition, op, q)
	} else {
		res, err = n.replica.Read(ctx, partition, op, q)
	}
	if err != nil {
		return ports.DataResult{}, err
	}
	n.guards[partition].cacheIfUnchanged(n.cache, ctx, key, res, gen)
	return res, nil
}

// Write escribe en la primaria del shard; el evento E6 replica a lectura e invalida la caché. [ADR-10]
func (n *DataNode) Write(ctx context.Context, travelerID string, cmd ports.DataCommand) (ports.DataResult, error) {
	if n.isDown() {
		return ports.DataResult{}, ports.ErrDataNodeUnavailable
	}
	op, err := n.catalog.Resolve(cmd.Resource, operations.Write)
	if err != nil {
		return ports.DataResult{}, err
	}
	partition, err := n.router.PartitionFor(travelerID, op.Scope)
	if err != nil {
		return ports.DataResult{}, err
	}
	res, rec, version, err := n.primary.Write(ctx, partition, op, cmd)
	if err != nil {
		return ports.DataResult{}, err
	}
	// E6 DatosEscritos → suscriptores internos (invalida caché, luego copia a lectura) y externos.
	// Síncrono: read-your-writes. Si la copia a lectura falla, la primaria ya confirmó: Write responde éxito
	// (un error haría reintentar al cliente y duplicar) y el ReplicationRetrier deja la copia pendiente. [ADR-10]
	e6 := events.DataWritten{IDViajero: travelerID, Entidad: op.Entity, Claves: []string{cacheKeyPrefix(partition, travelerID)}, Version: version}
	for _, sub := range n.internalSubs {
		sub.OnDataWritten(ctx, e6, rec)
	}
	n.mu.RLock()
	subs := append([]func(events.DataWritten){}, n.subscribers...)
	n.mu.RUnlock()
	for _, fn := range subs {
		fn(e6)
	}
	// Con varios workers por partición una lectura lenta pudo leer antes de la copia y quedar por cachear:
	// la guarda sube la generación e invalida otra vez, así esa lectura ya no deja datos viejos en la caché.
	n.guards[partition].invalidate(n.cache, ctx, e6.Claves)
	return res, nil
}

// InstallSnapshot instala en este nodo (pasivo) la imagen recibida del primario. [ADR-21]
func (n *DataNode) InstallSnapshot(_ context.Context, partition, snapshotPath string) error {
	if n.isDown() {
		return ports.ErrDataNodeUnavailable
	}
	if err := n.primary.InstallSnapshot(partition, snapshotPath); err != nil {
		return err
	}
	if err := n.replica.InstallFrom(partition, n.primary.Path(partition)); err != nil {
		return err
	}
	n.retrier.Clear(partition) // la réplica quedó reconstruida desde la primaria: no hay nada pendiente
	n.cache.Flush()
	return nil
}

// SyncNow sincroniza el estado hacia el par y devuelve E5. [ADR-21]
func (n *DataNode) SyncNow(ctx context.Context) (events.StateSync, error) {
	n.mu.RLock()
	peer := n.peer
	n.mu.RUnlock()
	if peer == nil {
		return events.StateSync{}, fmt.Errorf("el nodo %s no tiene par de sincronización", n.id)
	}
	if n.isDown() {
		return events.StateSync{}, ports.ErrDataNodeUnavailable
	}
	return n.syncEmitter.SyncTo(ctx, peer)
}

// Run arranca el heartbeat, el reintento de replicaciones y el bucle de sincronización periódica (solo emite
// cuando es ACTIVO). Bloquea hasta que el contexto termina. Si el nodo se detiene (Stop) espera a Recover
// y vuelve a arrancar. [ADR-21]
func (n *DataNode) Run(ctx context.Context) error {
	select { // descarta una señal vieja de Recover
	case <-n.recoverCh:
	default:
	}
	for {
		n.runOnce(ctx)
		if ctx.Err() != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-n.recoverCh:
		}
	}
}

func (n *DataNode) runOnce(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	n.mu.Lock()
	n.cancel = cancel
	stopped := n.down
	n.mu.Unlock()
	defer cancel()
	if stopped {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = n.heartbeat.Run(ctx) }()
	go func() { defer wg.Done(); n.retrier.Run(ctx, n.retryEvery) }()

	if n.syncInterval > 0 {
		t := time.NewTicker(n.syncInterval)
		defer t.Stop()
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case <-t.C:
				if n.Role() == RoleActive {
					_, _ = n.SyncNow(ctx) // el par puede estar caído: se reintenta en el siguiente ciclo
				}
			}
		}
	} else {
		<-ctx.Done()
	}
	wg.Wait()
}
