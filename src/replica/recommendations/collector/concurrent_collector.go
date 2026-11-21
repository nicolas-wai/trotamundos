// Package collector es el componente "Recolector concurrente" de Recomendaciones (AD §4.4).
package collector

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// ConcurrentCollector [ADR-24] Consulta en paralelo historial (varias tablas) y perfil, cada lectura
// por la cola de prioridades, y unifica el resultado. Lógica en la Fase 5.
type ConcurrentCollector struct {
	queue ports.PriorityQueuePort
}

var _ ports.HistoryPort = (*ConcurrentCollector)(nil)

// NewConcurrentCollector crea el recolector.
func NewConcurrentCollector(queue ports.PriorityQueuePort) *ConcurrentCollector {
	return &ConcurrentCollector{queue: queue}
}

// Get recolecta el historial unificado del viajero.
func (*ConcurrentCollector) Get(context.Context, string) (ports.TravelerHistory, error) {
	return ports.TravelerHistory{}, apierror.ErrNotImplemented
}
