package priorityqueue

// WriteQueue [ADR-09] Cola de escrituras: prioridad baja, FIFO. Se despacha cuando no hay lecturas pendientes.
// Riesgo conocido: starvation bajo carga sostenida de lecturas (aceptado en ADR-09).
type WriteQueue struct{ fifo }

// NewWriteQueue crea la cola de escrituras.
func NewWriteQueue() *WriteQueue { return &WriteQueue{} }

// Len devuelve la cantidad de escrituras pendientes.
func (q *WriteQueue) Len() int { return q.len() }
