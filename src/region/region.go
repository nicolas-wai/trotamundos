// Package region es la raíz de composición de una región completa (AD §3.1): LoadBalancer → N réplicas →
// cola de prioridades → nodo de datos primario y pasivo, con sus monitores de heartbeat.
// [ADR-08] N réplicas stateless detrás del LoadBalancer. [ADR-21] El sistema completo se replica por región. [QA-13] Una región se describe solo por RegionConfig.
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
	"github.com/nicolas-wai/trotamundos/src/messagequeue"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/priorityqueue"
	"github.com/nicolas-wai/trotamundos/src/replica"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations"
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

// ExternalAPIs agrupa los sistemas externos simulados que la región inyecta detrás de los puertos. [Prototipo]
type ExternalAPIs struct {
	OAuth        *stubs.MockOAuth2AuthAdapter
	Email        *stubs.StubEmailProviderAPI
	Agency       *stubs.StubAgencyAPI
	TravelSystem *stubs.StubTravelSystemAPI
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

	// Mensajería e integraciones de la región (Fase 4). La cola P2P tiene un único consumidor [ADR-04]: la
	// réplica 0 ejecuta el consumidor y el planificador de sondeo; el resto solo atiende la API.
	MessageQueue *messagequeue.InMemoryMessageQueue // cola P2P [ADR-04]
	EventBus     *messagequeue.InMemoryEventBus     // bus interno, E2 [ADR-07]
	ExternalAPIs ExternalAPIs                       // sistemas externos simulados [Prototipo]

	// Itinerarios compartidos y recomendaciones (Fase 5). El notificador recibe E7 por el bus (una sola vez por
	// región, no por réplica) y el catálogo admite fallas/demoras inyectables para probar ADR-23.
	OwnerNotifier   *stubs.InMemoryOwnerNotifier
	ProviderCatalog *stubs.StubProviderCatalog

	mu             sync.Mutex
	swaps          []events.ResponsibilitySwap
	rejoins        []events.PassiveRejoined
	alerts         []string
	runCtx         context.Context
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
			Heartbeat: r.DataHeartbeats, HeartbeatInterval: cfg.HeartbeatInterval, SyncInterval: cfg.HeartbeatInterval, ReplicationRetryInterval: cfg.HeartbeatInterval,
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
	r.Switch.OnRejoin(func(e4b events.PassiveRejoined) { // E4b: el nodo caído volvió como pasivo [ADR-21]
		r.mu.Lock()
		r.rejoins = append(r.rejoins, e4b)
		r.mu.Unlock()
	})
	monitor := priorityqueue.NewDataHeartbeatMonitor(r.DataHeartbeats, r.Switch, cfg.HeartbeatInterval)
	monitor.OnAlert(func(msg string) { // aviso registrado (p. ej. ambos nodos caídos) [ADR-22]
		r.mu.Lock()
		r.alerts = append(r.alerts, msg)
		r.mu.Unlock()
	})
	r.Queue = priorityqueue.NewPriorityQueueNode(r.Switch, monitor)

	var submitter priorityqueueclient.Submitter = r.Queue
	if opts.OnQueuedOperation != nil {
		submitter = observedQueue{inner: r.Queue, observe: opts.OnQueuedOperation}
	}
	r.MessageQueue = messagequeue.NewInMemoryMessageQueue()
	r.EventBus = messagequeue.NewInMemoryEventBus()
	r.ExternalAPIs = ExternalAPIs{
		OAuth: stubs.NewMockOAuth2AuthAdapter().WithClock(opts.Clock), Email: stubs.NewStubEmailProviderAPI(),
		Agency: stubs.NewStubAgencyAPI(), TravelSystem: stubs.NewStubTravelSystemAPI(),
	}
	r.OwnerNotifier = stubs.NewInMemoryOwnerNotifier()
	r.ProviderCatalog = stubs.NewStubProviderCatalog()
	// E7 SolicitudAccesoCreada: Control de permisos publica en el bus y el notificador del dueño lo registra. [ADR-13]
	r.EventBus.Subscribe(events.TopicAccessRequestCreated, func(ctx context.Context, payload any) {
		if e7, ok := payload.(events.AccessRequestCreated); ok {
			_ = r.OwnerNotifier.NotifyAccessRequest(ctx, e7)
		}
	})
	replicaIDs := make([]string, cfg.ReplicaCount)
	for i := range replicaIDs {
		replicaIDs[i] = ReplicaID(cfg, i)
	}
	for i := 0; i < cfg.ReplicaCount; i++ {
		id := replicaIDs[i]
		rep, err := replica.New(id, cfg, replica.Dependencies{
			Queue: submitter, Heartbeat: r.ReplicaHeartbeats, Clock: opts.Clock, PasswordCost: opts.PasswordCost,
			MessageQueue: r.MessageQueue, EventBus: r.EventBus, OAuth: r.ExternalAPIs.OAuth, ProviderCatalog: r.ProviderCatalog,
			Providers:  integrations.Clients{Mail: r.ExternalAPIs.Email, Agency: r.ExternalAPIs.Agency, TravelSystem: r.ExternalAPIs.TravelSystem},
			ReplicaIDs: replicaIDs,
		})
		if err != nil {
			r.ActiveNode.Close()
			r.PassiveNode.Close()
			return nil, err
		}
		r.Replicas = append(r.Replicas, rep)
		r.ReplicaHeartbeats.Track(id)
	}
	return r, nil
}

// ReplicaID devuelve el identificador de la réplica i-ésima (base 0).
func ReplicaID(cfg config.RegionConfig, i int) string {
	return fmt.Sprintf("%s-replica-%d", cfg.RegionID, i+1)
}

// NewLoadBalancer crea el balanceador sobre las URLs de las réplicas (en el mismo orden) y su monitor de heartbeats.
func (r *Region) NewLoadBalancer(urls []*url.URL, opts ...loadbalancer.Option) *loadbalancer.LoadBalancer {
	backends := make([]loadbalancer.Backend, len(urls))
	for i, u := range urls {
		backends[i] = loadbalancer.Backend{ID: r.Replicas[i].ID, URL: u}
	}
	r.LB = loadbalancer.NewLoadBalancer(backends, opts...)
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
	r.runCtx = ctx
	r.replicaCancels = make([]context.CancelFunc, len(r.Replicas))
	for i := range r.Replicas {
		r.startReplica(i)
	}
	if r.LBMonitor != nil {
		run(r.LBMonitor.Run, ctx)
	}
}

// startReplica lanza los procesos de la réplica i atados a su propio contexto: heartbeat y, si es la líder,
// consumidor y planificador [ADR-04][ADR-21]. Cancelar ese contexto simula su caída.
func (r *Region) startReplica(i int) {
	rep := r.Replicas[i]
	rctx, cancel := context.WithCancel(r.runCtx)
	r.mu.Lock()
	r.replicaCancels[i] = cancel
	r.mu.Unlock()
	r.wg.Add(1)
	go func() { defer r.wg.Done(); _ = rep.Heartbeat.Run(rctx) }()
	if rep.Leader != nil {
		r.wg.Add(1)
		go func() { defer r.wg.Done(); rep.RunIntegrations(rctx, r.Config.PollingEnabled) }()
	}
}

// StopReplica simula la caída de la réplica i: deja de emitir heartbeats y detiene su consumidor y planificador
// si los tenía. [QA-04][ADR-21]
func (r *Region) StopReplica(i int) {
	r.mu.Lock()
	cancel := r.replicaCancels[i]
	r.mu.Unlock()
	cancel()
}

// RecoverReplica simula que la réplica i vuelve: retoma heartbeats y, si le corresponde, el liderazgo. Solo tiene
// sentido tras StopReplica. [ADR-21]
func (r *Region) RecoverReplica(i int) { r.startReplica(i) }

// Swaps devuelve los eventos E4 emitidos hasta ahora.
func (r *Region) Swaps() []events.ResponsibilitySwap {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]events.ResponsibilitySwap(nil), r.swaps...)
}

// Rejoins devuelve los eventos E4b de reincorporación del pasivo emitidos hasta ahora. [ADR-21]
func (r *Region) Rejoins() []events.PassiveRejoined {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]events.PassiveRejoined(nil), r.rejoins...)
}

// DataAlerts devuelve los avisos del monitor de nodos de datos (ambos caídos, pasivo sin sincronizar). [ADR-22]
func (r *Region) DataAlerts() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.alerts...)
}

// Close espera a los procesos de fondo (el contexto de Start ya debe estar cancelado) y cierra los archivos.
func (r *Region) Close() {
	r.wg.Wait()
	r.ActiveNode.Close()
	r.PassiveNode.Close()
}
