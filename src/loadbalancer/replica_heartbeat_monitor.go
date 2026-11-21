package loadbalancer

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// ReplicaHeartbeatMonitor [ADR-22] Monitor de heartbeats de réplicas: marca una réplica caída tras
// 3 pulsos perdidos (pulso de 10 s) y la retira/reincorpora del LoadBalancer. Lógica en la Fase 2.
type ReplicaHeartbeatMonitor struct {
	heartbeats    ports.HeartbeatPort
	balancer      *LoadBalancer
	interval      time.Duration
	missThreshold int
}

// NewReplicaHeartbeatMonitor crea el monitor.
func NewReplicaHeartbeatMonitor(hb ports.HeartbeatPort, balancer *LoadBalancer, interval time.Duration, missThreshold int) *ReplicaHeartbeatMonitor {
	return &ReplicaHeartbeatMonitor{heartbeats: hb, balancer: balancer, interval: interval, missThreshold: missThreshold}
}

// Run vigila las réplicas hasta que el contexto termina.
func (m *ReplicaHeartbeatMonitor) Run(ctx context.Context) error {
	return apierror.ErrNotImplemented
}
