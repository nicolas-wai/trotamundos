package adapters_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/adapters/agency"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/adapters/email"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/adapters/travelsystem"
	"github.com/nicolas-wai/trotamundos/src/stubs"
)

type fixedToken string

func (f fixedToken) AccessToken(context.Context, string, string) (string, error) {
	return string(f), nil
}

var ref = ports.ProviderReference{ProviderID: "x:y", TravelerID: "v1", AccountID: "c1"}

func byTipo(t *testing.T, us []events.ProviderUpdateNormalized) map[string]events.ProviderUpdateNormalized {
	t.Helper()
	m := map[string]events.ProviderUpdateNormalized{}
	for _, u := range us {
		m[u.Arreglo.Tipo] = u
	}
	return m
}

// Cada formato de stub se traduce al modelo del puerto (ADR-05, QA-14, QA-15).
func TestEmailAdapterTranslatesSemiStructuredText(t *testing.T) {
	api := stubs.NewStubEmailProviderAPI()
	a := email.NewAdapter(api, fixedToken(stubs.AccessTokenPrefix+"gmail-ana"))
	got, err := a.Poll(context.Background(), ref, time.Time{})
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %d", err, len(got))
	}
	m := byTipo(t, got)
	f := m["VUELO"]
	if f.Proveedor.Tipo != "EMAIL" || f.IDViajero != "v1" || f.OrigenRemitente == nil || *f.OrigenRemitente != "reservas@solair.example" ||
		f.Arreglo.Estado != "CONFIRMADO" || f.Arreglo.Detalle["numeroVuelo"] != "SA123" || f.Arreglo.Detalle["origenIATA"] != "EZE" ||
		f.Arreglo.Detalle["destinoIATA"] != "MAD" || f.Arreglo.Detalle["aerolinea"] != "Sol Air" || f.GeneradoEnFuente.IsZero() {
		t.Fatalf("vuelo mal traducido: %+v", f)
	}
	if c := m["AUTO"]; c.Arreglo.Detalle["empresa"] != "Ruta Car" || c.Arreglo.Detalle["retiro"] != "Aeropuerto de Barajas" {
		t.Fatalf("auto mal traducido: %+v", c)
	}
	// Con since posterior a la generación no hay novedades.
	if got, err := a.Poll(context.Background(), ref, time.Now().Add(time.Hour)); err != nil || len(got) != 0 {
		t.Fatalf("sin novedades: %v %d", err, len(got))
	}
}

func TestEmailAdapterErrors(t *testing.T) {
	api := stubs.NewStubEmailProviderAPI()
	if _, err := email.NewAdapter(api, fixedToken("token-ajeno")).Poll(context.Background(), ref, time.Time{}); !errors.Is(err, ports.ErrInvalidCredential) {
		t.Fatalf("token inválido → CredencialInvalida, fue %v", err)
	}
	good := email.NewAdapter(api, fixedToken(stubs.AccessTokenPrefix+"x"))
	api.SetFailureMode(stubs.FailMalformed)
	if _, err := good.Poll(context.Background(), ref, time.Time{}); !errors.Is(err, ports.ErrUnparseableResponse) {
		t.Fatalf("respuesta malformada → RespuestaNoParseable, fue %v", err)
	}
	api.SetFailureMode(stubs.FailUnavailable)
	if _, err := good.Poll(context.Background(), ref, time.Time{}); !errors.Is(err, ports.ErrProviderUnavailable) {
		t.Fatalf("falla → ProveedorNoDisponible, fue %v", err)
	}
	api.SetFailureMode(stubs.FailTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := good.Poll(ctx, ref, time.Time{}); !errors.Is(err, ports.ErrProviderUnavailable) {
		t.Fatalf("timeout → ProveedorNoDisponible, fue %v", err)
	}
	// Un bloque al que le faltan campos obligatorios no es parseable.
	if _, err := email.Parse(ref, "From: a@b.c\nSubject: hola"); !errors.Is(err, ports.ErrUnparseableResponse) {
		t.Fatalf("bloque incompleto: %v", err)
	}
}

func TestAgencyAdapterTranslatesNestedJSON(t *testing.T) {
	a := agency.NewAdapter(stubs.NewStubAgencyAPI())
	got, err := a.Poll(context.Background(), ref, time.Time{})
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %d", err, len(got))
	}
	h := got[0]
	if h.Proveedor.Tipo != "AGENCIA" || h.Arreglo.Tipo != "ALOJAMIENTO" || h.Arreglo.Estado != "CONFIRMADO" ||
		h.Arreglo.Detalle["hotel"] != "Hotel Gran Vía" || h.Arreglo.Detalle["estrellas"] != 4 || h.Arreglo.Detalle["titulo"] != "Hotel Gran Vía" ||
		h.Arreglo.FechaInicio.IsZero() || !h.Arreglo.FechaFin.After(h.Arreglo.FechaInicio) || h.OrigenRemitente != nil {
		t.Fatalf("hotel mal traducido: %+v", h)
	}
}

func TestAgencyAdapterMalformedAndUnavailable(t *testing.T) {
	api := stubs.NewStubAgencyAPI()
	a := agency.NewAdapter(api)
	api.SetFailureMode(stubs.FailMalformed)
	if _, err := a.Poll(context.Background(), ref, time.Time{}); !errors.Is(err, ports.ErrUnparseableResponse) {
		t.Fatalf("JSON truncado: %v", err)
	}
	api.SetFailureMode(stubs.FailUnavailable)
	if _, err := a.Poll(context.Background(), ref, time.Time{}); !errors.Is(err, ports.ErrProviderUnavailable) {
		t.Fatalf("503: %v", err)
	}
	if _, err := agency.Parse(ref, []byte(`{"reservas":[{"localizador":"X","producto":{"categoria":"hotel"},"estado":"ok"}]}`)); !errors.Is(err, ports.ErrUnparseableResponse) {
		t.Fatalf("reserva sin fechas ni producto: %v", err)
	}
}

func TestTravelSystemAdapterTranslatesIATARecords(t *testing.T) {
	a := travelsystem.NewAdapter(stubs.NewStubTravelSystemAPI())
	got, err := a.Poll(context.Background(), ref, time.Time{})
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %d", err, len(got))
	}
	m := byTipo(t, got)
	f := m["VUELO"]
	if f.Proveedor.Tipo != "VIAJES" || f.Arreglo.Estado != "CONFIRMADO" || f.Arreglo.Detalle["numeroVuelo"] != "SA123" ||
		f.Arreglo.Detalle["origenIATA"] != "EZE" || f.Arreglo.Detalle["destinoIATA"] != "MAD" {
		t.Fatalf("vuelo mal traducido: %+v", f)
	}
	// Mismo instante que el correo aunque llegue con hora local -03:00: todo se normaliza a UTC.
	if h, m2, _ := f.Arreglo.FechaInicio.Clock(); h != 10 || m2 != 0 || f.Arreglo.FechaInicio.Location() != time.UTC {
		t.Fatalf("fecha sin normalizar a UTC: %v", f.Arreglo.FechaInicio)
	}
	if b := m["BUS"]; b.Arreglo.Detalle["operador"] != "Expreso Norte" || b.Arreglo.Detalle["origen"] != "MAD" || b.Arreglo.Detalle["destino"] != "BCN" {
		t.Fatalf("bus mal traducido: %+v", b)
	}
}

func TestTravelSystemAdapterMalformed(t *testing.T) {
	api := stubs.NewStubTravelSystemAPI()
	a := travelsystem.NewAdapter(api)
	api.SetFailureMode(stubs.FailMalformed)
	if _, err := a.Poll(context.Background(), ref, time.Time{}); !errors.Is(err, ports.ErrUnparseableResponse) {
		t.Fatalf("cuerpo ilegible: %v", err)
	}
	if _, err := travelsystem.Parse(ref, []byte(`{"records":[{"pnr":"P","type":"AIR","status":"HK","segments":[]}]}`)); !errors.Is(err, ports.ErrUnparseableResponse) {
		t.Fatalf("registro sin segmentos: %v", err)
	}
}
