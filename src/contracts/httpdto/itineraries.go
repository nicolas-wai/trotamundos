package httpdto

// --- Itinerarios compartidos [ADR-13] ---

// SharedItineraryResponse es la respuesta 200 de GET /itinerarios/{idItinerario}.
type SharedItineraryResponse struct {
	IDItinerario string               `json:"idItinerario"`
	Nombre       string               `json:"nombre"`
	Arreglos     []ArrangementSummary `json:"arreglos"`
}

// AccessDeniedResponse es el 403 de GET /itinerarios/{idItinerario}.
type AccessDeniedResponse struct {
	Codigo         string `json:"codigo"`
	Mensaje        string `json:"mensaje,omitempty"`
	PuedeSolicitar bool   `json:"puedeSolicitar"`
}

// CreateItineraryRequest es el body de POST /itinerarios.
type CreateItineraryRequest struct {
	Nombre      string   `json:"nombre"`
	IDsArreglos []string `json:"idsArreglos"`
}

// CreateItineraryResponse es la respuesta 201 de POST /itinerarios.
type CreateItineraryResponse struct {
	IDItinerario string `json:"idItinerario"`
	LinkPublico  string `json:"linkPublico"`
}

// AddArrangementRequest es el body de POST /itinerarios/{id}/arreglos.
type AddArrangementRequest struct {
	IDArreglo string `json:"idArreglo"`
}

// CreateAccessRequestResponse es la respuesta 201 de POST /itinerarios/{id}/solicitudes.
type CreateAccessRequestResponse struct {
	IDSolicitud string `json:"idSolicitud"`
	Estado      string `json:"estado"`
}

// AccessRequestsResponse es la respuesta 200 de GET /itinerarios/{id}/solicitudes.
type AccessRequestsResponse struct {
	Solicitudes []AccessRequestItem `json:"solicitudes"`
}

// AccessRequestItem es una solicitud pendiente.
type AccessRequestItem struct {
	IDSolicitud string `json:"idSolicitud"`
	IDViajero   string `json:"idViajero"`
	Estado      string `json:"estado"`
}

// DecisionRequest es el body de POST /itinerarios/{id}/solicitudes/{idSolicitud}/decision.
type DecisionRequest struct {
	Decision string `json:"decision"` // ACEPTAR | RECHAZAR
}

// DecisionResponse es la respuesta 200 de la decisión.
type DecisionResponse struct {
	Estado string `json:"estado"` // ACEPTADA | RECHAZADA
}

// OwnedItinerary es un itinerario del dueño dentro de GET /dashboard: sus reservas, los accesos vigentes y las
// solicitudes pendientes de decisión. [ADR-13]
type OwnedItinerary struct {
	IDItinerario string               `json:"idItinerario"`
	Nombre       string               `json:"nombre"`
	LinkPublico  string               `json:"linkPublico"`
	Arreglos     []ArrangementSummary `json:"arreglos"`
	Accesos      []string             `json:"accesos"`     // idViajero con acceso vigente
	Solicitudes  []AccessRequestItem  `json:"solicitudes"` // solo PENDIENTE
}
