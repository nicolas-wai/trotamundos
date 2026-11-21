// Package broker es el componente "Broker (middleware con normalización)" (AD §4.4).
package broker

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
)

// ErrNotNormalizable: la actualización no se puede llevar al formato E1 (la lógica nunca recibe datos sin formatear).
var ErrNotNormalizable = errors.New("actualización no normalizable")

// identityFields son los campos del detalle que identifican una reserva por tipo; con la fecha de inicio forman
// la claveDeduplicacion (US01). Agregar un tipo de arreglo = agregar su entrada (QA-10).
var identityFields = map[string][]string{
	"VUELO":       {"numeroVuelo"},
	"ALOJAMIENTO": {"hotel"},
	"AUTO":        {"empresa", "retiro"},
	"TREN":        {"operador", "origen", "destino"},
	"BUS":         {"operador", "origen", "destino"},
}

// Broker [ADR-06][ADR-04] Recibe datos del puerto único, normaliza a E1 (claveDeduplicacion, generadoEnFuente,
// recibidoEn, origenRemitente) y publica en la cola P2P. La lógica nunca recibe datos sin formatear.
type Broker struct {
	queue ports.MessageQueuePort
	now   func() time.Time
	newID func() string
}

// NewBroker crea el broker.
func NewBroker(queue ports.MessageQueuePort) *Broker {
	return &Broker{queue: queue, now: time.Now, newID: newUUID}
}

// SetClock inyecta el reloj (pruebas).
func (b *Broker) SetClock(now func() time.Time) { b.now = now }

// Ingest normaliza y encola las actualizaciones recibidas de un adapter. Las que no se pueden normalizar se
// descartan (no llegan a la cola) y se reportan en el error devuelto; el resto se publica igual.
func (b *Broker) Ingest(ctx context.Context, updates []events.ProviderUpdateNormalized) error {
	var errs []error
	for _, raw := range updates {
		e1, err := b.Normalize(raw)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := b.queue.Publish(ctx, e1); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Normalize lleva una actualización en formato del puerto al E1 del contrato: asigna eventId y recibidoEn, deduce
// tipoEvento del estado, calcula la claveDeduplicacion canónica (igual para todas las fuentes) y valida los campos.
func (b *Broker) Normalize(u events.ProviderUpdateNormalized) (events.ProviderUpdateNormalized, error) {
	a := u.Arreglo
	a.Tipo = strings.ToUpper(strings.TrimSpace(a.Tipo))
	a.Estado = strings.ToUpper(strings.TrimSpace(a.Estado))
	switch {
	case strings.TrimSpace(u.IDViajero) == "":
		return u, fmt.Errorf("%w: sin idViajero", ErrNotNormalizable)
	case u.Proveedor.Tipo != "EMAIL" && u.Proveedor.Tipo != "AGENCIA" && u.Proveedor.Tipo != "VIAJES":
		return u, fmt.Errorf("%w: tipo de proveedor %q", ErrNotNormalizable, u.Proveedor.Tipo)
	case identityFields[a.Tipo] == nil:
		return u, fmt.Errorf("%w: tipo de arreglo %q", ErrNotNormalizable, a.Tipo)
	case a.FechaInicio.IsZero():
		return u, fmt.Errorf("%w: sin fecha de inicio", ErrNotNormalizable)
	}
	switch a.Estado {
	case "CONFIRMADO":
		u.TipoEvento = "ARREGLO_CREADO"
	case "MODIFICADO":
		u.TipoEvento = "ARREGLO_MODIFICADO"
	case "CANCELADO":
		u.TipoEvento = "ARREGLO_CANCELADO"
	default:
		return u, fmt.Errorf("%w: estado %q", ErrNotNormalizable, a.Estado)
	}
	a.FechaInicio = a.FechaInicio.UTC()
	if a.FechaFin.IsZero() || a.FechaFin.Before(a.FechaInicio) {
		a.FechaFin = a.FechaInicio
	}
	a.FechaFin = a.FechaFin.UTC()
	key, ok := deduplicationKey(a)
	if !ok {
		return u, fmt.Errorf("%w: el arreglo no trae datos para identificarlo", ErrNotNormalizable)
	}
	a.ClaveDeduplicacion = key
	if a.Detalle == nil {
		a.Detalle = map[string]any{}
	}

	now := b.now().UTC()
	u.Arreglo = a
	u.EventID = b.newID()
	u.RecibidoEn = now
	if u.GeneradoEnFuente.IsZero() {
		u.GeneradoEnFuente = now
	}
	u.GeneradoEnFuente = u.GeneradoEnFuente.UTC()
	// El remitente solo aplica al correo (US03).
	if u.Proveedor.Tipo != "EMAIL" || u.OrigenRemitente == nil || strings.TrimSpace(*u.OrigenRemitente) == "" {
		u.OrigenRemitente = nil
	} else {
		sender := strings.ToLower(strings.TrimSpace(*u.OrigenRemitente))
		u.OrigenRemitente = &sender
	}
	return u, nil
}

// deduplicationKey arma "TIPO|identidad|YYYY-MM-DD" con la identidad canonicalizada (minúsculas, sin signos ni
// espacios), de modo que "SA 123", "SA-123" y "sa123" den la misma clave sin importar la fuente.
func deduplicationKey(a events.NormalizedArrangement) (string, bool) {
	var parts []string
	for _, f := range identityFields[a.Tipo] {
		if v, _ := a.Detalle[f].(string); canonical(v) != "" {
			parts = append(parts, canonical(v))
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return a.Tipo + "|" + strings.Join(parts, "-") + "|" + a.FechaInicio.Format(time.DateOnly), true
}

func canonical(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
