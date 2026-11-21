package adapters

import (
	"context"
	"errors"

	"github.com/nicolas-wai/trotamundos/src/replica/permissions/application"
	"github.com/nicolas-wai/trotamundos/src/replica/permissions/domain"
	taapp "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
)

// ArrangementRepositoryCatalog [ADR-13][ADR-17] Implementa ArrangementCatalog (AD §4.4: PERM → VIA) sobre el
// repositorio de Arreglos de viaje, leyendo en el shard del dueño. Solo expone datos críticos (sin detalle).
type ArrangementRepositoryCatalog struct{ arrangements taapp.ArrangementRepository }

var _ application.ArrangementCatalog = (*ArrangementRepositoryCatalog)(nil)

// NewArrangementRepositoryCatalog crea el adapter.
func NewArrangementRepositoryCatalog(arrangements taapp.ArrangementRepository) *ArrangementRepositoryCatalog {
	return &ArrangementRepositoryCatalog{arrangements: arrangements}
}

// Exists indica si el arreglo existe y es del dueño.
func (c *ArrangementRepositoryCatalog) Exists(ctx context.Context, ownerID, arrangementID string) (bool, error) {
	if _, err := c.arrangements.FindByID(ctx, ownerID, arrangementID); err != nil {
		if errors.Is(err, taapp.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Summaries devuelve los datos críticos de los arreglos pedidos con una sola lectura; los que ya no existen se omiten.
func (c *ArrangementRepositoryCatalog) Summaries(ctx context.Context, ownerID string, ids []string) ([]domain.ArrangementRef, error) {
	refs := []domain.ArrangementRef{}
	if len(ids) == 0 {
		return refs, nil
	}
	all, err := c.arrangements.ListSummariesByTraveler(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	for _, a := range all {
		if wanted[a.ArrangementID] {
			refs = append(refs, domain.ArrangementRef{
				ArrangementID: a.ArrangementID, Type: string(a.Type), Title: a.Title, StartAt: a.StartAt,
				Status: string(a.Status), Origin: string(a.Origin),
			})
		}
	}
	return refs, nil
}
