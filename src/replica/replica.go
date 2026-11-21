// Package replica es el contenedor "Monolito modular x N réplicas" (AD §4.4): ensambla los módulos
// de una réplica stateless. [ADR-02][ADR-08][ADR-16]
package replica

import (
	"net/http"
	"time"

	"github.com/nicolas-wai/trotamundos/src/config"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/api"
	authadapters "github.com/nicolas-wai/trotamundos/src/replica/authentication/adapters"
	authapp "github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	"github.com/nicolas-wai/trotamundos/src/replica/heartbeat"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/oauthmanager"
	permapp "github.com/nicolas-wai/trotamundos/src/replica/permissions/application"
	"github.com/nicolas-wai/trotamundos/src/replica/priorityqueueclient"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations"
	"github.com/nicolas-wai/trotamundos/src/replica/ssr"
	"github.com/nicolas-wai/trotamundos/src/replica/tokenvalidator"
	taapp "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
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
	Consumer       *updateconsumer.UpdateConsumer
}

// New arma una réplica. Autenticación es real (Fase 2); los demás módulos siguen en modo esqueleto.
func New(id string, cfg config.RegionConfig, deps Dependencies) *Replica {
	queueClient := priorityqueueclient.NewClient(deps.Queue)
	if deps.Clock != nil {
		queueClient.SetClock(deps.Clock)
	}
	renderer := ssr.NewRenderer()
	arrangements := taapp.UnimplementedTravelArrangementsService{}
	oauth := oauthmanager.NewOAuthManager(stubs.NewMockOAuth2AuthAdapter())
	recommender := recommendations.NewRecommender(queueClient)

	// Autenticación [ADR-14][ADR-16]: repos sobre la cola, hasher bcrypt y tokens JWT.
	travelers := authadapters.NewQueueTravelerRepository(queueClient)
	refreshTokens := authadapters.NewQueueRefreshTokenRepository(queueClient)
	tokens := authadapters.NewJWTTokenServiceAdapter(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, refreshTokens, deps.Clock)
	authService := authapp.NewService(travelers, authadapters.NewBcryptPasswordHasher(deps.PasswordCost), tokens, deps.Clock)
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
		Consumer:       updateconsumer.NewUpdateConsumer(nil, arrangements),
	}
}

// Handler devuelve el handler HTTP de la réplica.
func (r *Replica) Handler() http.Handler { return r.API }
