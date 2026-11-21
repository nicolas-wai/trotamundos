// Package travelsystem es el componente "Adapter Sistemas de viajes: traduce la API de sistemas de viajes al modelo del puerto [ADR-05][QA-14]".
package travelsystem

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

// Client es la API del sistema de viajes que el adapter consume (la implementa StubTravelSystemAPI).
type Client interface {
	Records(ctx context.Context, travelerID string, since time.Time) ([]byte, error)
}

// TravelSystemAdapter implementa ports.ExternalProviderPort para proveedores de tipo VIAJES: traduce los registros del sistema
// de viajes (códigos IATA, estados HK/TK/UC) al modelo del puerto. [ADR-05][QA-14]
type TravelSystemAdapter struct{ client Client }

var _ ports.ExternalProviderPort = (*TravelSystemAdapter)(nil)

// NewTravelSystemAdapter crea el adapter.
func NewTravelSystemAdapter(client Client) *TravelSystemAdapter {
	return &TravelSystemAdapter{client: client}
}

// ProviderType devuelve el tipo de proveedor que atiende.
func (*TravelSystemAdapter) ProviderType() domain.ProviderType { return domain.ProviderTravelSystem }

// Poll sondea el sistema de viajes y traduce su respuesta.
func (a *TravelSystemAdapter) Poll(ctx context.Context, ref ports.ProviderReference, since time.Time) ([]events.ProviderUpdateNormalized, error) {
	raw, err := a.client.Records(ctx, ref.TravelerID, since)
	if err != nil {
		return nil, providerport.ClassifyError(err)
	}
	return Parse(ref, raw)
}

type response struct {
	Records []record `json:"records"`
}

type record struct {
	PNR      string    `json:"pnr"`
	Type     string    `json:"type"`
	Status   string    `json:"status"`
	IssuedAt time.Time `json:"issuedAt"`
	Segments []struct {
		Carrier string    `json:"carrier"`
		Number  string    `json:"number"`
		Dep     string    `json:"dep"`
		Arr     string    `json:"arr"`
		DepTime time.Time `json:"depTime"`
		ArrTime time.Time `json:"arrTime"`
	} `json:"segments"`
}

// Parse traduce el JSON del sistema de viajes; JSON inválido o registros sin segmentos → ErrUnparseableResponse.
func Parse(ref ports.ProviderReference, raw []byte) ([]events.ProviderUpdateNormalized, error) {
	var resp response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("%w: %v", ports.ErrUnparseableResponse, err)
	}
	var out []events.ProviderUpdateNormalized
	for _, r := range resp.Records {
		u, err := toUpdate(ref, r)
		if err != nil {
			return nil, fmt.Errorf("%w: registro %q: %v", ports.ErrUnparseableResponse, r.PNR, err)
		}
		out = append(out, u)
	}
	return out, nil
}

func toUpdate(ref ports.ProviderReference, r record) (events.ProviderUpdateNormalized, error) {
	if len(r.Segments) == 0 || r.Segments[0].DepTime.IsZero() {
		return events.ProviderUpdateNormalized{}, fmt.Errorf("sin segmentos")
	}
	first, last := r.Segments[0], r.Segments[len(r.Segments)-1]
	detail := map[string]any{"referencia": r.PNR}
	var tipo string
	switch strings.ToUpper(r.Type) {
	case "AIR":
		tipo = "VUELO"
		detail["titulo"] = fmt.Sprintf("Vuelo %s%s %s-%s", first.Carrier, first.Number, first.Dep, last.Arr)
		detail["aerolinea"], detail["numeroVuelo"] = first.Carrier, first.Carrier+first.Number
		detail["origenIATA"], detail["destinoIATA"] = first.Dep, last.Arr
	case "BUS":
		tipo = "BUS"
		detail["titulo"] = fmt.Sprintf("Bus %s %s-%s", first.Carrier, first.Dep, last.Arr)
		detail["operador"], detail["origen"], detail["destino"] = first.Carrier, first.Dep, last.Arr
	case "TRAIN":
		tipo = "TREN"
		detail["titulo"] = fmt.Sprintf("Tren %s %s-%s", first.Carrier, first.Dep, last.Arr)
		detail["operador"], detail["origen"], detail["destino"] = first.Carrier, first.Dep, last.Arr
	default:
		return events.ProviderUpdateNormalized{}, fmt.Errorf("tipo desconocido %q", r.Type)
	}
	var estado string
	switch strings.ToUpper(r.Status) {
	case "HK":
		estado = "CONFIRMADO"
	case "TK":
		estado = "MODIFICADO"
	case "UC":
		estado = "CANCELADO"
	default:
		return events.ProviderUpdateNormalized{}, fmt.Errorf("estado desconocido %q", r.Status)
	}
	return events.ProviderUpdateNormalized{
		IDViajero:        ref.TravelerID,
		Proveedor:        events.ProviderRef{Tipo: string(domain.ProviderTravelSystem), IDProveedor: ref.ProviderID},
		GeneradoEnFuente: r.IssuedAt.UTC(),
		Arreglo: events.NormalizedArrangement{
			Tipo: tipo, Estado: estado, FechaInicio: first.DepTime.UTC(), FechaFin: last.ArrTime.UTC(), Detalle: detail,
		},
	}, nil
}
