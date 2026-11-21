package domain

// ArrangementTypeRegistry registra los tipos de arreglo soportados.
// [QA-10] Agregar un nuevo medio de transporte = registrar un tipo, sin tocar la lógica central.
type ArrangementTypeRegistry struct {
	factories map[ArrangementType]func() ArrangementDetail
}

// NewArrangementTypeRegistry crea un registro vacío. Los tipos base se registran en la composición (Fase 3).
func NewArrangementTypeRegistry() *ArrangementTypeRegistry {
	return &ArrangementTypeRegistry{factories: map[ArrangementType]func() ArrangementDetail{}}
}

// Register asocia un tipo con la fábrica de su detalle.
func (r *ArrangementTypeRegistry) Register(t ArrangementType, factory func() ArrangementDetail) {
	r.factories[t] = factory
}

// NewDetail crea un detalle vacío del tipo indicado; ok es false si el tipo no está registrado.
func (r *ArrangementTypeRegistry) NewDetail(t ArrangementType) (ArrangementDetail, bool) {
	f, ok := r.factories[t]
	if !ok {
		return nil, false
	}
	return f(), true
}
