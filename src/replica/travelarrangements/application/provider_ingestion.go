package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/events"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// Tipos de evento y estados de E1 (AD §6.3).
const (
	eventCreated   = "ARREGLO_CREADO"
	eventModified  = "ARREGLO_MODIFICADO"
	eventCancelled = "ARREGLO_CANCELADO"
)

// WithProviderNotices habilita la persistencia de avisos E2 (avisosProveedores). [ADR-07]
func (s *Service) WithProviderNotices(repo ProviderNoticeRepository) *Service {
	s.notices = repo
	return s
}

// ApplyProviderUpdate aplica E1 [ADR-04]: crea, modifica o cancela el arreglo. Un arreglo recibido de varias
// fuentes se identifica por claveDeduplicacion y se fusiona en lugar de duplicarse (US01).
// El origen y el remitente quedan marcados (US03) y recibidoEn se guarda para medir QA-01.
func (s *Service) ApplyProviderUpdate(ctx context.Context, u events.ProviderUpdateNormalized) error {
	if err := s.validateUpdate(u); err != nil {
		return err
	}
	a := u.Arreglo
	sender := ""
	if u.OrigenRemitente != nil {
		sender = strings.TrimSpace(*u.OrigenRemitente)
	}
	incoming := cloneMap(a.Detalle)
	title, _ := incoming["titulo"].(string)
	delete(incoming, "titulo")
	key := domain.DeduplicationKey(a.ClaveDeduplicacion)

	existing, err := s.arrangements.FindByDeduplicationKey(ctx, u.IDViajero, key)
	switch {
	case err == nil:
		return s.mergeUpdate(ctx, existing, u, title, sender, incoming)
	case !isNotFound(err):
		return err
	}

	detail, err := s.registry.DecodeDetail(domain.ArrangementType(a.Tipo), mustJSON(incoming), false)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	tripID, err := s.resolveTrip(ctx, u.IDViajero, a.FechaInicio)
	if err != nil {
		return err
	}
	id, err := newID()
	if err != nil {
		return err
	}
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprintf("%s (%s)", a.Tipo, u.Proveedor.Tipo)
	}
	return s.arrangements.Save(ctx, domain.TravelArrangement{
		ArrangementID: id, TripID: tripID, TravelerID: u.IDViajero, Type: domain.ArrangementType(a.Tipo),
		Origin: originFor(u.Proveedor.Tipo), Sender: domain.Sender(sender), Status: domain.Status(a.Estado),
		Title: title, StartAt: a.FechaInicio.UTC(), EndAt: a.FechaFin.UTC(), Detail: detail, DeduplicationKey: key,
		GeneratedAtSource: u.GeneradoEnFuente.UTC(), ReceivedAt: u.RecibidoEn.UTC(), UpdatedAt: s.now().UTC(),
	})
}

// mergeUpdate fusiona una actualización sobre un arreglo ya conocido (deduplicación, US01): conserva id, viaje,
// origen y título; completa remitente y detalle faltantes; y aplica los cambios de estado (modificado o cancelado).
func (s *Service) mergeUpdate(ctx context.Context, cur domain.TravelArrangement, u events.ProviderUpdateNormalized, title, sender string, incoming map[string]any) error {
	changing := u.TipoEvento == eventModified || u.TipoEvento == eventCancelled

	merged := map[string]any{}
	if cur.Detail != nil {
		_ = json.Unmarshal(mustJSON(cur.Detail), &merged)
	}
	for k, v := range incoming {
		if _, exists := merged[k]; !exists || changing {
			merged[k] = v
		}
	}
	detail, err := s.registry.DecodeDetail(cur.Type, mustJSON(merged), false)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	cur.Detail = detail
	if cur.Title == "" {
		cur.Title = title
	}
	if cur.Sender == "" && sender != "" {
		cur.Sender = domain.Sender(sender)
	}
	switch u.TipoEvento {
	case eventModified:
		cur.Status = domain.StatusModified
	case eventCancelled:
		cur.Status = domain.StatusCancelled
	}
	if changing {
		cur.StartAt, cur.EndAt = u.Arreglo.FechaInicio.UTC(), u.Arreglo.FechaFin.UTC()
	}
	// QA-01: se conserva la primera vez que el arreglo estuvo disponible para medir la latencia de punta a punta.
	if cur.GeneratedAtSource.IsZero() || (!u.GeneradoEnFuente.IsZero() && u.GeneradoEnFuente.Before(cur.GeneratedAtSource)) {
		cur.GeneratedAtSource = u.GeneradoEnFuente.UTC()
	}
	if cur.ReceivedAt.IsZero() {
		cur.ReceivedAt = u.RecibidoEn.UTC()
	}
	cur.UpdatedAt = s.now().UTC()
	return s.arrangements.Save(ctx, cur)
}

func (s *Service) validateUpdate(u events.ProviderUpdateNormalized) error {
	a := u.Arreglo
	switch {
	case strings.TrimSpace(u.IDViajero) == "":
		return fmt.Errorf("%w: E1 sin idViajero", ErrValidation)
	case !s.registry.Supports(domain.ArrangementType(a.Tipo)):
		return fmt.Errorf("%w: tipo de arreglo no soportado: %q", ErrValidation, a.Tipo)
	case strings.TrimSpace(a.ClaveDeduplicacion) == "":
		return fmt.Errorf("%w: E1 sin claveDeduplicacion", ErrValidation)
	case a.FechaInicio.IsZero():
		return fmt.Errorf("%w: E1 sin fechaInicio", ErrValidation)
	case u.TipoEvento != eventCreated && u.TipoEvento != eventModified && u.TipoEvento != eventCancelled:
		return fmt.Errorf("%w: tipoEvento desconocido: %q", ErrValidation, u.TipoEvento)
	}
	switch domain.Status(a.Estado) {
	case domain.StatusConfirmed, domain.StatusModified, domain.StatusCancelled:
	default:
		return fmt.Errorf("%w: estado desconocido: %q", ErrValidation, a.Estado)
	}
	return nil
}

// resolveTrip elige el viaje al que pertenece un arreglo recibido de un proveedor (E1 no trae idViaje):
// el viaje vigente que contiene su fecha de inicio; si no hay, el primer viaje vigente; si el viajero no tiene
// ninguno, se provisiona el viaje demo ◇.
func (s *Service) resolveTrip(ctx context.Context, travelerID string, start time.Time) (string, error) {
	trips, err := s.trips.ListByTraveler(ctx, travelerID)
	if err != nil {
		return "", err
	}
	var active []domain.Trip
	for _, t := range trips {
		if t.Status != domain.TripCompleted {
			active = append(active, t)
		}
	}
	sort.SliceStable(active, func(i, j int) bool { return active[i].StartDate.Before(active[j].StartDate) })
	day := startOfDay(start)
	for _, t := range active {
		if !day.Before(t.StartDate) && !day.After(t.EndDate) {
			return t.TripID, nil
		}
	}
	if len(active) > 0 {
		return active[0].TripID, nil
	}
	if err := s.ProvisionDemoTrip(ctx, travelerID); err != nil {
		return "", err
	}
	return DemoTripID(travelerID), nil
}

// ReportProviderDegraded registra E2 para exponerlo como avisosProveedores [ADR-07]: ABIERTO crea el aviso y
// CERRADO (el proveedor se recuperó) lo retira.
func (s *Service) ReportProviderDegraded(ctx context.Context, d events.ProviderDegraded) error {
	if s.notices == nil {
		return nil
	}
	if d.EstadoCircuito != "ABIERTO" {
		return s.notices.Clear(ctx, d.IDProveedor)
	}
	rec := ProviderNoticeRecord{ProviderID: d.IDProveedor, Message: "No se pudo conectar con el proveedor", RetryAt: d.ProximoIntento}
	if d.IDViajero != nil {
		rec.TravelerID = *d.IDViajero
	}
	return s.notices.Save(ctx, rec)
}

// providerNotices devuelve los avisos que aplican al viajero (globales o propios).
func (s *Service) providerNotices(ctx context.Context, travelerID string) ([]ProviderNotice, error) {
	out := []ProviderNotice{}
	if s.notices == nil {
		return out, nil
	}
	recs, err := s.notices.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		if r.TravelerID == "" || r.TravelerID == travelerID {
			out = append(out, ProviderNotice{Provider: providerDisplayName(r.ProviderID), Message: r.Message})
		}
	}
	return out, nil
}

func originFor(providerType string) domain.Origin {
	switch providerType {
	case "EMAIL":
		return domain.OriginEmail
	case "AGENCIA":
		return domain.OriginAgency
	case "VIAJES":
		return domain.OriginTravelSystem
	}
	return domain.OriginManual
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func mustJSON(v any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}

// providerDisplayName convierte el id de proveedor ("agencia:agencia-sol") en un nombre legible ("Agencia Sol").
func providerDisplayName(providerID string) string {
	_, name, ok := strings.Cut(providerID, ":")
	if !ok {
		name = providerID
	}
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
