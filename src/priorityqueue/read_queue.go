package priorityqueue

// ReadQueue [ADR-09] Cola de lecturas: prioridad alta, FIFO. Se despacha antes que la de escrituras.
type ReadQueue struct{}

// NewReadQueue crea la cola de lecturas.
func NewReadQueue() *ReadQueue { return &ReadQueue{} }
