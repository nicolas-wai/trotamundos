package ports

import (
	"context"

	permissions "github.com/nicolas-wai/trotamundos/src/replica/permissions/domain"
)

// AccessDecision es la decisión del dueño sobre una solicitud.
type AccessDecision string

// Decisiones del contrato (AD §6.2).
const (
	DecisionAccept AccessDecision = "ACEPTAR"
	DecisionReject AccessDecision = "RECHAZAR"
)

// ItineraryPermissionsPort [ADR-13] Permisos de itinerarios compartidos.
// Todo acceso nuevo requiere la decisión explícita del dueño (QA09).
type ItineraryPermissionsPort interface {
	CanView(ctx context.Context, travelerID, itineraryID string) (bool, error)
	RequestAccess(ctx context.Context, travelerID, itineraryID string) (permissions.AccessRequest, error)
	Decide(ctx context.Context, ownerID, requestID string, decision AccessDecision) error
	Revoke(ctx context.Context, ownerID, itineraryID, travelerID string) error
}
