// Package datanode es el contenedor "Nodo de datos primario / pasivo" (AD §4.6).
// El nodo pasivo es una instancia del mismo módulo con la misma estructura interna [ADR-21].
package datanode

import (
	"context"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
	"github.com/nicolas-wai/trotamundos/src/datanode/cache"
	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
	"github.com/nicolas-wai/trotamundos/src/datanode/primary"
	"github.com/nicolas-wai/trotamundos/src/datanode/readreplica"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// Role es el rol del nodo dentro de la región. [ADR-21]
type Role string

// Roles del nodo de datos.
const (
	RoleActive  Role = "ACTIVO"
	RolePassive Role = "PASIVO"
)

// DataNode [ADR-10][ADR-11][ADR-21] Caché + base primaria + réplica de lectura, particionado por idViajero.
type DataNode struct {
	ID        string
	Role      Role
	Router    *PartitionRouter
	Cache     *cache.InMemoryCacheAdapter
	Primary   *primary.PrimaryStore
	Replica   *readreplica.ReadReplicaStore
	Catalog   *operations.Catalog
	Sync      *SyncEmitter
	Heartbeat *HeartbeatEmitter
}

var _ ports.DataPort = (*DataNode)(nil)

// NewDataNode ensambla un nodo de datos con sus componentes vacíos.
func NewDataNode(id string, role Role, shards int) *DataNode {
	return &DataNode{
		ID:        id,
		Role:      role,
		Router:    NewPartitionRouter(shards),
		Cache:     cache.NewInMemoryCacheAdapter(),
		Primary:   primary.NewPrimaryStore(),
		Replica:   readreplica.NewReadReplicaStore(),
		Catalog:   operations.NewCatalog(),
		Sync:      NewSyncEmitter(),
		Heartbeat: NewHeartbeatEmitter(id),
	}
}

// Read lee con caché primero y réplica de lectura ante miss. [ADR-10]
func (n *DataNode) Read(context.Context, string, ports.DataQuery) (ports.DataResult, error) {
	return ports.DataResult{}, apierror.ErrNotImplemented
}

// Write escribe en la primaria del shard, replica a lectura e invalida la caché (E6). [ADR-10]
func (n *DataNode) Write(context.Context, string, ports.DataCommand) (ports.DataResult, error) {
	return ports.DataResult{}, apierror.ErrNotImplemented
}
