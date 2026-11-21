// Package readreplica es el componente "Réplicas de lectura" del nodo de datos (AD §4.6).
package readreplica

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/nicolas-wai/trotamundos/src/datanode/internal/sqlitefile"
	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
	"github.com/nicolas-wai/trotamundos/src/datanode/schema"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// ReadReplicaStore [ADR-10] Réplica de lectura por partición (replica-shard-N.db), atiende los cache miss.
// Se mantiene al día por replicación de sentencias disparada por E6 DatosEscritos.
type ReadReplicaStore struct {
	dbs map[string]*sqlitefile.DB
}

// NewReadReplicaStore abre un archivo replica-<partición>.db por partición en dir.
func NewReadReplicaStore(dir string, partitions []string) (*ReadReplicaStore, error) {
	s := &ReadReplicaStore{dbs: map[string]*sqlitefile.DB{}}
	for _, p := range partitions {
		db, err := sqlitefile.Open(filepath.Join(dir, "replica-"+p+".db"), schema.DDLFor(p))
		if err != nil {
			s.Close()
			return nil, err
		}
		s.dbs[p] = db
	}
	return s, nil
}

func (s *ReadReplicaStore) db(partition string) (*sqlitefile.DB, error) {
	db, ok := s.dbs[partition]
	if !ok {
		return nil, fmt.Errorf("partición desconocida: %s", partition)
	}
	return db, nil
}

// Read ejecuta una lectura nombrada en la partición.
func (s *ReadReplicaStore) Read(ctx context.Context, partition string, op operations.Operation, query ports.DataQuery) (ports.DataResult, error) {
	db, err := s.db(partition)
	if err != nil {
		return ports.DataResult{}, err
	}
	args, err := op.BindArgs(query.Params)
	if err != nil {
		return ports.DataResult{}, err
	}
	rows, err := db.Query(ctx, op.SQL, args...)
	if err != nil {
		return ports.DataResult{}, err
	}
	return ports.DataResult{Rows: rows}, nil
}

// Replicate aplica en la réplica la escritura que acaba de confirmar la primaria. [ADR-10]
func (s *ReadReplicaStore) Replicate(ctx context.Context, rec operations.ReplicationRecord) error {
	db, err := s.db(rec.Partition)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, rec.Op.SQL, rec.Args...)
	return err
}

// InstallFrom reemplaza la partición por una copia del archivo de la primaria (tras instalar un snapshot).
func (s *ReadReplicaStore) InstallFrom(partition, primaryPath string) error {
	db, err := s.db(partition)
	if err != nil {
		return err
	}
	return db.Replace(primaryPath, false)
}

// Close cierra todos los archivos.
func (s *ReadReplicaStore) Close() {
	for _, db := range s.dbs {
		_ = db.Close()
	}
}
