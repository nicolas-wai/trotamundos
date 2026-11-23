// Package operations contiene el catálogo de operaciones nombradas del nodo de datos.
// [ADR-10] El recurso de E8 se resuelve aquí a SQL plano parametrizado (database/sql, sin ORM).
package operations

import (
	"fmt"

	"github.com/nicolas-wai/trotamundos/src/contracts/resources"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Kind distingue lecturas de escrituras (E8).
type Kind string

// Tipos de operación.
const (
	Read  Kind = "LECTURA"
	Write Kind = "ESCRITURA"
)

// Scope indica en qué partición vive el recurso. [ADR-11]
type Scope string

// Ámbitos de partición.
const (
	ScopeShard     Scope = "SHARD"     // hash(idViajero) mod N
	ScopeDirectory Scope = "DIRECTORY" // partición global ◇
)

// Operation asocia un recurso de E8 con su SQL plano, su tipo y su partición.
type Operation struct {
	Resource string
	Kind     Kind
	Scope    Scope
	SQL      string   // con marcadores ? en el orden de Args
	Args     []string // nombres de los params que alimentan los ?
	Entity   string   // entidad de E6 al escribir (ARREGLO|VIAJE|PERFIL|PERMISO)
}

// BindArgs arma los argumentos posicionales a partir de los params de la operación.
func (o Operation) BindArgs(params map[string]any) ([]any, error) {
	args := make([]any, len(o.Args))
	for i, name := range o.Args {
		v, ok := params[name]
		if !ok {
			return nil, fmt.Errorf("operación %s: falta el parámetro %q", o.Resource, name)
		}
		args[i] = v
	}
	return args, nil
}

// ReplicationRecord es el registro que la primaria entrega a la réplica de lectura tras escribir:
// statement-based replication (ADR-10), disparada por el evento E6.
type ReplicationRecord struct {
	Partition string
	Op        Operation
	Args      []any
}

// Catalog [ADR-10] Catálogo de operaciones nombradas (recurso → SQL).
type Catalog struct {
	operations map[string]Operation
}

// NewCatalog crea un catálogo vacío.
func NewCatalog() *Catalog { return &Catalog{operations: map[string]Operation{}} }

// Register agrega una operación al catálogo.
func (c *Catalog) Register(op Operation) { c.operations[op.Resource] = op }

// Lookup busca una operación por recurso.
func (c *Catalog) Lookup(resource string) (Operation, bool) {
	op, ok := c.operations[resource]
	return op, ok
}

// Resolve busca la operación esperando un tipo concreto.
func (c *Catalog) Resolve(resource string, kind Kind) (Operation, error) {
	op, ok := c.operations[resource]
	if !ok || op.Kind != kind {
		return Operation{}, fmt.Errorf("%w: %s (%s)", ports.ErrUnknownResource, resource, kind)
	}
	return op, nil
}

// DefaultCatalog devuelve el catálogo con las operaciones de las Fases 2 (Autenticación), 3 (Arreglos de viaje), 4 (Integraciones) y 5 (Permisos y Recomendaciones).
func DefaultCatalog() *Catalog {
	c := NewCatalog()
	for _, op := range []Operation{
		{resources.DirectoryTravelerReserve, Write, ScopeDirectory, `INSERT INTO traveler_directory (email, id_viajero) VALUES (?, ?)`, []string{"email", "idViajero"}, "PERFIL"},
		{resources.DirectoryTravelerLookup, Read, ScopeDirectory, `SELECT id_viajero FROM traveler_directory WHERE email = ?`, []string{"email"}, ""},
		{resources.DirectoryTravelerRelease, Write, ScopeDirectory, `DELETE FROM traveler_directory WHERE email = ? AND id_viajero = ?`, []string{"email", "idViajero"}, "PERFIL"},
		{resources.TravelerCreate, Write, ScopeShard, `INSERT INTO travelers (id_viajero, email, encrypted_password, created_at) VALUES (?, ?, ?, ?)`, []string{"idViajero", "email", "encryptedPassword", "createdAt"}, "PERFIL"},
		{resources.TravelerFindByID, Read, ScopeShard, `SELECT id_viajero, email, encrypted_password, created_at FROM travelers WHERE id_viajero = ?`, []string{"idViajero"}, ""},
		{resources.RefreshTokenSave, Write, ScopeShard, `INSERT INTO refresh_tokens (token_id, id_viajero, expires_at, revoked) VALUES (?, ?, ?, 0)`, []string{"tokenId", "idViajero", "expiresAt"}, "PERFIL"},
		{resources.RefreshTokenFind, Read, ScopeShard, `SELECT token_id, id_viajero, expires_at, revoked FROM refresh_tokens WHERE token_id = ? AND id_viajero = ?`, []string{"tokenId", "idViajero"}, ""},
		{resources.RefreshTokenRevoke, Write, ScopeShard, `UPDATE refresh_tokens SET revoked = 1 WHERE token_id = ? AND id_viajero = ?`, []string{"tokenId", "idViajero"}, "PERFIL"},

		// Arreglos de viaje (Fase 3). Todas filtran por id_viajero además del id del recurso: un viajero
		// nunca alcanza datos de otro. El SQL es determinístico porque la réplica de lectura lo reejecuta (ADR-10).
		{resources.TripCreate, Write, ScopeShard, `INSERT OR IGNORE INTO trips (id_viaje, id_viajero, nombre, destino, fecha_inicio, fecha_fin, estado) VALUES (?, ?, ?, ?, ?, ?, ?)`, []string{"idViaje", "idViajero", "nombre", "destino", "fechaInicio", "fechaFin", "estado"}, "VIAJE"},
		{resources.TripList, Read, ScopeShard, `SELECT id_viaje, id_viajero, nombre, destino, fecha_inicio, fecha_fin, estado FROM trips WHERE id_viajero = ? ORDER BY fecha_inicio, id_viaje`, []string{"idViajero"}, ""},
		{resources.TripFindByID, Read, ScopeShard, `SELECT id_viaje, id_viajero, nombre, destino, fecha_inicio, fecha_fin, estado FROM trips WHERE id_viaje = ? AND id_viajero = ?`, []string{"idViaje", "idViajero"}, ""},
		// Dashboard: solo columnas críticas, sin detalle ni remitente [ADR-17].
		{resources.ArrangementListSummaries, Read, ScopeShard, `SELECT id_arreglo, id_viaje, tipo, titulo, origen, estado, fecha_inicio, fecha_fin FROM arrangements WHERE id_viajero = ? ORDER BY fecha_inicio, id_arreglo`, []string{"idViajero"}, ""},
		{resources.ArrangementFindByID, Read, ScopeShard, `SELECT id_arreglo, id_viaje, id_viajero, tipo, titulo, origen, remitente, estado, detalle, clave_deduplicacion, fecha_inicio, fecha_fin, generado_en_fuente, recibido_en, actualizado_en FROM arrangements WHERE id_arreglo = ? AND id_viajero = ?`, []string{"idArreglo", "idViajero"}, ""},
		{resources.ArrangementFindByDeduplication, Read, ScopeShard, `SELECT id_arreglo, id_viaje, id_viajero, tipo, titulo, origen, remitente, estado, detalle, clave_deduplicacion, fecha_inicio, fecha_fin, generado_en_fuente, recibido_en, actualizado_en FROM arrangements WHERE clave_deduplicacion = ? AND id_viajero = ?`, []string{"claveDeduplicacion", "idViajero"}, ""},
		{resources.ArrangementSave, Write, ScopeShard, `INSERT INTO arrangements (id_arreglo, id_viaje, id_viajero, tipo, titulo, origen, remitente, estado, detalle, clave_deduplicacion, fecha_inicio, fecha_fin, generado_en_fuente, recibido_en, actualizado_en) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id_arreglo) DO UPDATE SET id_viaje = excluded.id_viaje, tipo = excluded.tipo, titulo = excluded.titulo, origen = excluded.origen, remitente = excluded.remitente, estado = excluded.estado, detalle = excluded.detalle, clave_deduplicacion = excluded.clave_deduplicacion, fecha_inicio = excluded.fecha_inicio, fecha_fin = excluded.fecha_fin, generado_en_fuente = excluded.generado_en_fuente, recibido_en = excluded.recibido_en, actualizado_en = excluded.actualizado_en WHERE arrangements.id_viajero = excluded.id_viajero`, []string{"idArreglo", "idViaje", "idViajero", "tipo", "titulo", "origen", "remitente", "estado", "detalle", "claveDeduplicacion", "fechaInicio", "fechaFin", "generadoEnFuente", "recibidoEn", "actualizadoEn"}, "ARREGLO"},
		{resources.ArrangementDelete, Write, ScopeShard, `DELETE FROM arrangements WHERE id_arreglo = ? AND id_viajero = ?`, []string{"idArreglo", "idViajero"}, "ARREGLO"},

		// Fase 4: directorio de viajeros y cuentas vinculadas (lo recorre el planificador de sondeo) y avisos E2.
		{resources.DirectoryTravelerList, Read, ScopeDirectory, `SELECT id_viajero FROM traveler_directory ORDER BY id_viajero`, nil, ""},
		{resources.DirectoryLinkedAccountRegister, Write, ScopeDirectory, `INSERT OR REPLACE INTO linked_account_directory (id_cuenta, id_viajero, proveedor) VALUES (?, ?, ?)`, []string{"idCuenta", "idViajero", "proveedor"}, "PERFIL"},
		{resources.DirectoryLinkedAccountList, Read, ScopeDirectory, `SELECT id_cuenta, id_viajero, proveedor FROM linked_account_directory ORDER BY id_cuenta`, nil, ""},
		{resources.DirectoryLinkedAccountRemove, Write, ScopeDirectory, `DELETE FROM linked_account_directory WHERE id_cuenta = ? AND id_viajero = ?`, []string{"idCuenta", "idViajero"}, "PERFIL"},
		{resources.DirectoryProviderNoticeSave, Write, ScopeDirectory, `INSERT OR REPLACE INTO provider_notices (id_proveedor, id_viajero, mensaje, proximo_intento) VALUES (?, ?, ?, ?)`, []string{"idProveedor", "idViajero", "mensaje", "proximoIntento"}, "PERFIL"},
		{resources.DirectoryProviderNoticeList, Read, ScopeDirectory, `SELECT id_proveedor, id_viajero, mensaje, proximo_intento FROM provider_notices ORDER BY id_proveedor`, nil, ""},
		{resources.DirectoryProviderNoticeClear, Write, ScopeDirectory, `DELETE FROM provider_notices WHERE id_proveedor = ?`, []string{"idProveedor"}, "PERFIL"},
		// Cuentas de correo vinculadas (tokens OAuth cifrados, en base64) en el shard del viajero [ADR-15].
		{resources.LinkedAccountSave, Write, ScopeShard, `INSERT OR REPLACE INTO linked_email_accounts (id_cuenta, id_viajero, proveedor, encrypted_access_token, encrypted_refresh_token, estado, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, []string{"idCuenta", "idViajero", "proveedor", "encryptedAccessToken", "encryptedRefreshToken", "estado", "expiresAt"}, "PERFIL"},
		{resources.LinkedAccountFind, Read, ScopeShard, `SELECT id_cuenta, id_viajero, proveedor, encrypted_access_token, encrypted_refresh_token, estado, expires_at FROM linked_email_accounts WHERE id_cuenta = ? AND id_viajero = ?`, []string{"idCuenta", "idViajero"}, ""},
		{resources.LinkedAccountDelete, Write, ScopeShard, `DELETE FROM linked_email_accounts WHERE id_cuenta = ? AND id_viajero = ?`, []string{"idCuenta", "idViajero"}, "PERFIL"},

		// Fase 5 — Itinerarios compartidos [ADR-13][ADR-11]. El directorio ◇ resuelve el shard del dueño a partir de
		// idItinerario; el resto vive en el shard del dueño y siempre filtra por id_viajero (= dueño).
		{resources.DirectoryItineraryRegister, Write, ScopeDirectory, `INSERT INTO itinerary_directory (id_itinerario, id_viajero_dueno) VALUES (?, ?)`, []string{"idItinerario", "idDueno"}, "PERMISO"},
		{resources.DirectoryItineraryResolve, Read, ScopeDirectory, `SELECT id_viajero_dueno FROM itinerary_directory WHERE id_itinerario = ?`, []string{"idItinerario"}, ""},
		{resources.DirectoryItineraryRemove, Write, ScopeDirectory, `DELETE FROM itinerary_directory WHERE id_itinerario = ? AND id_viajero_dueno = ?`, []string{"idItinerario", "idDueno"}, "PERMISO"},
		{resources.ItineraryCreate, Write, ScopeShard, `INSERT INTO shared_itineraries (id_itinerario, id_viajero, nombre, link_publico) VALUES (?, ?, ?, ?)`, []string{"idItinerario", "idViajero", "nombre", "linkPublico"}, "PERMISO"},
		{resources.ItineraryFind, Read, ScopeShard, `SELECT id_itinerario, id_viajero, nombre, link_publico FROM shared_itineraries WHERE id_itinerario = ? AND id_viajero = ?`, []string{"idItinerario", "idViajero"}, ""},
		{resources.ItineraryListByOwner, Read, ScopeShard, `SELECT id_itinerario, id_viajero, nombre, link_publico FROM shared_itineraries WHERE id_viajero = ? ORDER BY id_itinerario`, []string{"idViajero"}, ""},
		{resources.ItineraryDelete, Write, ScopeShard, `DELETE FROM shared_itineraries WHERE id_itinerario = ? AND id_viajero = ?`, []string{"idItinerario", "idViajero"}, "PERMISO"},
		{resources.ItineraryArrangementAdd, Write, ScopeShard, `INSERT OR IGNORE INTO itinerary_arrangements (id_itinerario, id_viajero, id_arreglo) VALUES (?, ?, ?)`, []string{"idItinerario", "idViajero", "idArreglo"}, "PERMISO"},
		{resources.ItineraryArrangementRemove, Write, ScopeShard, `DELETE FROM itinerary_arrangements WHERE id_itinerario = ? AND id_viajero = ? AND id_arreglo = ?`, []string{"idItinerario", "idViajero", "idArreglo"}, "PERMISO"},
		{resources.ItineraryArrangementList, Read, ScopeShard, `SELECT id_arreglo FROM itinerary_arrangements WHERE id_itinerario = ? AND id_viajero = ? ORDER BY id_arreglo`, []string{"idItinerario", "idViajero"}, ""},
		{resources.ItineraryArrangementsClear, Write, ScopeShard, `DELETE FROM itinerary_arrangements WHERE id_itinerario = ? AND id_viajero = ?`, []string{"idItinerario", "idViajero"}, "PERMISO"},
		{resources.AccessPermissionSave, Write, ScopeShard, `INSERT INTO access_permissions (id_itinerario, id_viajero, id_viajero_autorizado, estado) VALUES (?, ?, ?, ?) ON CONFLICT(id_itinerario, id_viajero_autorizado) DO UPDATE SET estado = excluded.estado`, []string{"idItinerario", "idViajero", "idViajeroAutorizado", "estado"}, "PERMISO"},
		{resources.AccessPermissionList, Read, ScopeShard, `SELECT id_viajero_autorizado, estado FROM access_permissions WHERE id_itinerario = ? AND id_viajero = ? ORDER BY id_viajero_autorizado`, []string{"idItinerario", "idViajero"}, ""},
		{resources.AccessPermissionsClear, Write, ScopeShard, `DELETE FROM access_permissions WHERE id_itinerario = ? AND id_viajero = ?`, []string{"idItinerario", "idViajero"}, "PERMISO"},
		{resources.AccessRequestCreate, Write, ScopeShard, `INSERT INTO access_requests (id_solicitud, id_itinerario, id_viajero, id_viajero_solicitante, estado, fecha_solicitud) VALUES (?, ?, ?, ?, ?, ?)`, []string{"idSolicitud", "idItinerario", "idViajero", "idSolicitante", "estado", "fechaSolicitud"}, "PERMISO"},
		{resources.AccessRequestList, Read, ScopeShard, `SELECT id_solicitud, id_viajero_solicitante, estado, fecha_solicitud FROM access_requests WHERE id_itinerario = ? AND id_viajero = ? ORDER BY fecha_solicitud, id_solicitud`, []string{"idItinerario", "idViajero"}, ""},
		{resources.AccessRequestFindItinerary, Read, ScopeShard, `SELECT id_itinerario FROM access_requests WHERE id_solicitud = ? AND id_viajero = ?`, []string{"idSolicitud", "idViajero"}, ""},
		{resources.AccessRequestUpdateStatus, Write, ScopeShard, `UPDATE access_requests SET estado = ? WHERE id_solicitud = ? AND id_itinerario = ? AND id_viajero = ?`, []string{"estado", "idSolicitud", "idItinerario", "idViajero"}, "PERMISO"},
		{resources.AccessRequestsClear, Write, ScopeShard, `DELETE FROM access_requests WHERE id_itinerario = ? AND id_viajero = ?`, []string{"idItinerario", "idViajero"}, "PERMISO"},

		// Fase 5 — Recomendaciones [ADR-24][ADR-25]. Preferencias del perfil y feedback HITL (historial).
		{resources.PreferencesGet, Read, ScopeShard, `SELECT recommendation_params, accommodation_preferences, transport_preferences, route_preferences FROM preferences WHERE id_viajero = ?`, []string{"idViajero"}, ""},
		{resources.PreferencesSave, Write, ScopeShard, `INSERT INTO preferences (id_viajero, recommendation_params, accommodation_preferences, transport_preferences, route_preferences) VALUES (?, ?, ?, ?, ?) ON CONFLICT(id_viajero) DO UPDATE SET recommendation_params = excluded.recommendation_params, accommodation_preferences = excluded.accommodation_preferences, transport_preferences = excluded.transport_preferences, route_preferences = excluded.route_preferences`, []string{"idViajero", "recommendationParams", "accommodationPreferences", "transportPreferences", "routePreferences"}, "PERFIL"},
		{resources.RecommendationFeedbackSave, Write, ScopeShard, `INSERT INTO recommendation_feedback (id_feedback, id_recomendacion, id_viajero, accion, parametros_ajustados, fecha) VALUES (?, ?, ?, ?, ?, ?)`, []string{"idFeedback", "idRecomendacion", "idViajero", "accion", "parametrosAjustados", "fecha"}, "PERFIL"},
		{resources.RecommendationFeedbackList, Read, ScopeShard, `SELECT id_feedback, id_recomendacion, accion, parametros_ajustados, fecha FROM recommendation_feedback WHERE id_viajero = ? ORDER BY fecha, id_feedback`, []string{"idViajero"}, ""},
	} {
		c.Register(op)
	}
	return c
}
