// Package stubs contiene los sistemas externos simulados, determinísticos, detrás de los puertos reales.
// [Prototipo] No hay proveedores reales: cada API tiene su propio formato y fallas inyectables (Fase 4).
package stubs

// StubEmailProviderAPI simula la API de un proveedor de correo (texto semiestructurado con remitente). [ADR-15]
type StubEmailProviderAPI struct{}

// NewStubEmailProviderAPI crea el stub.
func NewStubEmailProviderAPI() *StubEmailProviderAPI { return &StubEmailProviderAPI{} }

// StubAgencyAPI simula la API de una agencia (JSON anidado). [ADR-05]
type StubAgencyAPI struct{}

// NewStubAgencyAPI crea el stub.
func NewStubAgencyAPI() *StubAgencyAPI { return &StubAgencyAPI{} }

// StubTravelSystemAPI simula la API de un sistema de viajes (códigos IATA). [ADR-05]
type StubTravelSystemAPI struct{}

// NewStubTravelSystemAPI crea el stub.
func NewStubTravelSystemAPI() *StubTravelSystemAPI { return &StubTravelSystemAPI{} }
