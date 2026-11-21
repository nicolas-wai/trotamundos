// Package operations contiene el catálogo de operaciones nombradas del nodo de datos.
package operations

// Kind distingue lecturas de escrituras (E8).
type Kind string

// Tipos de operación.
const (
	Read  Kind = "LECTURA"
	Write Kind = "ESCRITURA"
)

// Operation asocia un recurso de E8 con su SQL plano y su tipo.
type Operation struct {
	Resource string
	Kind     Kind
	SQL      string
}

// Catalog [ADR-10] Catálogo de operaciones nombradas (recurso → SQL). Se puebla en la Fase 2.
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
