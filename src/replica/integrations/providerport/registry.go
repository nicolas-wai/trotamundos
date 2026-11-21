// Package providerport es el componente "Puerto único de proveedores" (AD §4.4).
package providerport

import (
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
)

// Registry [ADR-05][QA-10][QA-14] Registro de adapters por tipo de proveedor, único punto por el que entran
// datos externos. Agregar un tipo de proveedor = registrar un adapter.
type Registry struct {
	adapters map[domain.ProviderType]ports.ExternalProviderPort
}

// NewRegistry crea un registro vacío.
func NewRegistry() *Registry {
	return &Registry{adapters: map[domain.ProviderType]ports.ExternalProviderPort{}}
}

// Register asocia un adapter con el tipo de proveedor que declara.
func (r *Registry) Register(adapter ports.ExternalProviderPort) {
	r.adapters[adapter.ProviderType()] = adapter
}

// Adapter devuelve el adapter del tipo indicado.
func (r *Registry) Adapter(t domain.ProviderType) (ports.ExternalProviderPort, bool) {
	a, ok := r.adapters[t]
	return a, ok
}
