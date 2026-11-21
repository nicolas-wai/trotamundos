package priorityqueue

// WriteQueue [ADR-09] Cola de escrituras: prioridad baja, FIFO. Se despacha cuando no hay lecturas pendientes.
type WriteQueue struct{}

// NewWriteQueue crea la cola de escrituras.
func NewWriteQueue() *WriteQueue { return &WriteQueue{} }
