// Package agency es el componente "Adapter Agencias: traduce la API de agencias al modelo del puerto [ADR-05][QA-14]".
package agency

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/providerport"
)

// Client es la API de la agencia que el adapter consume (la implementa StubAgencyAPI).
type Client interface {
	Bookings(ctx context.Context, travelerID string, since time.Time) ([]byte, error)
}

// Adapter implementa ports.ExternalProviderPort para proveedores de tipo AGENCIA: traduce el JSON anidado de la
// agencia (reservas → producto → fechas) al modelo del puerto. [ADR-05][QA-14]
type Adapter struct{ client Client }

var _ ports.ExternalProviderPort = (*Adapter)(nil)

// NewAdapter crea el adapter.
func NewAdapter(client Client) *Adapter { return &Adapter{client: client} }

// ProviderType devuelve el tipo de proveedor que atiende.
func (*Adapter) ProviderType() domain.ProviderType { return domain.ProviderAgency }

// Poll sondea la agencia y traduce su respuesta.
func (a *Adapter) Poll(ctx context.Context, ref ports.ProviderReference, since time.Time) ([]events.ProviderUpdateNormalized, error) {
	raw, err := a.client.Bookings(ctx, ref.TravelerID, since)
	if err != nil {
		return nil, providerport.ClassifyError(err)
	}
	return Parse(ref, raw)
}

type response struct {
	Reservas []booking `json:"reservas"`
}

type booking struct {
	Localizador string `json:"localizador"`
	Producto    struct {
		Categoria string `json:"categoria"`
		Hotel     *struct {
			Nombre    string `json:"nombre"`
			Direccion string `json:"direccion"`
			Estrellas int    `json:"estrellas"`
		} `json:"hotel"`
		Vuelo *struct {
			Aerolinea string `json:"aerolinea"`
			Numero    string `json:"numero"`
			Origen    string `json:"origen"`
			Destino   string `json:"destino"`
		} `json:"vuelo"`
		Auto *struct {
			Empresa    string `json:"empresa"`
			Retiro     string `json:"retiro"`
			Devolucion string `json:"devolucion"`
		} `json:"auto"`
	} `json:"producto"`
	Fechas struct {
		Desde time.Time `json:"desde"`
		Hasta time.Time `json:"hasta"`
	} `json:"fechas"`
	Estado     string    `json:"estado"`
	GeneradoEn time.Time `json:"generadoEn"`
}

// Parse traduce el JSON de la agencia; JSON inválido o reservas sin producto/fechas → ErrUnparseableResponse.
func Parse(ref ports.ProviderReference, raw []byte) ([]events.ProviderUpdateNormalized, error) {
	var resp response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("%w: %v", ports.ErrUnparseableResponse, err)
	}
	var out []events.ProviderUpdateNormalized
	for _, b := range resp.Reservas {
		u, err := toUpdate(ref, b)
		if err != nil {
			return nil, fmt.Errorf("%w: reserva %q: %v", ports.ErrUnparseableResponse, b.Localizador, err)
		}
		out = append(out, u)
	}
	return out, nil
}

func toUpdate(ref ports.ProviderReference, b booking) (events.ProviderUpdateNormalized, error) {
	if b.Fechas.Desde.IsZero() {
		return events.ProviderUpdateNormalized{}, fmt.Errorf("sin fecha de inicio")
	}
	detail := map[string]any{"referencia": b.Localizador}
	var tipo string
	switch strings.ToLower(b.Producto.Categoria) {
	case "hotel":
		h := b.Producto.Hotel
		if h == nil {
			return events.ProviderUpdateNormalized{}, fmt.Errorf("producto hotel sin datos")
		}
		tipo = "ALOJAMIENTO"
		detail["titulo"], detail["hotel"], detail["direccion"], detail["estrellas"] = h.Nombre, h.Nombre, h.Direccion, h.Estrellas
	case "vuelo":
		v := b.Producto.Vuelo
		if v == nil {
			return events.ProviderUpdateNormalized{}, fmt.Errorf("producto vuelo sin datos")
		}
		tipo = "VUELO"
		detail["titulo"] = "Vuelo " + v.Numero
		detail["aerolinea"], detail["numeroVuelo"], detail["origenIATA"], detail["destinoIATA"] = v.Aerolinea, v.Numero, v.Origen, v.Destino
	case "auto":
		c := b.Producto.Auto
		if c == nil {
			return events.ProviderUpdateNormalized{}, fmt.Errorf("producto auto sin datos")
		}
		tipo = "AUTO"
		detail["titulo"], detail["empresa"], detail["retiro"], detail["devolucion"] = "Auto "+c.Empresa, c.Empresa, c.Retiro, c.Devolucion
	default:
		return events.ProviderUpdateNormalized{}, fmt.Errorf("categoría desconocida %q", b.Producto.Categoria)
	}
	var estado string
	switch strings.ToLower(b.Estado) {
	case "ok":
		estado = "CONFIRMADO"
	case "modificada":
		estado = "MODIFICADO"
	case "cancelada":
		estado = "CANCELADO"
	default:
		return events.ProviderUpdateNormalized{}, fmt.Errorf("estado desconocido %q", b.Estado)
	}
	end := b.Fechas.Hasta
	if end.IsZero() {
		end = b.Fechas.Desde
	}
	return events.ProviderUpdateNormalized{
		IDViajero:        ref.TravelerID,
		Proveedor:        events.ProviderRef{Tipo: string(domain.ProviderAgency), IDProveedor: ref.ProviderID},
		GeneradoEnFuente: b.GeneradoEn.UTC(),
		Arreglo: events.NormalizedArrangement{
			Tipo: tipo, Estado: estado, FechaInicio: b.Fechas.Desde.UTC(), FechaFin: end.UTC(), Detalle: detail,
		},
	}, nil
}
