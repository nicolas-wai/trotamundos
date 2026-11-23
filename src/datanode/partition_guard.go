package datanode

import (
	"context"
	"sync"

	"github.com/nicolas-wai/trotamundos/src/datanode/cache"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// partitionGuard [ADR-09][ADR-10] Guarda por partición: con workers de lectura concurrentes a la escritura, una
// lectura que empezó antes de una escritura no debe dejar su resultado viejo en la caché. La lectura anota la
// generación al empezar y solo cachea si no cambió; la escritura la sube e invalida bajo el mismo candado.
type partitionGuard struct {
	mu  sync.RWMutex
	gen int64
}

func (g *partitionGuard) generation() int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.gen
}

func (g *partitionGuard) cacheIfUnchanged(c *cache.InMemoryCacheAdapter, ctx context.Context, key string, res ports.DataResult, gen int64) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.gen == gen {
		_ = c.Set(ctx, key, res)
	}
}

func (g *partitionGuard) invalidate(c *cache.InMemoryCacheAdapter, ctx context.Context, prefixes []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.gen++
	_ = c.Invalidate(ctx, prefixes...)
}
