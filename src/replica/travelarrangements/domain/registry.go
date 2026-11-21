package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ArrangementTypeRegistry registra los tipos de arreglo soportados.
// [QA-10] Agregar un nuevo medio de transporte = registrar un tipo, sin tocar la lógica central.
type ArrangementTypeRegistry struct {
	factories map[ArrangementType]func() ArrangementDetail
}

// NewArrangementTypeRegistry crea un registro vacío.
func NewArrangementTypeRegistry() *ArrangementTypeRegistry {
	return &ArrangementTypeRegistry{factories: map[ArrangementType]func() ArrangementDetail{}}
}

// DefaultArrangementTypeRegistry crea el registro con los cinco tipos base del contrato (AD §6.2).
// Las fábricas devuelven punteros para poder decodificar el JSON del detalle sobre ellos.
func DefaultArrangementTypeRegistry() *ArrangementTypeRegistry {
	r := NewArrangementTypeRegistry()
	r.Register(TypeAccommodation, func() ArrangementDetail { return &Accommodation{} })
	r.Register(TypeFlight, func() ArrangementDetail { return &Flight{} })
	r.Register(TypeRentalCar, func() ArrangementDetail { return &RentalCar{} })
	r.Register(TypeTrain, func() ArrangementDetail { return &Train{} })
	r.Register(TypeBus, func() ArrangementDetail { return &Bus{} })
	return r
}

// Register asocia un tipo con la fábrica de su detalle.
func (r *ArrangementTypeRegistry) Register(t ArrangementType, factory func() ArrangementDetail) {
	r.factories[t] = factory
}

// Supports indica si el tipo está registrado.
func (r *ArrangementTypeRegistry) Supports(t ArrangementType) bool {
	_, ok := r.factories[t]
	return ok
}

// NewDetail crea un detalle vacío del tipo indicado; ok es false si el tipo no está registrado.
func (r *ArrangementTypeRegistry) NewDetail(t ArrangementType) (ArrangementDetail, bool) {
	f, ok := r.factories[t]
	if !ok {
		return nil, false
	}
	return f(), true
}

// DecodeDetail construye el detalle tipado a partir de su JSON. Con strict se rechazan claves que el tipo no
// conoce (validación de altas manuales); sin strict se toleran (lectura de datos persistidos, p. ej. de proveedores).
func (r *ArrangementTypeRegistry) DecodeDetail(t ArrangementType, raw []byte, strict bool) (ArrangementDetail, error) {
	detail, ok := r.NewDetail(t)
	if !ok {
		return nil, fmt.Errorf("tipo de arreglo no registrado: %s", t)
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return detail, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(detail); err != nil {
		return nil, fmt.Errorf("detalle inválido para %s: %w", t, err)
	}
	return detail, nil
}
