// Package sqlitefile encapsula un archivo SQLite (modernc.org/sqlite vía database/sql, SQL plano, sin ORM)
// usado por la base primaria y las réplicas de lectura del nodo de datos. [ADR-10]
package sqlitefile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"modernc.org/sqlite"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

// sqliteConstraint es el código primario SQLITE_CONSTRAINT.
const sqliteConstraint = 19

// DB es un archivo SQLite con su DDL. Replace permite instalar un snapshot de sincronización [ADR-21].
type DB struct {
	mu   sync.RWMutex
	path string
	ddl  string
	db   *sql.DB
}

// Open abre (o crea) el archivo y aplica el DDL.
func Open(path, ddl string) (*DB, error) {
	d := &DB{path: path, ddl: ddl}
	if err := d.open(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) open() error {
	if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", d.path)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1) // un archivo, una conexión: serializa y evita SQLITE_BUSY
	if _, err := db.Exec(d.ddl); err != nil {
		db.Close()
		return fmt.Errorf("DDL de %s: %w", d.path, err)
	}
	d.db = db
	return nil
}

// Path devuelve la ruta del archivo.
func (d *DB) Path() string { return d.path }

// Exec ejecuta una escritura y devuelve las filas afectadas. Las violaciones de unicidad se mapean a ports.ErrDataConflict.
func (d *DB) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	res, err := d.db.ExecContext(ctx, query, args...)
	if err != nil {
		var se *sqlite.Error
		if errors.As(err, &se) && se.Code()&0xff == sqliteConstraint {
			return 0, ports.ErrDataConflict
		}
		return 0, err
	}
	return res.RowsAffected()
}

// Query ejecuta una lectura y devuelve las filas como mapas columna → valor.
func (d *DB) Query(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = vals[i]
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// Snapshot copia una imagen consistente del archivo con VACUUM INTO. [ADR-21]
func (d *DB) Snapshot(ctx context.Context, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return err
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, err := d.db.ExecContext(ctx, "VACUUM INTO '"+strings.ReplaceAll(dest, "'", "''")+"'")
	return err
}

// Replace cierra el archivo, lo reemplaza por el contenido de src y lo reabre.
// move=true renombra src (se consume); move=false lo copia.
func (d *DB) Replace(src string, move bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.db.Close(); err != nil {
		return err
	}
	var err error
	if move {
		err = os.Rename(src, d.path)
	} else {
		err = copyFile(src, d.path)
	}
	if err != nil {
		_ = d.open() // intenta recuperar el archivo previo
		return err
	}
	return d.open()
}

// Close cierra el archivo.
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.Close()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
