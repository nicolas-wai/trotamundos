// Package engine es el componente "Motor de filtrado basado en contenido" de Recomendaciones (AD §4.4).
package engine

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
)

// ContentBasedFilteringEngine [ADR-25] Filtra y puntúa el catálogo de proveedores por similitud de metadatos
// con el historial y las preferencias del viajero. Lógica en la Fase 5.
type ContentBasedFilteringEngine struct{}

// NewContentBasedFilteringEngine crea el motor.
func NewContentBasedFilteringEngine() *ContentBasedFilteringEngine {
	return &ContentBasedFilteringEngine{}
}

// Filter devuelve las recomendaciones ordenadas para el historial dado.
func (*ContentBasedFilteringEngine) Filter(ctx context.Context, history ports.TravelerHistory) ([]domain.Recommendation, error) {
	return nil, apierror.ErrNotImplemented
}
