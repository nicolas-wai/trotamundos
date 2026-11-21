// Package cache es el componente "Caché" del nodo de datos (AD §4.6).
package cache

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// InMemoryCacheAdapter [ADR-10] Caché en memoria: lectura cache-first, invalidación por clave al escribir.
type InMemoryCacheAdapter struct{}

// NewInMemoryCacheAdapter crea la caché.
func NewInMemoryCacheAdapter() *InMemoryCacheAdapter { return &InMemoryCacheAdapter{} }

// Get devuelve el valor cacheado.
func (*InMemoryCacheAdapter) Get(ctx context.Context, key string) (ports.DataResult, bool, error) {
	return ports.DataResult{}, false, apierror.ErrNotImplemented
}

// Set guarda un valor.
func (*InMemoryCacheAdapter) Set(ctx context.Context, key string, value ports.DataResult) error {
	return apierror.ErrNotImplemented
}

// Invalidate elimina las claves modificadas (E6).
func (*InMemoryCacheAdapter) Invalidate(ctx context.Context, keys ...string) error {
	return apierror.ErrNotImplemented
}
