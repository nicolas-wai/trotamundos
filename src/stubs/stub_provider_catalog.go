package stubs

import (
	"context"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
	recommendations "github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
)

// StubProviderCatalog [ADR-25] Catálogo determinístico de ítems de alojamiento, transporte y ruta
// con metadatos (precio, destino, estrellas…) que consume el motor de filtrado. [Prototipo]
// Admite fallas y demoras inyectables para probar el panel asíncrono (ADR-23).
type StubProviderCatalog struct {
	mu    sync.Mutex
	err   error
	delay time.Duration
}

var _ ports.ProviderCatalogPort = (*StubProviderCatalog)(nil)

// NewStubProviderCatalog crea el catálogo.
func NewStubProviderCatalog() *StubProviderCatalog { return &StubProviderCatalog{} }

// SetFailure hace que List falle con err (nil = sin falla).
func (c *StubProviderCatalog) SetFailure(err error) {
	c.mu.Lock()
	c.err = err
	c.mu.Unlock()
}

// SetDelay hace que List demore (respeta la cancelación del contexto).
func (c *StubProviderCatalog) SetDelay(d time.Duration) {
	c.mu.Lock()
	c.delay = d
	c.mu.Unlock()
}

// List devuelve el catálogo, siempre en el mismo orden.
func (c *StubProviderCatalog) List(ctx context.Context) ([]recommendations.CatalogItem, error) {
	c.mu.Lock()
	err, delay := c.err, c.delay
	c.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}
	return catalogItems(), nil
}

func item(id string, category recommendations.Category, title, provider, destination string, price float64, extra map[string]any) recommendations.CatalogItem {
	meta := map[string]any{"destino": destination, "precio": price, "moneda": "USD"}
	for k, v := range extra {
		meta[k] = v
	}
	return recommendations.CatalogItem{ItemID: id, Category: category, Title: title, Provider: provider, Metadata: meta}
}

func catalogItems() []recommendations.CatalogItem {
	const (
		stay  = recommendations.CategoryAccommodation
		move  = recommendations.CategoryTransport
		route = recommendations.CategoryRoute
	)
	return []recommendations.CatalogItem{
		item("alo-hostal-madrid", stay, "Hostal Gran Vía", "HostelNet", "Madrid", 45, map[string]any{"estrellas": 2}),
		item("alo-hotel-lisboa-centro", stay, "Hotel Baixa Lisboa", "StayCo", "Lisboa", 95, map[string]any{"estrellas": 4}),
		item("alo-apart-paris", stay, "Apartamento Le Marais", "StayCo", "París", 135, map[string]any{"estrellas": 4}),
		item("alo-hotel-roma", stay, "Hotel Colosseo", "StayCo", "Roma", 150, map[string]any{"estrellas": 4}),
		item("alo-hotel-lisboa-plaza", stay, "Lisboa Plaza Grand", "LuxStay", "Lisboa", 180, map[string]any{"estrellas": 5}),
		item("tra-bus-lisboa-oporto", move, "Bus Lisboa–Oporto", "BusLine", "Oporto", 25, map[string]any{"medio": "BUS"}),
		item("tra-tren-madrid-barcelona", move, "Tren AVE Madrid–Barcelona", "RailEurope", "Barcelona", 60, map[string]any{"medio": "TREN"}),
		item("tra-auto-roma", move, "Auto compacto en Roma", "RentaYa", "Roma", 110, map[string]any{"medio": "AUTO"}),
		item("tra-vuelo-madrid-lisboa", move, "Vuelo Madrid–Lisboa", "Sol Air", "Lisboa", 210, map[string]any{"medio": "VUELO"}),
		item("rut-alfama", route, "Ruta a pie por Alfama", "WalkTours", "Lisboa", 30, map[string]any{"duracionHoras": 3}),
		item("rut-vaticano", route, "Ruta Vaticano y Trastevere", "WalkTours", "Roma", 55, map[string]any{"duracionHoras": 5}),
		item("rut-costa-vicentina", route, "Ruta Costa Vicentina en auto", "RoadTrips", "Portugal", 240, map[string]any{"duracionHoras": 48}),
	}
}
