package httpdto

import "time"

// --- Dashboard y arreglos [ADR-17, ADR-18, ADR-20] ---

// DashboardResponse es la representación JSON de GET /dashboard (solo datos críticos).
type DashboardResponse struct {
	Viajes            []TripSummary    `json:"viajes"`
	AvisosProveedores []ProviderNotice `json:"avisosProveedores"`
	// Itinerarios son los itinerarios compartidos del viajero como dueño (pantalla "Mis itinerarios"). [ADR-13]
	Itinerarios []OwnedItinerary `json:"itinerarios"`
}

// TripSummary es un viaje del dashboard con sus arreglos críticos.
type TripSummary struct {
	IDViaje     string               `json:"idViaje"`
	Nombre      string               `json:"nombre"`
	Destino     string               `json:"destino"`
	FechaInicio string               `json:"fechaInicio"` // fecha YYYY-MM-DD
	FechaFin    string               `json:"fechaFin"`    // fecha YYYY-MM-DD
	Arreglos    []ArrangementSummary `json:"arreglos"`
}

// ArrangementSummary es el arreglo con payload reducido (sin detalle) [ADR-17].
type ArrangementSummary struct {
	IDArreglo   string    `json:"idArreglo"`
	Tipo        string    `json:"tipo"`
	Titulo      string    `json:"titulo"`
	FechaInicio time.Time `json:"fechaInicio"`
	Estado      string    `json:"estado"`
	Origen      string    `json:"origen"`
}

// ProviderNotice es un aviso de proveedor degradado [ADR-07].
type ProviderNotice struct {
	Proveedor string `json:"proveedor"`
	Mensaje   string `json:"mensaje"`
}

// ArrangementDetailResponse es la respuesta 200 de GET /arreglos/{idArreglo}.
type ArrangementDetailResponse struct {
	IDArreglo     string         `json:"idArreglo"`
	IDViaje       string         `json:"idViaje"`
	Tipo          string         `json:"tipo"`
	Estado        string         `json:"estado"`
	Origen        string         `json:"origen"`
	Remitente     *string        `json:"remitente"`
	Detalle       map[string]any `json:"detalle"`
	ActualizadoEn time.Time      `json:"actualizadoEn"`
}

// ArrangementRequest es el body de POST /arreglos y PUT /arreglos/{idArreglo}.
type ArrangementRequest struct {
	IDViaje     string         `json:"idViaje"`
	Tipo        string         `json:"tipo"`
	Titulo      string         `json:"titulo"`
	FechaInicio time.Time      `json:"fechaInicio"`
	FechaFin    time.Time      `json:"fechaFin"`
	Detalle     map[string]any `json:"detalle"`
}

// CreateArrangementResponse es la respuesta 201 de POST /arreglos.
type CreateArrangementResponse struct {
	IDArreglo string `json:"idArreglo"`
}
