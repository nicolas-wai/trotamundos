// Package primary es el componente "Base primaria" del nodo de datos: escrituras en SQLite plano (AD §4.6).
package primary

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/nicolas-wai/trotamundos/src/datanode/internal/sqlitefile"
	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
	"github.com/nicolas-wai/trotamundos/src/datanode/schema"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// PrimaryStore [ADR-10] Base primaria por partición (primary-shard-N.db, primary-directory.db).
// Tras escribir, el DataNode emite E6 DatosEscritos con la versión que devuelve Write.
type PrimaryStore struct {
	dbs      map[string]*sqlitefile.DB
	mu       sync.Mutex
	versions map[string]int64
}

// NewPrimaryStore abre un archivo primary-<partición>.db por partición en dir.
func NewPrimaryStore(dir string, partitions []string) (*PrimaryStore, error) {
	s := &PrimaryStore{dbs: map[string]*sqlitefile.DB{}, versions: map[string]int64{}}
	for _, p := range partitions {
		db, err := sqlitefile.Open(filepath.Join(dir, "primary-"+p+".db"), schema.DDLFor(p))
		if err != nil {
			s.Close()
			return nil, err
		}
		s.dbs[p] = db
	}
	return s, nil
}

func (s *PrimaryStore) db(partition string) (*sqlitefile.DB, error) {
	db, ok := s.dbs[partition]
	if !ok {
		return nil, fmt.Errorf("partición desconocida: %s", partition)
	}
	return db, nil
}

// Write ejecuta una escritura nombrada en la partición. Devuelve el resultado, los argumentos
// ligados (para el registro de replicación) y la nueva versión de la partición.
func (s *PrimaryStore) Write(ctx context.Context, partition string, op operations.Operation, cmd ports.DataCommand) (ports.DataResult, operations.ReplicationRecord, int64, error) {
	db, err := s.db(partition)
	if err != nil {
		return ports.DataResult{}, operations.ReplicationRecord{}, 0, err
	}
	args, err := op.BindArgs(cmd.Params)
	if err != nil {
		return ports.DataResult{}, operations.ReplicationRecord{}, 0, err
	}
	n, err := db.Exec(ctx, op.SQL, args...)
	if err != nil {
		return ports.DataResult{}, operations.ReplicationRecord{}, 0, err
	}
	s.mu.Lock()
	s.versions[partition]++
	version := s.versions[partition]
	s.mu.Unlock()
	return ports.DataResult{RowsAffected: n}, operations.ReplicationRecord{Partition: partition, Op: op, Args: args}, version, nil
}

// Version devuelve la suma de versiones de todas las particiones (versionEstado de E5).
func (s *PrimaryStore) Version() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var total int64
	for _, v := range s.versions {
		total += v
	}
	return total
}

// Snapshot genera una imagen consistente de la partición con VACUUM INTO. [ADR-21]
func (s *PrimaryStore) Snapshot(ctx context.Context, partition, dest string) error {
	db, err := s.db(partition)
	if err != nil {
		return err
	}
	return db.Snapshot(ctx, dest)
}

// InstallSnapshot reemplaza la partición por la imagen recibida del primario (se consume el archivo).
func (s *PrimaryStore) InstallSnapshot(partition, src string) error {
	db, err := s.db(partition)
	if err != nil {
		return err
	}
	return db.Replace(src, true)
}

// Path devuelve el archivo de la partición.
func (s *PrimaryStore) Path(partition string) string {
	if db, ok := s.dbs[partition]; ok {
		return db.Path()
	}
	return ""
}

// Close cierra todos los archivos.
func (s *PrimaryStore) Close() {
	for _, db := range s.dbs {
		_ = db.Close()
	}
}
