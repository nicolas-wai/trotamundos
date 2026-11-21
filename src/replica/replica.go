// Package replica es el contenedor "Monolito modular x N réplicas" (AD §4.4): ensambla los módulos
// de una réplica stateless. [ADR-02][ADR-08][ADR-16]
package replica

import (
	"net/http"

	"github.com/nicolas-wai/trotamundos/src/config"
	"github.com/nicolas-wai/trotamundos/src/replica/api"
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

// New arma una réplica con servicios de aplicación en modo esqueleto (Fase 1).
func New(id string, cfg config.RegionConfig) *Replica {
	queueClient := priorityqueueclient.NewClient()
	renderer := ssr.NewRenderer()
	arrangements := taapp.UnimplementedTravelArrangementsService{}
	oauth := oauthmanager.NewOAuthManager(stubs.NewMockOAuth2AuthAdapter())
	recommender := recommendations.NewRecommender(queueClient)

	controllers := api.NewControllers(api.Dependencies{
		Authentication: authapp.UnimplementedAuthenticationService{},
		Profile:        authapp.UnimplementedTravelerProfileService{},
		Arrangements:   arrangements,
		Permissions:    permapp.UnimplementedPermissionsService{},
		Recommender:    recommender,
		EmailLinking:   oauth,
		Renderer:       renderer,
	})

	return &Replica{
		ID:             id,
		Config:         cfg,
		API:            api.NewRouter(controllers),
		TokenValidator: tokenvalidator.NewValidator(nil),
		SSR:            renderer,
		QueueClient:    queueClient,
		Recommender:    recommender,
		OAuthManager:   oauth,
		Heartbeat:      heartbeat.NewEmitter(id, nil, cfg.HeartbeatInterval),
		Consumer:       updateconsumer.NewUpdateConsumer(nil, arrangements),
	}
}

// Handler devuelve el handler HTTP de la réplica.
func (r *Replica) Handler() http.Handler { return r.API }
