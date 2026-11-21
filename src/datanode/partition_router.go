package datanode

import "github.com/nicolas-wai/trotamundos/src/contracts/apierror"

// DirectoryPartition es la partición global ◇ para email → idViajero e idItinerario → dueño. [ADR-11]
const DirectoryPartition = "directory"

// PartitionRouter [ADR-11] Enrutador de partición: hash(idViajero) mod N, y ruta especial a "directory".
type PartitionRouter struct {
	shards int
}

// NewPartitionRouter crea el enrutador para N shards.
func NewPartitionRouter(shards int) *PartitionRouter { return &PartitionRouter{shards: shards} }

// ShardFor devuelve el shard de un idViajero.
func (r *PartitionRouter) ShardFor(travelerID string) (int, error) {
	return 0, apierror.ErrNotImplemented
}
