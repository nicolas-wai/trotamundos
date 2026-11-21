// Package domain (integrations) modela proveedores externos y sus actualizaciones.
package domain

import (
	"strings"
	"time"
)

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

// ProviderIDFor arma el id de proveedor "<tipo>:<nombre>" (p. ej. "agencia:agencia-sol"). El circuit breaker
// es uno por proveedor [ADR-07] y el tipo se deduce del propio id.
func ProviderIDFor(t ProviderType, name string) string {
	return strings.ToLower(string(t)) + ":" + strings.ToLower(strings.TrimSpace(name))
}

// ProviderTypeOf deduce el tipo de proveedor de su id; vacío si el id no sigue la convención.
func ProviderTypeOf(providerID string) ProviderType {
	prefix, _, _ := strings.Cut(providerID, ":")
	for _, t := range []ProviderType{ProviderEmail, ProviderAgency, ProviderTravelSystem} {
		if strings.EqualFold(prefix, string(t)) {
			return t
		}
	}
	return ""
}

// Proveedores del prototipo que se sondean para todos los viajeros (agencia y sistema de viajes). [ADR-05]
var (
	DefaultAgencyProviderID       = ProviderIDFor(ProviderAgency, "agencia-sol")
	DefaultTravelSystemProviderID = ProviderIDFor(ProviderTravelSystem, "sistema-viajes")
)
