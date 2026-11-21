package ports

import "context"

// DataQuery es una lectura nombrada contra el catálogo de operaciones del nodo de datos.
type DataQuery struct {
	Resource string
	Params   map[string]any
}

// DataCommand es una escritura nombrada contra el catálogo de operaciones del nodo de datos.
type DataCommand struct {
	Resource string
	Params   map[string]any
}

// DataResult es el resultado de una operación de datos.
type DataResult struct {
	Rows         []map[string]any
	RowsAffected int64
}

// DataPort [ADR-10][ADR-11] Acceso a datos tras la cola de prioridades.
// Enrutado al shard por idViajero. Leer: caché primero, réplica de lectura si hay miss.
// Escribir: primaria, replica e invalida caché.
type DataPort interface {
	Read(ctx context.Context, travelerID string, query DataQuery) (DataResult, error)
	Write(ctx context.Context, travelerID string, command DataCommand) (DataResult, error)
}
