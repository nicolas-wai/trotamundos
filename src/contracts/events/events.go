// Package events contiene los eventos asíncronos E1..E8 de AD §6.3.
package events

import "time"

// Tópicos del bus de eventos interno [ADR-07, ADR-13].
const (
	TopicProviderDegraded     = "ProveedorDegradado"    // E2
	TopicAccessRequestCreated = "SolicitudAccesoCreada" // E7
)

// ProviderRef identifica al proveedor que originó una actualización.
type ProviderRef struct {
	Tipo        string `json:"tipo"` // EMAIL | AGENCIA | VIAJES
	IDProveedor string `json:"idProveedor"`
}

// NormalizedArrangement es el arreglo ya normalizado por el broker.
type NormalizedArrangement struct {
	Tipo               string         `json:"tipo"`
	ClaveDeduplicacion string         `json:"claveDeduplicacion"`
	Estado             string         `json:"estado"`
	FechaInicio        time.Time      `json:"fechaInicio"`
	FechaFin           time.Time      `json:"fechaFin"`
	Detalle            map[string]any `json:"detalle"`
}

// ProviderUpdateNormalized es E1 `ActualizacionProveedorNormalizada`.
// [ADR-04] viaja por la cola P2P. [ADR-06] normalizada en el broker.
type ProviderUpdateNormalized struct {
	EventID          string                `json:"eventId"`
	TipoEvento       string                `json:"tipoEvento"` // ARREGLO_CREADO | ARREGLO_MODIFICADO | ARREGLO_CANCELADO
	IDViajero        string                `json:"idViajero"`
	Proveedor        ProviderRef           `json:"proveedor"`
	OrigenRemitente  *string               `json:"origenRemitente"`
	Arreglo          NormalizedArrangement `json:"arreglo"`
	GeneradoEnFuente time.Time             `json:"generadoEnFuente"`
	RecibidoEn       time.Time             `json:"recibidoEn"`
}

// ProviderDegraded es E2 `ProveedorDegradado`. [ADR-07]
type ProviderDegraded struct {
	IDProveedor    string    `json:"idProveedor"`
	Tipo           string    `json:"tipo"`
	IDViajero      *string   `json:"idViajero"`
	EstadoCircuito string    `json:"estadoCircuito"` // ABIERTO | CERRADO
	Reintentos     int       `json:"reintentos"`
	ProximoIntento time.Time `json:"proximoIntento"`
}

// Heartbeat es E3. [ADR-22]
type Heartbeat struct {
	NodoID    string    `json:"nodoId"`
	TipoNodo  string    `json:"tipoNodo"` // BACKEND | DATOS
	Rol       string    `json:"rol"`      // ACTIVO | PASIVO | N/A
	Secuencia int64     `json:"secuencia"`
	EmitidoEn time.Time `json:"emitidoEn"`
}

// ResponsibilitySwap es E4 `IntercambioResponsabilidades`. [ADR-22]
type ResponsibilitySwap struct {
	RegionID      string    `json:"regionId"`
	NuevoPrimario string    `json:"nuevoPrimario"`
	NuevoPasivo   string    `json:"nuevoPasivo"`
	Motivo        string    `json:"motivo"`
	OcurridoEn    time.Time `json:"ocurridoEn"`
}

// PassiveRejoined es E4b `PasivoReincorporado`: el nodo caído volvió, recibió la imagen completa del activo
// y quedó como respaldo válido. [ADR-21]
type PassiveRejoined struct {
	RegionID      string    `json:"regionId"`
	Nodo          string    `json:"nodo"`
	VersionEstado int64     `json:"versionEstado"`
	OcurridoEn    time.Time `json:"ocurridoEn"`
}

// StateSync es E5 `SincronizacionEstado`. [ADR-21]
type StateSync struct {
	Origen        string    `json:"origen"`
	Destino       string    `json:"destino"`
	VersionEstado int64     `json:"versionEstado"`
	EmitidoEn     time.Time `json:"emitidoEn"`
}

// DataWritten es E6 `DatosEscritos`. [ADR-10]
type DataWritten struct {
	IDViajero string   `json:"idViajero"`
	Entidad   string   `json:"entidad"` // ARREGLO | VIAJE | PERFIL | PERMISO
	Claves    []string `json:"claves"`
	Version   int64    `json:"version"`
}

// AccessRequestCreated es E7 `SolicitudAccesoCreada`. [ADR-13]
type AccessRequestCreated struct {
	IDItinerario  string    `json:"idItinerario"`
	IDDueno       string    `json:"idDueno"`
	IDSolicitante string    `json:"idSolicitante"`
	IDSolicitud   string    `json:"idSolicitud"`
	CreadaEn      time.Time `json:"creadaEn"`
}

// Tipos de operación de E8.
const (
	OperationRead  = "LECTURA"
	OperationWrite = "ESCRITURA"
)

// QueuedOperation es E8 `OperacionEncolada`. [ADR-09] idViajero siempre sale del token [ADR-11].
type QueuedOperation struct {
	OperacionID string         `json:"operacionId"`
	Tipo        string         `json:"tipo"` // LECTURA | ESCRITURA
	Prioridad   int            `json:"prioridad"`
	IDViajero   string         `json:"idViajero"`
	Recurso     string         `json:"recurso"`
	Payload     map[string]any `json:"payload"`
	EncoladaEn  time.Time      `json:"encoladaEn"`
}
