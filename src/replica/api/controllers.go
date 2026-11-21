// Package api es el componente "API y controladores" de la réplica (AD §4.4): único punto de entrada
// de los clientes. [ADR-01][ADR-02] Autentica (tokenvalidator) y enruta a los módulos.
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
	authapp "github.com/nicolas-wai/trotamundos/src/replica/authentication/application"
	authdomain "github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
	permapp "github.com/nicolas-wai/trotamundos/src/replica/permissions/application"
	"github.com/nicolas-wai/trotamundos/src/replica/ssr"
	taapp "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
)

// EmailLinkingService es el servicio que consume la API para vincular correos (lo implementa oauthmanager.OAuthManager).
// [ADR-15]
type EmailLinkingService interface {
	Link(ctx context.Context, travelerID, provider, authorizationCode, redirectURI string) (authdomain.LinkedEmailAccount, error)
	Unlink(ctx context.Context, travelerID, accountID string) error
}

// Dependencies agrupa los módulos que la API enruta (inyectados por la raíz de composición).
type Dependencies struct {
	Authentication authapp.AuthenticationService
	Profile        authapp.TravelerProfileService
	Arrangements   taapp.TravelArrangementsService
	Permissions    permapp.PermissionsService
	Recommender    ports.RecommenderPort
	EmailLinking   EmailLinkingService
	Renderer       *ssr.Renderer
}

// Controllers contiene los handlers HTTP de todas las rutas de AD §6.2.
type Controllers struct {
	deps Dependencies
}

// NewControllers crea los controladores.
func NewControllers(deps Dependencies) *Controllers { return &Controllers{deps: deps} }

// travelerIDFrom devuelve el idViajero del contexto. [ADR-11] Siempre sale del token, nunca del body.
// Fase 2: lo inyecta tokenvalidator.
func travelerIDFrom(context.Context) string { return "" }

// respondSkeleton cierra un handler del esqueleto de la Fase 1. Los errores de los servicios se traducen
// al formato de AD §6.1; la serialización de respuestas exitosas llega en la fase de cada módulo.
func respondSkeleton(w http.ResponseWriter, err error) {
	switch {
	case err == nil, errors.Is(err, apierror.ErrNotImplemented):
		apierror.WriteNotImplemented(w)
	default:
		apierror.WriteError(w, http.StatusInternalServerError, apierror.ErrorInterno, "Error interno")
	}
}
