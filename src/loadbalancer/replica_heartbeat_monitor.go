package loadbalancer

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

// ReplicaHeartbeatMonitor [ADR-22][ADR-08] Monitor de heartbeats de réplicas: marca una réplica caída tras
// 3 pulsos perdidos (pulso de 10 s; el umbral lo resuelve el HeartbeatPort) y la retira/reincorpora
// del LoadBalancer.
type ReplicaHeartbeatMonitor struct {
	heartbeats ports.HeartbeatPort
	balancer   *LoadBalancer
	interval   time.Duration
}

// NewReplicaHeartbeatMonitor crea el monitor.
func NewReplicaHeartbeatMonitor(hb ports.HeartbeatPort, balancer *LoadBalancer, interval time.Duration) *ReplicaHeartbeatMonitor {
	return &ReplicaHeartbeatMonitor{heartbeats: hb, balancer: balancer, interval: interval}
}

// Check actualiza una vez la rotación del balanceador según los heartbeats observados.
func (m *ReplicaHeartbeatMonitor) Check() {
	for _, id := range m.balancer.BackendIDs() {
		m.balancer.SetHealthy(id, m.heartbeats.Observe(id) == ports.NodeAlive)
	}
}

// Run vigila las réplicas hasta que el contexto termina.
func (m *ReplicaHeartbeatMonitor) Run(ctx context.Context) error {
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			m.Check()
		}
	}
}
