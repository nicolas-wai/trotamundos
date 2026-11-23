package ports

import "github.com/nicolas-wai/trotamundos/src/contracts/events"

// PartitionResolver [ADR-11][ADR-09] Puerto pequeño que deja a la cola de prioridades saber en qué partición
// ("shard-N" o "directory") cae una operación, sin importar el contenedor datanode.
type PartitionResolver interface {
	// PartitionOf devuelve la partición que atiende la operación (error si el recurso o el idViajero no resuelven).
	PartitionOf(op events.QueuedOperation) (string, error)
	// Partitions lista todas las particiones del nodo de datos.
	Partitions() []string
}
