package domain

import "testing"

func TestDefaultRegistrySupportsTheFiveContractTypes(t *testing.T) {
	r := DefaultArrangementTypeRegistry()
	for _, typ := range []ArrangementType{TypeFlight, TypeAccommodation, TypeRentalCar, TypeTrain, TypeBus} {
		d, ok := r.NewDetail(typ)
		if !ok || d.ArrangementType() != typ {
			t.Fatalf("tipo %s no registrado o con detalle incorrecto: %#v", typ, d)
		}
	}
	if r.Supports("CRUCERO") {
		t.Fatal("un tipo no registrado no debe estar soportado")
	}
}

// [QA-10] Agregar un medio de transporte = registrar un tipo, sin tocar el registro base ni la lógica central.
func TestRegistryAcceptsNewTypesWithoutChangingTheCore(t *testing.T) {
	r := DefaultArrangementTypeRegistry()
	r.Register("FERRY", func() ArrangementDetail { return &Bus{} })
	if !r.Supports("FERRY") {
		t.Fatal("el tipo registrado debe estar soportado")
	}
}

func TestDecodeDetailStrictAndLenient(t *testing.T) {
	r := DefaultArrangementTypeRegistry()
	d, err := r.DecodeDetail(TypeFlight, []byte(`{"aerolinea":"LATAM","numeroVuelo":"LA800"}`), true)
	if err != nil || d.(*Flight).Airline != "LATAM" {
		t.Fatalf("detalle válido: %#v %v", d, err)
	}
	if _, err := r.DecodeDetail(TypeFlight, []byte(`{"hotel":"x"}`), true); err == nil {
		t.Fatal("strict debe rechazar claves desconocidas")
	}
	if _, err := r.DecodeDetail(TypeFlight, []byte(`{"hotel":"x","aerolinea":"A"}`), false); err != nil {
		t.Fatalf("lenient debe tolerar claves desconocidas: %v", err)
	}
	for _, empty := range []string{``, `null`, `{}`} {
		if d, err := r.DecodeDetail(TypeBus, []byte(empty), true); err != nil || d == nil {
			t.Fatalf("detalle vacío %q: %v", empty, err)
		}
	}
	if _, err := r.DecodeDetail("CRUCERO", []byte(`{}`), true); err == nil {
		t.Fatal("tipo no registrado debe fallar")
	}
}
