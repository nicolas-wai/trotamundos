// Package cache es el componente "Caché" del nodo de datos (AD §4.6).
package cache

import (
	"context"
	"strings"
	"sync"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

// InMemoryCacheAdapter [ADR-10] Caché en memoria: lectura cache-first, invalidación por clave al escribir.
// [Prototipo] Reemplaza a la caché distribuida de producción.
type InMemoryCacheAdapter struct {
	mu      sync.RWMutex
	entries map[string]ports.DataResult
	hits    int
	misses  int
}

// NewInMemoryCacheAdapter crea la caché.
func NewInMemoryCacheAdapter() *InMemoryCacheAdapter {
	return &InMemoryCacheAdapter{entries: map[string]ports.DataResult{}}
}

// Get devuelve el valor cacheado.
func (c *InMemoryCacheAdapter) Get(_ context.Context, key string) (ports.DataResult, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[key]
	if ok {
		c.hits++
	} else {
		c.misses++
	}
	return v, ok, nil
}

// Set guarda un valor.
func (c *InMemoryCacheAdapter) Set(_ context.Context, key string, value ports.DataResult) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = value
	return nil
}

// Invalidate elimina todas las entradas cuya clave empieza por alguno de los prefijos (claves de E6).
func (c *InMemoryCacheAdapter) Invalidate(_ context.Context, prefixes ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		for _, p := range prefixes {
			if strings.HasPrefix(key, p) {
				delete(c.entries, key)
				break
			}
		}
	}
	return nil
}

// Flush vacía la caché (p. ej. al instalar un snapshot de sincronización).
func (c *InMemoryCacheAdapter) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = map[string]ports.DataResult{}
}

// Stats devuelve aciertos y fallos acumulados (útil para pruebas).
func (c *InMemoryCacheAdapter) Stats() (hits, misses int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses
}
