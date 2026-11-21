package ports

import (
	"context"

	recommendations "github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
)

// ProviderCatalogPort [ADR-25] Catálogo de ítems de proveedores (alojamiento, transporte y ruta) con metadatos
// que el motor de filtrado basado en contenido puntúa. Sistema externo: en el prototipo, StubProviderCatalog.
type ProviderCatalogPort interface {
	List(ctx context.Context) ([]recommendations.CatalogItem, error)
}
