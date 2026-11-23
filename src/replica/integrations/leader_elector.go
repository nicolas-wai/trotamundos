package integrations

import (
	"context"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

// IntegrationsLeaderElector [ADR-04][ADR-21] Elección de líder simple y sin consenso distribuido: es líder la
// réplica viva de menor posición en la lista, según el HeartbeatPort de réplicas [ADR-22]. Solo la líder ejecuta
// el consumidor de la cola P2P y el planificador de sondeo; cuando deja de serlo (o cae) se cancela su contexto.
//
// Tramo sin líder: entre que la líder cae y su heartbeat se declara caído pasan hasta 3 pulsos (~40 s con pulso
// de 10 s, igual que en el heartbeat de réplicas). En ese tramo nadie consume ni sondea; los mensajes ya publicados
// quedan retenidos en la cola y los procesa la nueva líder. Si la réplica de menor id vuelve, retoma el liderazgo
// y la sucesora la cede; el tramo en que ambas pudieran coincidir lo cubre ErrSingleConsumer (la cola admite un
// único consumidor y el runner reintenta hasta que la otra libere).
type IntegrationsLeaderElector struct {
	selfID     string
	replicaIDs []string // en orden de prioridad: la primera viva lidera
	heartbeat  ports.HeartbeatPort
	interval   time.Duration
}

// NewIntegrationsLeaderElector crea el elector de la réplica selfID. interval es cada cuánto reevalúa el liderazgo.
func NewIntegrationsLeaderElector(selfID string, replicaIDs []string, hb ports.HeartbeatPort, interval time.Duration) *IntegrationsLeaderElector {
	if interval <= 0 {
		interval = time.Second
	}
	return &IntegrationsLeaderElector{selfID: selfID, replicaIDs: replicaIDs, heartbeat: hb, interval: interval}
}

// LeaderID devuelve la réplica viva de menor posición ("" si ninguna está viva).
func (e *IntegrationsLeaderElector) LeaderID() string {
	for _, id := range e.replicaIDs {
		if e.heartbeat.Observe(id) == ports.NodeAlive {
			return id
		}
	}
	return ""
}

// IsLeader indica si esta réplica es hoy la líder.
func (e *IntegrationsLeaderElector) IsLeader() bool { return e.LeaderID() == e.selfID }

// Run reevalúa el liderazgo cada intervalo. Al volverse líder lanza start con un contexto propio; al dejar de
// serlo (o al terminar ctx) cancela ese contexto y espera a que start termine antes de ceder.
func (e *IntegrationsLeaderElector) Run(ctx context.Context, start func(ctx context.Context)) {
	var (
		cancel context.CancelFunc
		wg     sync.WaitGroup
	)
	stop := func() {
		if cancel != nil {
			cancel()
			wg.Wait()
			cancel = nil
		}
	}
	defer stop()
	t := time.NewTicker(e.interval)
	defer t.Stop()
	for {
		switch leader := e.IsLeader(); {
		case leader && cancel == nil:
			lctx, c := context.WithCancel(ctx)
			cancel = c
			wg.Add(1)
			go func() { defer wg.Done(); start(lctx) }()
		case !leader:
			stop()
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// RunWithRetry ejecuta f hasta que ctx termina; si f falla (p. ej. ErrSingleConsumer de la cola P2P porque la
// líder anterior aún no liberó el consumo) reintenta tras retry.
func RunWithRetry(ctx context.Context, retry time.Duration, f func(context.Context) error) {
	for ctx.Err() == nil {
		err := f(ctx)
		if err == nil || ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retry):
		}
	}
}
