// Package replica es el contenedor "Monolito modular x N réplicas" (AD §4.4): ensambla los módulos
// de una réplica stateless. [ADR-02][ADR-08][ADR-16]
package replica

import (
	"context"
	"net/http"
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
	permapp "github.com/nicolas-wai/trotamundos/src/replica/permissions/application"
	"github.com/nicolas-wai/trotamundos/src/replica/priorityqueueclient"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations"
	"github.com/nicolas-wai/trotamundos/src/replica/ssr"
	"github.com/nicolas-wai/trotamundos/src/replica/tokenvalidator"
	taadapters "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/adapters"
	taapp "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
	tadomain "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/updateconsumer"
	"github.com/nicolas-wai/trotamundos/src/stubs"
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
	OAuth        ports.OAuthEmailPort   // proveedor OAuth del correo (nil = MockOAuth2AuthAdapter propio)
	Providers    integrations.Clients   // APIs externas de los adapters (stubs)
	// RunIntegrations marca a la réplica que ejecuta el consumidor de la cola P2P y el planificador de sondeo.
	// La cola P2P admite un único consumidor [ADR-04], por lo que la región designa una sola réplica.
	RunIntegrations bool
}

// Replica ensambla los módulos de una réplica del monolito modular.
type Replica struct {
	ID             string
	Config         config.RegionConfig
	API            http.Handler
	TokenValidator *tokenvalidator.Validator
	SSR            *ssr.Renderer
	QueueClient    *priorityqueueclient.Client
	Recommender    *recommendations.Recommender
	OAuthManager   *oauthmanager.OAuthManager
	Heartbeat      *heartbeat.Emitter
	Consumer       *updateconsumer.UpdateConsumer // nil si la réplica no ejecuta Integraciones
	Integrations   *integrations.Module           // nil si la réplica no ejecuta Integraciones
}

// New arma una réplica. Autenticación (Fase 2), Arreglos de viaje (Fase 3) e Integraciones externas (Fase 4) son
// reales; Permisos y Recomendaciones siguen en modo esqueleto.
func New(id string, cfg config.RegionConfig, deps Dependencies) *Replica {
	queueClient := priorityqueueclient.NewClient(deps.Queue)
	if deps.Clock != nil {
		queueClient.SetClock(deps.Clock)
	}
	renderer := ssr.NewRenderer()
	// Arreglos de viaje [ADR-02][ADR-09]: repositorios sobre la cola de prioridades y registro de tipos [QA-10].
	typeRegistry := tadomain.DefaultArrangementTypeRegistry()
	arrangements := taapp.NewService(
		taadapters.NewQueueTripRepository(queueClient),
		taadapters.NewQueueArrangementRepository(queueClient, typeRegistry),
		typeRegistry, deps.Clock,
	).WithProviderNotices(taadapters.NewQueueProviderNoticeRepository(queueClient)) // avisos E2 [ADR-07]

	// Gestor OAuth del correo [ADR-15]: tokens cifrados con AES-GCM detrás del puerto TokenCipher [QA-08].
	tokenCipher, err := authadapters.NewAESGCMTokenCipher(cfg.TokenEncryptionKey)
	if err != nil {
		panic(err) // la configuración ya fue validada (RegionConfig.Validate)
	}
	var oauthProvider ports.OAuthEmailPort = deps.OAuth
	if oauthProvider == nil {
		oauthProvider = stubs.NewMockOAuth2AuthAdapter()
	}
	oauth := oauthmanager.NewOAuthManager(oauthProvider, authadapters.NewQueueLinkedEmailAccountRepository(queueClient), tokenCipher)
	recommender := recommendations.NewRecommender(queueClient)

	// Autenticación [ADR-14][ADR-16]: repos sobre la cola, hasher bcrypt y tokens JWT.
	travelers := authadapters.NewQueueTravelerRepository(queueClient)
	refreshTokens := authadapters.NewQueueRefreshTokenRepository(queueClient)
	tokens := authadapters.NewJWTTokenServiceAdapter(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, refreshTokens, deps.Clock)
	// El viaje demo se provisiona al registrarse (gestión de viajes mínima ◇) sin acoplar los módulos.
	authService := authapp.NewService(travelers, authadapters.NewBcryptPasswordHasher(deps.PasswordCost), tokens, deps.Clock).
		WithRegistrationHook(arrangements.ProvisionDemoTrip)
	validator := tokenvalidator.NewValidator(tokens)

	controllers := api.NewControllers(api.Dependencies{
		Authentication: authService,
		Profile:        authapp.UnimplementedTravelerProfileService{},
		Arrangements:   arrangements,
		Permissions:    permapp.UnimplementedPermissionsService{},
		Recommender:    recommender,
		EmailLinking:   oauth,
		Renderer:       renderer,
	})
	router := api.NewRouter(controllers, validator)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(ReplicaHeaderName, id)
		router.ServeHTTP(w, r)
	})

	var (
		consumer *updateconsumer.UpdateConsumer
		module   *integrations.Module
	)
	if deps.RunIntegrations && deps.MessageQueue != nil {
		// Integraciones externas [ADR-04..07]: sondeo → circuit breaker → adapters → broker → cola P2P →
		// consumidor → Arreglos de viaje. Los avisos E2 del circuit breaker llegan por el bus de eventos.
		module = integrations.NewModule(integrations.Config{
			PollInterval: cfg.PollInterval, PollTimeout: cfg.ProviderPollTimeout,
			Breaker: circuitbreaker.Config{MaxRetries: cfg.ProviderMaxRetries, Backoff: cfg.ProviderBackoff, Cooldown: cfg.ProviderCooldown},
		}, deps.MessageQueue, deps.EventBus, queueClient, oauth, deps.Providers)
		consumer = updateconsumer.NewUpdateConsumer(deps.MessageQueue, arrangements)
		if deps.EventBus != nil {
			deps.EventBus.Subscribe(events.TopicProviderDegraded, func(ctx context.Context, payload any) {
				if d, ok := payload.(events.ProviderDegraded); ok {
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
		Heartbeat:      heartbeat.NewEmitter(id, deps.Heartbeat, cfg.HeartbeatInterval),
		Consumer:       consumer,
		Integrations:   module,
	}
}

// Handler devuelve el handler HTTP de la réplica.
func (r *Replica) Handler() http.Handler { return r.API }
