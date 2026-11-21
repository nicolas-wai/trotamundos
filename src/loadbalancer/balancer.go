// Package loadbalancer es el contenedor "Load Balancer y Health Monitor" (AD §4.3).
package loadbalancer

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
)

// Backend es una réplica registrada en el balanceador.
type Backend struct {
	ID  string
	URL *url.URL
}

type member struct {
	id      string
	proxy   *httputil.ReverseProxy
	healthy bool
}

// LoadBalancer [ADR-08] Balanceador de carga round-robin sobre las réplicas vivas del monolito.
// [Prototipo] HTTP local simulando HTTPS/TLS [ADR-12].
// ReplicaHeartbeatMonitor retira y reincorpora réplicas con SetHealthy [ADR-22].
type LoadBalancer struct {
	mu      sync.RWMutex
	members []*member
	next    atomic.Uint64
}

// NewLoadBalancer crea el balanceador; todas las réplicas arrancan vivas.
func NewLoadBalancer(backends []Backend) *LoadBalancer {
	lb := &LoadBalancer{}
	for _, b := range backends {
		p := httputil.NewSingleHostReverseProxy(b.URL)
		p.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
			apierror.WriteError(w, http.StatusServiceUnavailable, apierror.ModuloNoDisponible, "Réplica no disponible")
		}
		lb.members = append(lb.members, &member{id: b.ID, proxy: p, healthy: true})
	}
	return lb
}

// BackendIDs devuelve los identificadores de todas las réplicas registradas.
func (lb *LoadBalancer) BackendIDs() []string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	ids := make([]string, len(lb.members))
	for i, m := range lb.members {
		ids[i] = m.id
	}
	return ids
}

// SetHealthy retira (false) o reincorpora (true) una réplica de la rotación.
func (lb *LoadBalancer) SetHealthy(id string, healthy bool) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	for _, m := range lb.members {
		if m.id == id {
			m.healthy = healthy
		}
	}
}

// IsHealthy indica si la réplica está en rotación.
func (lb *LoadBalancer) IsHealthy(id string) bool {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	for _, m := range lb.members {
		if m.id == id {
			return m.healthy
		}
	}
	return false
}

func (lb *LoadBalancer) pick() *member {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	alive := make([]*member, 0, len(lb.members))
	for _, m := range lb.members {
		if m.healthy {
			alive = append(alive, m)
		}
	}
	if len(alive) == 0 {
		return nil
	}
	return alive[(lb.next.Add(1)-1)%uint64(len(alive))]
}

// ServeHTTP reparte la request a la siguiente réplica viva en orden round-robin.
func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m := lb.pick()
	if m == nil {
		apierror.WriteError(w, http.StatusServiceUnavailable, apierror.ModuloNoDisponible, "No hay réplicas disponibles")
		return
	}
	m.proxy.ServeHTTP(w, r)
}
