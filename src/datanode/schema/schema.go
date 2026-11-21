// Package schema contiene el DDL SQL plano embebido de cada shard y de la partición directory.
// [ADR-10][ADR-11] Todas las tablas particionadas llevan id_viajero. Sin ORM.
package schema

import _ "embed"

//go:embed shard.sql
var shardDDL string

//go:embed directory.sql
var directoryDDL string

// DirectoryPartition es el nombre de la partición global ◇ (email → idViajero, idItinerario → dueño).
const DirectoryPartition = "directory"

// ShardDDL devuelve el DDL de un shard por idViajero.
func ShardDDL() string { return shardDDL }

// DirectoryDDL devuelve el DDL de la partición directory.
func DirectoryDDL() string { return directoryDDL }

// DDLFor devuelve el DDL que corresponde al nombre de partición.
func DDLFor(partition string) string {
	if partition == DirectoryPartition {
		return directoryDDL
	}
	return shardDDL
}
