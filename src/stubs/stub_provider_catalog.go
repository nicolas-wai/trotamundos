package stubs

// StubProviderCatalog [ADR-25] Catálogo determinístico de ítems de alojamiento, transporte y ruta
// con metadatos (precio, destino, categoría, estrellas…) que consume el motor de filtrado (Fase 5).
type StubProviderCatalog struct{}

// NewStubProviderCatalog crea el catálogo.
func NewStubProviderCatalog() *StubProviderCatalog { return &StubProviderCatalog{} }
