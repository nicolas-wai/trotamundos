// Package integrations ensambla el módulo "Integraciones externas" de una réplica (AD §4.4):
// Planificador de sondeo → Circuit breaker → Adapters (Email, Agencias, Sistemas de viajes) → Puerto único →
// Broker → cola P2P. [ADR-04][ADR-05][ADR-06][ADR-07]
package integrations

import (
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/adapters/agency"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/adapters/email"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/adapters/travelsystem"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/broker"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/circuitbreaker"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/providerport"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/scheduler"
)

// Clients son las APIs externas que consumen los adapters (en el prototipo, los stubs). [ADR-05]
type Clients struct {
	Mail         email.MailClient
	Agency       agency.Client
	TravelSystem travelsystem.Client
}

// Config parametriza el módulo.
type Config struct {
	PollInterval time.Duration
	PollTimeout  time.Duration
	Breaker      circuitbreaker.Config
}

// Module agrupa los componentes de Integraciones externas de una réplica.
type Module struct {
	Registry  *providerport.ProviderPortRegistry
	Breaker   *circuitbreaker.ProviderCircuitBreaker
	Broker    *broker.Broker
	Scheduler *scheduler.PollingScheduler
}

// NewModule arma el módulo: registra un adapter por tipo de proveedor en el puerto único [QA-10][QA-14] y conecta
// planificador → circuit breaker → adapter → broker → cola P2P.
func NewModule(cfg Config, queue ports.MessageQueuePort, bus ports.EventBusPort, data ports.DataGateway, tokens email.TokenResolver, clients Clients) *Module {
	registry := providerport.NewProviderPortRegistry()
	registry.Register(email.NewEmailAdapter(clients.Mail, tokens))
	registry.Register(agency.NewAgencyAdapter(clients.Agency))
	registry.Register(travelsystem.NewTravelSystemAdapter(clients.TravelSystem))

	breaker := circuitbreaker.NewProviderCircuitBreaker(bus).WithConfig(cfg.Breaker)
	brk := broker.NewBroker(queue)
	sch := scheduler.NewPollingScheduler(cfg.PollInterval, breaker, registry, scheduler.NewDirectoryTargetSource(data), brk).
		WithPollTimeout(cfg.PollTimeout)
	return &Module{Registry: registry, Breaker: breaker, Broker: brk, Scheduler: sch}
}
