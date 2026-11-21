// Package region es la raíz de composición de una región completa (AD §3.1): LoadBalancer → N réplicas →
// cola de prioridades → nodo de datos primario y pasivo, con sus monitores de heartbeat.
// [ADR-21] El sistema completo se replica por región. [QA-13] Una región se describe solo por RegionConfig.
// La usan cmd/trotamundos y las pruebas de integración (así no se duplica la composición).
package region

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/config"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/datanode"
	"github.com/nicolas-wai/trotamundos/src/loadbalancer"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/priorityqueue"
	"github.com/nicolas-wai/trotamundos/src/replica"
	"github.com/nicolas-wai/trotamundos/src/replica/priorityqueueclient"
	"github.com/nicolas-wai/trotamundos/src/stubs"
)

// Options son los ajustes de composición que no pertenecen a RegionConfig.
type Options struct {
	Clock        func() time.Time // reloj de tokens (nil = time.Now); útil en pruebas
	PasswordCost int              // costo de bcrypt (0 = por defecto)
	// OnQueuedOperation espía cada operación E8 que las réplicas entregan al nodo de cola de prioridades
	// (pruebas: verificar que lecturas y escrituras entran con su tipo). nil = sin espía.
	OnQueuedOperation func(events.QueuedOperation)
}

// observedQueue decora el nodo de cola con un observador de las operaciones admitidas.
type observedQueue struct {
	inner   priorityqueueclient.Submitter
	observe func(events.QueuedOperation)
}

func (o observedQueue) Submit(ctx context.Context, op events.QueuedOperation) (ports.Future, error) {
	o.observe(op)
	return o.inner.Submit(ctx, op)
}

// Region agrupa todos los contenedores de una región.
type Region struct {
	Config config.RegionConfig

	Replicas    []*replica.Replica
	ActiveNode  *datanode.DataNode // primario al arrancar
	PassiveNode *datanode.DataNode // pasivo al arrancar
	Queue       *priorityqueue.PriorityQueueNode
	Switch      *priorityqueue.DataNodeSwitch

	ReplicaHeartbeats *stubs.InMemoryHeartbeatChannel // réplicas → LB [ADR-22]
	DataHeartbeats    *stubs.InMemoryHeartbeatChannel // nodos de datos → cola [ADR-22]

	LB        *loadbalancer.LoadBalancer
	LBMonitor *loadbalancer.ReplicaHeartbeatMonitor

	mu             sync.Mutex
	swaps          []events.ResponsibilitySwap
	replicaCancels []context.CancelFunc
	wg             sync.WaitGroup
}

// New arma la región (sin arrancar goroutines ni el balanceador, que necesita las URLs de las réplicas).
func New(cfg config.RegionConfig, opts Options) (*Region, error) {
	r := &Region{Config: cfg}
	r.ReplicaHeartbeats = stubs.NewInMemoryHeartbeatChannel(cfg.HeartbeatInterval, cfg.HeartbeatMissThreshold)
	r.DataHeartbeats = stubs.NewInMemoryHeartbeatChannel(cfg.HeartbeatInterval, cfg.HeartbeatMissThreshold)

	newNode := func(suffix string, role datanode.Role) (*datanode.DataNode, error) {
		id := fmt.Sprintf("%s-data-%s", cfg.RegionID, suffix)
		return datanode.NewDataNode(datanode.Config{
			ID: id, Role: role, Dir: filepath.Join(cfg.DataDir, id), Shards: cfg.ShardCount,
			Heartbeat: r.DataHeartbeats, HeartbeatInterval: cfg.HeartbeatInterval, SyncInterval: cfg.HeartbeatInterval,
		})
	}
	var err error
	if r.ActiveNode, err = newNode("1", datanode.RoleActive); err != nil {
		return nil, err
	}
	if r.PassiveNode, err = newNode("2", datanode.RolePassive); err != nil {
		r.ActiveNode.Close()
		return nil, err
	}
	r.ActiveNode.SetPeer(r.PassiveNode)
	r.PassiveNode.SetPeer(r.ActiveNode)
	r.DataHeartbeats.Track(r.ActiveNode.ID(), r.PassiveNode.ID())

	r.Switch = priorityqueue.NewDataNodeSwitch(cfg.RegionID, r.ActiveNode, r.PassiveNode)
	r.Switch.OnSwap(func(e4 events.ResponsibilitySwap) {
		r.mu.Lock()
		r.swaps = append(r.swaps, e4)
		r.mu.Unlock()
	})
	monitor := priorityqueue.NewDataHeartbeatMonitor(r.DataHeartbeats, r.Switch, cfg.HeartbeatInterval)
	r.Queue = priorityqueue.NewPriorityQueueNode(r.Switch, monitor)

	var submitter priorityqueueclient.Submitter = r.Queue
	if opts.OnQueuedOperation != nil {
		submitter = observedQueue{inner: r.Queue, observe: opts.OnQueuedOperation}
	}
	for i := 0; i < cfg.ReplicaCount; i++ {
		id := ReplicaID(cfg, i)
		r.Replicas = append(r.Replicas, replica.New(id, cfg, replica.Dependencies{
			Queue: submitter, Heartbeat: r.ReplicaHeartbeats, Clock: opts.Clock, PasswordCost: opts.PasswordCost,
		}))
		r.ReplicaHeartbeats.Track(id)
	}
	return r, nil
}

// ReplicaID devuelve el identificador de la réplica i-ésima (base 0).
func ReplicaID(cfg config.RegionConfig, i int) string {
	return fmt.Sprintf("%s-replica-%d", cfg.RegionID, i+1)
}

// NewLoadBalancer crea el balanceador sobre las URLs de las réplicas (en el mismo orden) y su monitor de heartbeats.
func (r *Region) NewLoadBalancer(urls []*url.URL) *loadbalancer.LoadBalancer {
	backends := make([]loadbalancer.Backend, len(urls))
	for i, u := range urls {
		backends[i] = loadbalancer.Backend{ID: r.Replicas[i].ID, URL: u}
	}
	r.LB = loadbalancer.NewLoadBalancer(backends)
	r.LBMonitor = loadbalancer.NewReplicaHeartbeatMonitor(r.ReplicaHeartbeats, r.LB, r.Config.HeartbeatInterval)
	return r.LB
}

// Start lanza los procesos de fondo (cola, nodos de datos, heartbeats, monitores). No bloquea.
func (r *Region) Start(ctx context.Context) {
	run := func(f func(context.Context) error, c context.Context) {
		r.wg.Add(1)
		go func() { defer r.wg.Done(); _ = f(c) }()
	}
	run(r.Queue.Run, ctx)
	run(r.ActiveNode.Run, ctx)
	run(r.PassiveNode.Run, ctx)
	for _, rep := range r.Replicas {
		rctx, cancel := context.WithCancel(ctx)
		r.replicaCancels = append(r.replicaCancels, cancel)
		run(rep.Heartbeat.Run, rctx)
	}
	if r.LBMonitor != nil {
		run(r.LBMonitor.Run, ctx)
	}
}

// StopReplica simula la caída de la réplica i: deja de emitir heartbeats. [QA-04]
func (r *Region) StopReplica(i int) { r.replicaCancels[i]() }

// Swaps devuelve los eventos E4 emitidos hasta ahora.
func (r *Region) Swaps() []events.ResponsibilitySwap {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]events.ResponsibilitySwap(nil), r.swaps...)
}

// Close espera a los procesos de fondo (el contexto de Start ya debe estar cancelado) y cierra los archivos.
func (r *Region) Close() {
	r.wg.Wait()
	r.ActiveNode.Close()
	r.PassiveNode.Close()
}
