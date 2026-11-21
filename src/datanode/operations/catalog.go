// Package operations contiene el catálogo de operaciones nombradas del nodo de datos.
// [ADR-10] El recurso de E8 se resuelve aquí a SQL plano parametrizado (database/sql, sin ORM).
package operations

import (
	"fmt"

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

// Nombres de recursos de Autenticación (Fase 2).
const (
	DirectoryTravelerReserve = "directory.traveler.reserve"
	DirectoryTravelerLookup  = "directory.traveler.lookup"
	DirectoryTravelerRelease = "directory.traveler.release"
	TravelerCreate           = "traveler.create"
	TravelerFindByID         = "traveler.find_by_id"
	RefreshTokenSave         = "refresh_token.save"
	RefreshTokenFind         = "refresh_token.find"
	RefreshTokenRevoke       = "refresh_token.revoke"
)

// Nombres de recursos de Arreglos de viaje (Fase 3).
const (
	TripCreate                     = "trip.create"
	TripList                       = "trip.list"
	TripFindByID                   = "trip.find_by_id"
	ArrangementListSummaries       = "arrangement.list_summaries"
	ArrangementFindByID            = "arrangement.find_by_id"
	ArrangementFindByDeduplication = "arrangement.find_by_deduplication_key"
	ArrangementSave                = "arrangement.save"
	ArrangementDelete              = "arrangement.delete"
)

// DefaultCatalog devuelve el catálogo con las operaciones de las Fases 2 (Autenticación) y 3 (Arreglos de viaje).
func DefaultCatalog() *Catalog {
	c := NewCatalog()
	for _, op := range []Operation{
		{DirectoryTravelerReserve, Write, ScopeDirectory, `INSERT INTO traveler_directory (email, id_viajero) VALUES (?, ?)`, []string{"email", "idViajero"}, "PERFIL"},
		{DirectoryTravelerLookup, Read, ScopeDirectory, `SELECT id_viajero FROM traveler_directory WHERE email = ?`, []string{"email"}, ""},
		{DirectoryTravelerRelease, Write, ScopeDirectory, `DELETE FROM traveler_directory WHERE email = ? AND id_viajero = ?`, []string{"email", "idViajero"}, "PERFIL"},
		{TravelerCreate, Write, ScopeShard, `INSERT INTO travelers (id_viajero, email, encrypted_password, created_at) VALUES (?, ?, ?, ?)`, []string{"idViajero", "email", "encryptedPassword", "createdAt"}, "PERFIL"},
		{TravelerFindByID, Read, ScopeShard, `SELECT id_viajero, email, encrypted_password, created_at FROM travelers WHERE id_viajero = ?`, []string{"idViajero"}, ""},
		{RefreshTokenSave, Write, ScopeShard, `INSERT INTO refresh_tokens (token_id, id_viajero, expires_at, revoked) VALUES (?, ?, ?, 0)`, []string{"tokenId", "idViajero", "expiresAt"}, "PERFIL"},
		{RefreshTokenFind, Read, ScopeShard, `SELECT token_id, id_viajero, expires_at, revoked FROM refresh_tokens WHERE token_id = ? AND id_viajero = ?`, []string{"tokenId", "idViajero"}, ""},
		{RefreshTokenRevoke, Write, ScopeShard, `UPDATE refresh_tokens SET revoked = 1 WHERE token_id = ? AND id_viajero = ?`, []string{"tokenId", "idViajero"}, "PERFIL"},

		// Arreglos de viaje (Fase 3). Todas filtran por id_viajero además del id del recurso: un viajero
		// nunca alcanza datos de otro. El SQL es determinístico porque la réplica de lectura lo reejecuta (ADR-10).
		{TripCreate, Write, ScopeShard, `INSERT OR IGNORE INTO trips (id_viaje, id_viajero, nombre, destino, fecha_inicio, fecha_fin, estado) VALUES (?, ?, ?, ?, ?, ?, ?)`, []string{"idViaje", "idViajero", "nombre", "destino", "fechaInicio", "fechaFin", "estado"}, "VIAJE"},
		{TripList, Read, ScopeShard, `SELECT id_viaje, id_viajero, nombre, destino, fecha_inicio, fecha_fin, estado FROM trips WHERE id_viajero = ? ORDER BY fecha_inicio, id_viaje`, []string{"idViajero"}, ""},
		{TripFindByID, Read, ScopeShard, `SELECT id_viaje, id_viajero, nombre, destino, fecha_inicio, fecha_fin, estado FROM trips WHERE id_viaje = ? AND id_viajero = ?`, []string{"idViaje", "idViajero"}, ""},
		// Dashboard: solo columnas críticas, sin detalle ni remitente [ADR-17].
		{ArrangementListSummaries, Read, ScopeShard, `SELECT id_arreglo, id_viaje, tipo, titulo, origen, estado, fecha_inicio, fecha_fin FROM arrangements WHERE id_viajero = ? ORDER BY fecha_inicio, id_arreglo`, []string{"idViajero"}, ""},
		{ArrangementFindByID, Read, ScopeShard, `SELECT id_arreglo, id_viaje, id_viajero, tipo, titulo, origen, remitente, estado, detalle, clave_deduplicacion, fecha_inicio, fecha_fin, generado_en_fuente, actualizado_en FROM arrangements WHERE id_arreglo = ? AND id_viajero = ?`, []string{"idArreglo", "idViajero"}, ""},
		{ArrangementFindByDeduplication, Read, ScopeShard, `SELECT id_arreglo, id_viaje, id_viajero, tipo, titulo, origen, remitente, estado, detalle, clave_deduplicacion, fecha_inicio, fecha_fin, generado_en_fuente, actualizado_en FROM arrangements WHERE clave_deduplicacion = ? AND id_viajero = ?`, []string{"claveDeduplicacion", "idViajero"}, ""},
		{ArrangementSave, Write, ScopeShard, `INSERT INTO arrangements (id_arreglo, id_viaje, id_viajero, tipo, titulo, origen, remitente, estado, detalle, clave_deduplicacion, fecha_inicio, fecha_fin, generado_en_fuente, actualizado_en) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id_arreglo) DO UPDATE SET id_viaje = excluded.id_viaje, tipo = excluded.tipo, titulo = excluded.titulo, origen = excluded.origen, remitente = excluded.remitente, estado = excluded.estado, detalle = excluded.detalle, clave_deduplicacion = excluded.clave_deduplicacion, fecha_inicio = excluded.fecha_inicio, fecha_fin = excluded.fecha_fin, generado_en_fuente = excluded.generado_en_fuente, actualizado_en = excluded.actualizado_en WHERE arrangements.id_viajero = excluded.id_viajero`, []string{"idArreglo", "idViaje", "idViajero", "tipo", "titulo", "origen", "remitente", "estado", "detalle", "claveDeduplicacion", "fechaInicio", "fechaFin", "generadoEnFuente", "actualizadoEn"}, "ARREGLO"},
		{ArrangementDelete, Write, ScopeShard, `DELETE FROM arrangements WHERE id_arreglo = ? AND id_viajero = ?`, []string{"idArreglo", "idViajero"}, "ARREGLO"},
	} {
		c.Register(op)
	}
	return c
}
