// Package stubs contiene los sistemas externos simulados, determinísticos, detrás de los puertos reales.
// [Prototipo] No hay proveedores reales: cada API tiene su propio formato y fallas inyectables.
// [Prototipo] Las conexiones con proveedores (HTTPS en producción [ADR-12]) se simulan en proceso, sin red ni TLS.
package stubs

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/ports"
)

// FailureMode es la falla inyectable de un stub de proveedor. [ADR-07][QA-04]
type FailureMode int

// Modos de falla.
const (
	FailNone        FailureMode = iota
	FailUnavailable             // el proveedor responde con error (5xx simulado)
	FailTimeout                 // el proveedor no responde: bloquea hasta que vence el contexto
	FailMalformed               // responde 2xx con un cuerpo que no se puede interpretar
)

// failureControl es el estado común de falla y conteo de llamadas de los tres stubs.
type failureControl struct {
	mu          sync.Mutex
	mode        FailureMode
	calls       int
	generatedAt time.Time
}

// SetFailureMode inyecta (o retira) una falla.
func (f *failureControl) SetFailureMode(m FailureMode) { f.mu.Lock(); f.mode = m; f.mu.Unlock() }

// Calls devuelve cuántas veces se consultó al proveedor.
func (f *failureControl) Calls() int { f.mu.Lock(); defer f.mu.Unlock(); return f.calls }

// SetGeneratedAt fija el instante en que "se generaron" los datos en la fuente (mide QA-01).
func (f *failureControl) SetGeneratedAt(t time.Time) {
	f.mu.Lock()
	f.generatedAt = t.UTC()
	f.mu.Unlock()
}

func (f *failureControl) generated() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.generatedAt
}

// enter registra la llamada y aplica la falla; malformed indica que debe devolverse un cuerpo ilegible.
func (f *failureControl) enter(ctx context.Context) (malformed bool, err error) {
	f.mu.Lock()
	f.calls++
	mode := f.mode
	f.mu.Unlock()
	switch mode {
	case FailUnavailable:
		return false, fmt.Errorf("%w: el proveedor respondió 503", ports.ErrProviderUnavailable)
	case FailTimeout:
		<-ctx.Done()
		return false, fmt.Errorf("%w: sin respuesta: %v", ports.ErrProviderUnavailable, ctx.Err())
	case FailMalformed:
		return true, nil
	}
	return false, ctx.Err()
}

// Fechas de las reservas del dataset: fijas para que las pruebas sean determinísticas.
var (
	flightStart = time.Date(2027, 3, 10, 10, 0, 0, 0, time.UTC)
	flightEnd   = time.Date(2027, 3, 10, 22, 30, 0, 0, time.UTC)
)

// StubEmailProviderAPI simula la API de un proveedor de correo: devuelve los correos de reserva como texto
// semiestructurado ("Campo: valor") con remitente. Exige un access token emitido por MockOAuth2AuthAdapter. [ADR-15]
type StubEmailProviderAPI struct{ failureControl }

// NewStubEmailProviderAPI crea el stub con datos generados al momento de crearlo.
func NewStubEmailProviderAPI() *StubEmailProviderAPI {
	s := &StubEmailProviderAPI{}
	s.generatedAt = time.Now().UTC()
	return s
}

// FetchMessages devuelve los correos de reserva generados después de since. Bloques separados por una línea "---".
func (s *StubEmailProviderAPI) FetchMessages(ctx context.Context, accessToken string, since time.Time) (string, error) {
	malformed, err := s.enter(ctx)
	if err != nil {
		return "", err
	}
	if malformed {
		return "<html><body>502 Bad Gateway</body></html>", nil
	}
	if !strings.HasPrefix(accessToken, AccessTokenPrefix) {
		return "", ports.ErrInvalidCredential
	}
	gen := s.generated()
	if !gen.After(since) {
		return "", nil
	}
	date := gen.Format(time.RFC3339)
	blocks := []string{
		strings.Join([]string{
			"From: reservas@solair.example", "Date: " + date, "Subject: Confirmación de vuelo SA123 a Madrid",
			"Reserva: ZX81PQ", "Tipo: VUELO", "Estado: CONFIRMADO",
			"Inicio: " + flightStart.Format(time.RFC3339), "Fin: " + flightEnd.Format(time.RFC3339),
			"Aerolinea: Sol Air", "Vuelo: SA123", "Origen: EZE", "Destino: MAD",
		}, "\n"),
		strings.Join([]string{
			"From: alquileres@rutacar.example", "Date: " + date, "Subject: Tu auto de alquiler en Madrid",
			"Reserva: RC-5521", "Tipo: AUTO", "Estado: CONFIRMADO",
			"Inicio: 2027-03-11T09:00:00Z", "Fin: 2027-03-15T09:00:00Z",
			"Empresa: Ruta Car", "Retiro: Aeropuerto de Barajas", "Devolucion: Aeropuerto de Barajas",
		}, "\n"),
	}
	return strings.Join(blocks, "\n---\n"), nil
}

// StubAgencyAPI simula la API de una agencia: JSON anidado con reservas, productos y fechas. [ADR-05]
type StubAgencyAPI struct{ failureControl }

// NewStubAgencyAPI crea el stub con datos generados al momento de crearlo.
func NewStubAgencyAPI() *StubAgencyAPI {
	s := &StubAgencyAPI{}
	s.generatedAt = time.Now().UTC()
	return s
}

// Bookings devuelve el JSON de reservas del viajero generadas después de since.
func (s *StubAgencyAPI) Bookings(ctx context.Context, travelerID string, since time.Time) ([]byte, error) {
	malformed, err := s.enter(ctx)
	if err != nil {
		return nil, err
	}
	if malformed {
		return []byte(`{"agencia": "Agencia Sol", "reservas": [`), nil // JSON truncado
	}
	gen := s.generated()
	if !gen.After(since) {
		return []byte(`{"agencia":"Agencia Sol","reservas":[]}`), nil
	}
	body := fmt.Sprintf(`{
  "agencia": "Agencia Sol",
  "reservas": [
    {
      "localizador": "AG-7731",
      "producto": {"categoria": "hotel", "hotel": {"nombre": "Hotel Gran Vía", "direccion": "Gran Vía 28, Madrid", "estrellas": 4}},
      "fechas": {"desde": "2027-03-10T14:00:00Z", "hasta": "2027-03-15T11:00:00Z"},
      "estado": "ok",
      "generadoEn": %q
    }
  ]
}`, gen.Format(time.RFC3339))
	return []byte(body), nil
}

// StubTravelSystemAPI simula la API de un sistema de viajes (GDS): registros con códigos IATA y estados de dos letras
// (HK confirmado, TK modificado, UC cancelado). [ADR-05]
type StubTravelSystemAPI struct{ failureControl }

// NewStubTravelSystemAPI crea el stub con datos generados al momento de crearlo.
func NewStubTravelSystemAPI() *StubTravelSystemAPI {
	s := &StubTravelSystemAPI{}
	s.generatedAt = time.Now().UTC()
	return s
}

// Records devuelve el JSON de registros del viajero generados después de since.
func (s *StubTravelSystemAPI) Records(ctx context.Context, travelerID string, since time.Time) ([]byte, error) {
	malformed, err := s.enter(ctx)
	if err != nil {
		return nil, err
	}
	if malformed {
		return []byte("PNR;ZX81PQ;??;;"), nil
	}
	gen := s.generated()
	if !gen.After(since) {
		return []byte(`{"records":[]}`), nil
	}
	// La misma reserva SA123 que llega por correo, con otro formato de fecha (hora local -03:00) y otro código.
	body := fmt.Sprintf(`{
  "records": [
    {
      "pnr": "ZX81PQ", "type": "AIR", "status": "HK", "issuedAt": %[1]q,
      "segments": [{"carrier": "SA", "number": "123", "dep": "EZE", "arr": "MAD",
                    "depTime": "2027-03-10T07:00:00-03:00", "arrTime": "2027-03-10T19:30:00-03:00"}]
    },
    {
      "pnr": "BU-310", "type": "BUS", "status": "HK", "issuedAt": %[1]q,
      "segments": [{"carrier": "Expreso Norte", "number": "", "dep": "MAD", "arr": "BCN",
                    "depTime": "2027-03-12T08:00:00+01:00", "arrTime": "2027-03-12T15:30:00+01:00"}]
    }
  ]
}`, gen.Format(time.RFC3339))
	return []byte(body), nil
}
