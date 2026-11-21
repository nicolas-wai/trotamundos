package datanode

import (
	"fmt"
	"hash/fnv"

	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
	"github.com/nicolas-wai/trotamundos/src/datanode/schema"
)

// DirectoryPartition es la partición global ◇ para email → idViajero e idItinerario → dueño. [ADR-11]
const DirectoryPartition = schema.DirectoryPartition

// PartitionRouter [ADR-11] Enrutador de partición: hash(idViajero) mod N, y ruta especial a "directory".
type PartitionRouter struct {
	shards int
}

// NewPartitionRouter crea el enrutador para N shards.
func NewPartitionRouter(shards int) *PartitionRouter { return &PartitionRouter{shards: shards} }

// ShardFor devuelve el shard de un idViajero (FNV-32a mod N, estable entre procesos).
func (r *PartitionRouter) ShardFor(travelerID string) (int, error) {
	if r.shards < 1 {
		return 0, fmt.Errorf("sin shards configurados")
	}
	if travelerID == "" {
		return 0, fmt.Errorf("idViajero vacío: no se puede resolver el shard")
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(travelerID))
	return int(h.Sum32() % uint32(r.shards)), nil
}

// PartitionFor devuelve el nombre de la partición ("shard-N" o "directory") que atiende la operación.
func (r *PartitionRouter) PartitionFor(travelerID string, scope operations.Scope) (string, error) {
	if scope == operations.ScopeDirectory {
		return DirectoryPartition, nil
	}
	shard, err := r.ShardFor(travelerID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("shard-%d", shard), nil
}

// Partitions lista todas las particiones del nodo (shards + directory).
func (r *PartitionRouter) Partitions() []string {
	parts := make([]string, 0, r.shards+1)
	for i := 0; i < r.shards; i++ {
		parts = append(parts, fmt.Sprintf("shard-%d", i))
	}
	return append(parts, DirectoryPartition)
}
