// Package domain (integrations) modela proveedores externos y sus actualizaciones.
package domain

import "time"

// ProviderType es el tipo de proveedor consultado por sondeo. [ADR-05]
type ProviderType string

// Tipos de proveedor del contrato E1.
const (
	ProviderEmail        ProviderType = "EMAIL"
	ProviderAgency       ProviderType = "AGENCIA"
	ProviderTravelSystem ProviderType = "VIAJES"
)

// CircuitState es el estado del circuit breaker de un proveedor. [ADR-07]
type CircuitState string

// Estados del circuit breaker.
const (
	CircuitClosed   CircuitState = "CERRADO"
	CircuitOpen     CircuitState = "ABIERTO"
	CircuitHalfOpen CircuitState = "SEMIABIERTO"
)

// ExternalProvider es una fuente consultada por sondeo, con el estado de su circuit breaker.
type ExternalProvider struct {
	ProviderID   string
	Type         ProviderType
	CircuitState CircuitState
}

// ProviderUpdate es un mensaje transitorio ya normalizado [ADR-06]; no es agregado persistente.
type ProviderUpdate struct {
	EventID     string
	EventType   string
	GeneratedAt time.Time
	ReceivedAt  time.Time
}
