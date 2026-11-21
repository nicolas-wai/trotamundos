// Package providerport es el componente "Puerto único de proveedores" (AD §4.4).
package providerport

import (
	"errors"
	"fmt"

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

// ClassifyError normaliza un error de transporte del proveedor a los errores del puerto [ADR-05]:
// credencial inválida y respuesta no parseable se conservan; cualquier otra falla (red, timeout, 5xx)
// es ErrProviderUnavailable.
func ClassifyError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ports.ErrInvalidCredential), errors.Is(err, ports.ErrUnparseableResponse), errors.Is(err, ports.ErrProviderUnavailable):
		return err
	}
	return fmt.Errorf("%w: %v", ports.ErrProviderUnavailable, err)
}
