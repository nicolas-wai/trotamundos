// Package ssr es el componente "Renderizador SSR" de la réplica (AD §4.4).
package ssr

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
)

// Renderer [ADR-17][ADR-20] Genera el HTML del dashboard con payload reducido usando html/template (Fase 3).
type Renderer struct{}

// NewRenderer crea el renderizador.
func NewRenderer() *Renderer { return &Renderer{} }

// RenderDashboard devuelve el HTML del dashboard con solo datos críticos.
func (*Renderer) RenderDashboard(ctx context.Context, dashboard application.Dashboard) ([]byte, error) {
	return nil, apierror.ErrNotImplemented
}
