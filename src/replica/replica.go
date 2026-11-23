// Package replica es el contenedor "Monolito modular x N réplicas" (AD §4.4): ensambla los módulos
// de una réplica stateless. [ADR-02][ADR-08][ADR-16]
package replica

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/config"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/api"
	authadapters "github.com/nicolas-wai/trotamundos/src/replica/authentication/adapters"
	authapp "github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	"github.com/nicolas-wai/trotamundos/src/replica/heartbeat"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/circuitbreaker"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/oauthmanager"
	permadapters "github.com/nicolas-wai/trotamundos/src/replica/permissions/adapters"
	permapp "github.com/nicolas-wai/trotamundos/src/replica/permissions/application"
	"github.com/nicolas-wai/trotamundos/src/replica/priorityqueueclient"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations"
	"github.com/nicolas-wai/trotamundos/src/replica/ssr"
	"github.com/nicolas-wai/trotamundos/src/replica/tokenvalidator"
	taadapters "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/adapters"
	taapp "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
	tadomain "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/updateconsumer"
)

// ReplicaHeaderName identifica qué réplica atendió la request (útil para verificar el balanceo).
const ReplicaHeaderName = "X-Replica-Id"

// Dependencies agrupa lo que una réplica recibe de la raíz de composición (ningún estado compartido entre réplicas).
type Dependencies struct {
	Queue        priorityqueueclient.Submitter // nodo de cola de prioridades
	Heartbeat    ports.HeartbeatPort           // canal de heartbeats hacia el balanceador
	Clock        func() time.Time              // reloj (nil = time.Now)
	PasswordCost int                           // costo de bcrypt (0 = por defecto)

	// Infraestructura de la región compartida entre réplicas (Fase 4). Si MessageQueue es nil la réplica no
	// ejecuta Integraciones externas (solo vincula correos con el OAuth por defecto).
	MessageQueue ports.MessageQueuePort // cola P2P [ADR-04]
	EventBus     ports.EventBusPort     // bus de eventos interno (E2) [ADR-07]
	OAuth        ports.OAuthEmailPort   // proveedor OAuth del correo (obligatorio; la región inyecta el adaptador) [ADR-15]
	// ProviderCatalog es el catálogo de proveedores de las recomendaciones (obligatorio; lo inyecta la región). [ADR-25]
	ProviderCatalog ports.ProviderCatalogPort
	Providers       integrations.Clients // APIs externas de los adapters (stubs)
	// ReplicaIDs son los identificadores de todas las réplicas de la región, en orden de prioridad de liderazgo.
	// Todas las réplicas arman el módulo de integraciones, pero solo la líder según el heartbeat ejecuta el
	// consumidor de la cola P2P y el planificador de sondeo [ADR-04][ADR-21]. Vacío = la réplica no participa.
	ReplicaIDs []string
}

// Replica ensambla los módulos de una réplica del monolito modular.
type Replica struct {
	ID             string
	Config         config.RegionConfig
	API            http.Handler
	TokenValidator *tokenvalidator.SignedTokenValidator
	SSR            *ssr.SSRRenderer
	QueueClient    *priorityqueueclient.PriorityQueueClient
	Recommender    *recommendations.Recommender
	OAuthManager   *oauthmanager.OAuthManager
	Heartbeat      *heartbeat.ReplicaHeartbeatEmitter
	Consumer       *updateconsumer.UpdateConsumer // nil si la réplica no participa en Integraciones
	Integrations   *integrations.Module           // nil si la réplica no participa en Integraciones
	// Leader decide si esta réplica ejecuta hoy el consumidor y el planificador. [ADR-04][ADR-21]
	Leader *integrations.IntegrationsLeaderElector
}

// New arma una réplica con todos sus módulos: Autenticación (Fase 2), Arreglos de viaje (Fase 3), Integraciones
// externas (Fase 4), Permisos y Recomendaciones (Fase 5).
// La réplica no crea mocks por su cuenta: OAuth y ProviderCatalog deben inyectarse desde la raíz de composición
// (región); si faltan, New devuelve error en vez de caer en un stub en silencio.
func New(id string, cfg config.RegionConfig, deps Dependencies) (*Replica, error) {
	if deps.OAuth == nil {
		return nil, errors.New("réplica " + id + ": falta Dependencies.OAuth (ports.OAuthEmailPort)")
	}
	if deps.ProviderCatalog == nil {
		return nil, errors.New("réplica " + id + ": falta Dependencies.ProviderCatalog (ports.ProviderCatalogPort)")
	}
	queueClient := priorityqueueclient.NewPriorityQueueClient(deps.Queue)
	if deps.Clock != nil {
		queueClient.SetClock(deps.Clock)
	}
	renderer := ssr.NewSSRRenderer()
	// Arreglos de viaje [ADR-02][ADR-09]: repositorios sobre la cola de prioridades y registro de tipos [QA-10].
	typeRegistry := tadomain.DefaultArrangementTypeRegistry()
	arrangements := taapp.NewService(
		taadapters.NewQueueTripRepository(queueClient),
		taadapters.NewQueueArrangementRepository(queueClient, typeRegistry),
		typeRegistry, deps.Clock,
	).WithProviderNotices(taadapters.NewQueueProviderNoticeRepository(queueClient)) // avisos E2 [ADR-07]

	// Gestor OAuth del correo [ADR-15]: tokens cifrados con AES-GCM detrás del puerto TokenCipherPort [QA-08].
	tokenCipher, err := authadapters.NewAESGCMTokenCipher(cfg.TokenEncryptionKey)
	if err != nil {
		return nil, err
	}
	oauth := oauthmanager.NewOAuthManager(deps.OAuth, authadapters.NewQueueLinkedEmailAccountRepository(queueClient), tokenCipher).WithClock(deps.Clock)

	// Preferencias del perfil [ADR-25]: las usan el perfil (PUT /perfil/preferencias), el recolector y HITL.
	preferences := authadapters.NewQueuePreferencesRepository(queueClient)
	// Recomendaciones [ADR-23][ADR-24][ADR-25]: recolector concurrente → motor de filtrado → HITL.
	recommender := recommendations.NewRecommender(queueClient, preferences, deps.ProviderCatalog)

	// Control de permisos [ADR-13][QA-09]: itinerarios en el shard del dueño, resuelto por el directorio ◇ [ADR-11].
	// Consulta Arreglos de viaje por su repositorio (AD §4.4: PERM → VIA) y publica E7 en el bus de eventos.
	permissions := permapp.NewService(
		permadapters.NewQueueSharedItineraryRepository(queueClient, permadapters.NewQueueItineraryDirectory(queueClient)),
		permadapters.NewArrangementRepositoryCatalog(taadapters.NewQueueArrangementRepository(queueClient, typeRegistry)),
		deps.Clock,
	)
	if deps.EventBus != nil {
		permissions.WithEventBus(deps.EventBus)
	}

	// Autenticación [ADR-14][ADR-16]: repos sobre la cola, hasher bcrypt y tokens JWT.
	travelers := authadapters.NewQueueTravelerRepository(queueClient)
	refreshTokens := authadapters.NewQueueRefreshTokenRepository(queueClient)
	tokens := authadapters.NewJWTTokenServiceAdapter(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, refreshTokens, deps.Clock)
	// El viaje demo se provisiona al registrarse (gestión de viajes mínima ◇) sin acoplar los módulos.
	authService := authapp.NewService(travelers, authadapters.NewBcryptPasswordHasher(deps.PasswordCost), tokens, deps.Clock).
		WithRegistrationHook(arrangements.ProvisionDemoTrip)
	validator := tokenvalidator.NewSignedTokenValidator(tokens)

	controllers := api.NewControllers(api.Dependencies{
		Authentication: authService,
		Profile:        authapp.NewProfileService(preferences),
		Arrangements:   arrangements,
		Permissions:    permissions,
		Recommender:    recommender,
		EmailLinking:   oauth,
		Renderer:       renderer,

		RecommendationTimeout: cfg.RecommendationTimeout,
	})
	router := api.NewRouter(controllers, validator)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(ReplicaHeaderName, id)
		router.ServeHTTP(w, r)
	})

	var (
		consumer *updateconsumer.UpdateConsumer
		module   *integrations.Module
		leader   *integrations.IntegrationsLeaderElector
	)
	if len(deps.ReplicaIDs) > 0 && deps.MessageQueue != nil {
		// Integraciones externas [ADR-04..07]: sondeo → circuit breaker → adapters → broker → cola P2P →
		// consumidor → Arreglos de viaje. Los avisos E2 del circuit breaker llegan por el bus de eventos.
		module = integrations.NewModule(integrations.Config{
			PollInterval: cfg.PollInterval, PollTimeout: cfg.ProviderPollTimeout,
			Breaker: circuitbreaker.Config{MaxRetries: cfg.ProviderMaxRetries, Backoff: cfg.ProviderBackoff, Cooldown: cfg.ProviderCooldown},
		}, deps.MessageQueue, deps.EventBus, queueClient, oauth, deps.Providers)
		consumer = updateconsumer.NewUpdateConsumer(deps.MessageQueue, arrangements)
		leader = integrations.NewIntegrationsLeaderElector(id, deps.ReplicaIDs, deps.Heartbeat, cfg.HeartbeatInterval)
		if deps.EventBus != nil {
			// Todas las réplicas escuchan E2, pero solo la líder lo registra (una sola vez por región): así los
			// avisos siguen llegando al dashboard después de un relevo. [ADR-07]
			deps.EventBus.Subscribe(events.TopicProviderDegraded, func(ctx context.Context, payload any) {
				if d, ok := payload.(events.ProviderDegraded); ok && leader.IsLeader() {
					_ = arrangements.ReportProviderDegraded(ctx, d)
				}
			})
		}
	}

	return &Replica{
		ID:             id,
		Config:         cfg,
		API:            handler,
		TokenValidator: validator,
		SSR:            renderer,
		QueueClient:    queueClient,
		Recommender:    recommender,
		OAuthManager:   oauth,
		Heartbeat:      heartbeat.NewReplicaHeartbeatEmitter(id, deps.Heartbeat, cfg.HeartbeatInterval),
		Consumer:       consumer,
		Integrations:   module,
		Leader:         leader,
	}, nil
}

// RunIntegrations ejecuta el consumidor de la cola P2P y el planificador de sondeo mientras esta réplica sea la
// líder; al dejar de serlo (o caer) cancela ambos. Bloquea hasta que ctx termina. [ADR-04][ADR-21]
// ctx debe ser el contexto de la réplica (no el raíz de la región) para que su caída detenga el liderazgo.
func (r *Replica) RunIntegrations(ctx context.Context, polling bool) {
	if r.Leader == nil {
		return
	}
	retry := r.Config.HeartbeatInterval
	r.Leader.Run(ctx, func(lctx context.Context) {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); integrations.RunWithRetry(lctx, retry, r.Consumer.Run) }() // único consumidor [ADR-04]
		if polling {
			wg.Add(1)
			go func() { defer wg.Done(); _ = r.Integrations.Scheduler.Run(lctx) }() // sondeo cada <= 2 min [ADR-05]
		}
		wg.Wait()
	})
}

// Handler devuelve el handler HTTP de la réplica.
func (r *Replica) Handler() http.Handler { return r.API }
