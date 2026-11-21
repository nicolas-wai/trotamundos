// Package loadbalancer es el contenedor "Load Balancer y Health Monitor" (AD §4.3).
package loadbalancer

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
)

// LoadBalancer [ADR-08] Balanceador de carga round-robin sobre réplicas stateless del monolito.
// [Prototipo] HTTP local simulando HTTPS/TLS [ADR-12].
// Fase 1: lista estática de réplicas. La retirada/reincorporación por heartbeat llega en la Fase 2 [ADR-22].
type LoadBalancer struct {
	proxies []*httputil.ReverseProxy
	next    atomic.Uint64
}

// NewLoadBalancer crea el balanceador sobre las URLs de las réplicas.
func NewLoadBalancer(replicas []*url.URL) *LoadBalancer {
	lb := &LoadBalancer{}
	for _, target := range replicas {
		p := httputil.NewSingleHostReverseProxy(target)
		p.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
			apierror.WriteError(w, http.StatusServiceUnavailable, apierror.ModuloNoDisponible, "Réplica no disponible")
		}
		lb.proxies = append(lb.proxies, p)
	}
	return lb
}

// ServeHTTP reparte la request a la siguiente réplica en orden round-robin.
func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(lb.proxies) == 0 {
		apierror.WriteError(w, http.StatusServiceUnavailable, apierror.ModuloNoDisponible, "No hay réplicas disponibles")
		return
	}
	i := (lb.next.Add(1) - 1) % uint64(len(lb.proxies))
	lb.proxies[i].ServeHTTP(w, r)
}
