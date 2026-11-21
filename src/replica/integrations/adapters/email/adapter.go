// Package email es el componente "Adapter Email: traduce la API de correo al modelo del puerto [ADR-05][ADR-15][QA-15]".
package email

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/integrations/providerport"
)

// MailClient es la API del proveedor de correo que el adapter consume (la implementa StubEmailProviderAPI).
type MailClient interface {
	FetchMessages(ctx context.Context, accessToken string, since time.Time) (string, error)
}

// TokenResolver entrega el access token (descifrado) de una cuenta vinculada. [ADR-15]
type TokenResolver interface {
	AccessToken(ctx context.Context, travelerID, accountID string) (string, error)
}

// EmailAdapter implementa ports.ExternalProviderPort para proveedores de tipo EMAIL: traduce los correos de reserva
// (texto semiestructurado "Campo: valor", con remitente) al modelo del puerto. [ADR-05][QA-15]
type EmailAdapter struct {
	client MailClient
	tokens TokenResolver
}

var _ ports.ExternalProviderPort = (*EmailAdapter)(nil)

// NewEmailAdapter crea el adapter.
func NewEmailAdapter(client MailClient, tokens TokenResolver) *EmailAdapter {
	return &EmailAdapter{client: client, tokens: tokens}
}

// ProviderType devuelve el tipo de proveedor que atiende.
func (*EmailAdapter) ProviderType() domain.ProviderType { return domain.ProviderEmail }

// Poll sondea el correo de la cuenta vinculada y traduce los mensajes. Errores: ErrProviderUnavailable,
// ErrInvalidCredential (token rechazado o cuenta sin tokens) y ErrUnparseableResponse.
func (a *EmailAdapter) Poll(ctx context.Context, ref ports.ProviderReference, since time.Time) ([]events.ProviderUpdateNormalized, error) {
	token, err := a.tokens.AccessToken(ctx, ref.TravelerID, ref.AccountID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ports.ErrInvalidCredential, err)
	}
	raw, err := a.client.FetchMessages(ctx, token, since)
	if err != nil {
		return nil, providerport.ClassifyError(err)
	}
	return Parse(ref, raw)
}

// Parse traduce el texto del proveedor. Un cuerpo vacío no tiene novedades; un bloque sin los campos
// obligatorios (Tipo, Estado, Inicio, Date) hace que toda la respuesta sea no parseable.
func Parse(ref ports.ProviderReference, raw string) ([]events.ProviderUpdateNormalized, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []events.ProviderUpdateNormalized
	for _, block := range strings.Split(raw, "\n---\n") {
		fields := parseFields(block)
		u, err := toUpdate(ref, fields)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ports.ErrUnparseableResponse, err)
		}
		out = append(out, u)
	}
	return out, nil
}

func parseFields(block string) map[string]string {
	fields := map[string]string{}
	for _, line := range strings.Split(block, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	return fields
}

func toUpdate(ref ports.ProviderReference, f map[string]string) (events.ProviderUpdateNormalized, error) {
	for _, required := range []string{"tipo", "estado", "inicio", "date"} {
		if f[required] == "" {
			return events.ProviderUpdateNormalized{}, fmt.Errorf("falta el campo %q", required)
		}
	}
	start, err := time.Parse(time.RFC3339, f["inicio"])
	if err != nil {
		return events.ProviderUpdateNormalized{}, fmt.Errorf("Inicio inválido: %w", err)
	}
	end := start
	if f["fin"] != "" {
		if end, err = time.Parse(time.RFC3339, f["fin"]); err != nil {
			return events.ProviderUpdateNormalized{}, fmt.Errorf("Fin inválido: %w", err)
		}
	}
	generated, err := time.Parse(time.RFC3339, f["date"])
	if err != nil {
		return events.ProviderUpdateNormalized{}, fmt.Errorf("Date inválido: %w", err)
	}
	tipo := strings.ToUpper(f["tipo"])
	detail := map[string]any{"titulo": f["subject"], "referencia": f["reserva"]}
	for k, v := range f {
		switch k {
		case "from", "date", "subject", "reserva", "tipo", "estado", "inicio", "fin":
		case "vuelo":
			detail["numeroVuelo"] = v
		case "origen", "destino":
			if tipo == "VUELO" {
				detail[k+"IATA"] = v // origenIATA / destinoIATA
			} else {
				detail[k] = v
			}
		default:
			detail[k] = v
		}
	}
	sender := f["from"]
	var senderPtr *string
	if sender != "" {
		senderPtr = &sender
	}
	return events.ProviderUpdateNormalized{
		IDViajero:        ref.TravelerID,
		Proveedor:        events.ProviderRef{Tipo: string(domain.ProviderEmail), IDProveedor: ref.ProviderID},
		OrigenRemitente:  senderPtr,
		GeneradoEnFuente: generated.UTC(),
		Arreglo: events.NormalizedArrangement{
			Tipo: tipo, Estado: strings.ToUpper(f["estado"]), FechaInicio: start.UTC(), FechaFin: end.UTC(), Detalle: detail,
		},
	}, nil
}
