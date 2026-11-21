// Package adapters (travelarrangements) implementa los repositorios del módulo sobre la cola de prioridades:
// lecturas con prioridad alta, escrituras con prioridad baja. [ADR-09][ADR-11]
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nicolas-wai/trotamundos/src/contracts/resources"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// QueueTripRepository [ADR-09][ADR-11] Implementa TripRepository sobre la cola de prioridades.
// Toda operación sale como E8 (DataGateway → PriorityQueuePort) hacia el shard del viajero.
type QueueTripRepository struct{ data ports.DataGateway }

var _ application.TripRepository = (*QueueTripRepository)(nil)

// NewQueueTripRepository crea el repositorio.
func NewQueueTripRepository(data ports.DataGateway) *QueueTripRepository {
	return &QueueTripRepository{data: data}
}

// ListByTraveler lee los viajes del viajero (LECTURA, prioridad alta).
func (r *QueueTripRepository) ListByTraveler(ctx context.Context, travelerID string) ([]domain.Trip, error) {
	res, err := r.data.Read(ctx, travelerID, resources.TripList, map[string]any{"idViajero": travelerID})
	if err != nil {
		return nil, err
	}
	trips := make([]domain.Trip, 0, len(res.Rows))
	for _, row := range res.Rows {
		trips = append(trips, tripFromRow(row))
	}
	return trips, nil
}

// FindByID lee un viaje del viajero; ErrNotFound si no existe o es de otro viajero.
func (r *QueueTripRepository) FindByID(ctx context.Context, travelerID, tripID string) (domain.Trip, error) {
	res, err := r.data.Read(ctx, travelerID, resources.TripFindByID, map[string]any{"idViaje": tripID, "idViajero": travelerID})
	if err != nil {
		return domain.Trip{}, err
	}
	if len(res.Rows) == 0 {
		return domain.Trip{}, application.ErrNotFound
	}
	return tripFromRow(res.Rows[0]), nil
}

// Save crea el viaje si no existe (ESCRITURA, prioridad baja).
func (r *QueueTripRepository) Save(ctx context.Context, t domain.Trip) error {
	_, err := r.data.Write(ctx, t.TravelerID, resources.TripCreate, map[string]any{
		"idViaje": t.TripID, "idViajero": t.TravelerID, "nombre": t.Name, "destino": t.Destination,
		"fechaInicio": t.StartDate.UTC().Format(time.DateOnly), "fechaFin": t.EndDate.UTC().Format(time.DateOnly),
		"estado": string(t.Status),
	})
	return err
}

// QueueArrangementRepository [ADR-09][ADR-11] Implementa ArrangementRepository sobre la cola de prioridades.
type QueueArrangementRepository struct {
	data     ports.DataGateway
	registry *domain.ArrangementTypeRegistry // [QA-10] reconstruye el detalle tipado
}

var _ application.ArrangementRepository = (*QueueArrangementRepository)(nil)

// NewQueueArrangementRepository crea el repositorio (registry nil = tipos base).
func NewQueueArrangementRepository(data ports.DataGateway, registry *domain.ArrangementTypeRegistry) *QueueArrangementRepository {
	if registry == nil {
		registry = domain.DefaultArrangementTypeRegistry()
	}
	return &QueueArrangementRepository{data: data, registry: registry}
}

// ListSummariesByTraveler lee solo datos críticos (sin detalle) para el dashboard. [ADR-17]
func (r *QueueArrangementRepository) ListSummariesByTraveler(ctx context.Context, travelerID string) ([]domain.TravelArrangement, error) {
	res, err := r.data.Read(ctx, travelerID, resources.ArrangementListSummaries, map[string]any{"idViajero": travelerID})
	if err != nil {
		return nil, err
	}
	out := make([]domain.TravelArrangement, 0, len(res.Rows))
	for _, row := range res.Rows {
		out = append(out, domain.TravelArrangement{
			ArrangementID: str(row, "id_arreglo"), TripID: str(row, "id_viaje"), TravelerID: travelerID,
			Type: domain.ArrangementType(str(row, "tipo")), Title: str(row, "titulo"),
			Origin: domain.Origin(str(row, "origen")), Status: domain.Status(str(row, "estado")),
			StartAt: parseTime(str(row, "fecha_inicio")), EndAt: parseTime(str(row, "fecha_fin")),
		})
	}
	return out, nil
}

// FindByID lee el arreglo completo; ErrNotFound si no existe o es de otro viajero.
func (r *QueueArrangementRepository) FindByID(ctx context.Context, travelerID, arrangementID string) (domain.TravelArrangement, error) {
	res, err := r.data.Read(ctx, travelerID, resources.ArrangementFindByID, map[string]any{"idArreglo": arrangementID, "idViajero": travelerID})
	if err != nil {
		return domain.TravelArrangement{}, err
	}
	if len(res.Rows) == 0 {
		return domain.TravelArrangement{}, application.ErrNotFound
	}
	return r.fromRow(res.Rows[0])
}

// FindByDeduplicationKey busca un arreglo del viajero por su clave de deduplicación (US01).
func (r *QueueArrangementRepository) FindByDeduplicationKey(ctx context.Context, travelerID string, key domain.DeduplicationKey) (domain.TravelArrangement, error) {
	res, err := r.data.Read(ctx, travelerID, resources.ArrangementFindByDeduplication, map[string]any{"claveDeduplicacion": string(key), "idViajero": travelerID})
	if err != nil {
		return domain.TravelArrangement{}, err
	}
	if len(res.Rows) == 0 {
		return domain.TravelArrangement{}, application.ErrNotFound
	}
	return r.fromRow(res.Rows[0])
}

// Save inserta o actualiza el arreglo (ESCRITURA, prioridad baja).
func (r *QueueArrangementRepository) Save(ctx context.Context, a domain.TravelArrangement) error {
	detail := []byte("{}")
	if a.Detail != nil {
		raw, err := json.Marshal(a.Detail)
		if err != nil {
			return fmt.Errorf("serializando detalle: %w", err)
		}
		detail = raw
	}
	_, err := r.data.Write(ctx, a.TravelerID, resources.ArrangementSave, map[string]any{
		"idArreglo": a.ArrangementID, "idViaje": a.TripID, "idViajero": a.TravelerID, "tipo": string(a.Type),
		"titulo": a.Title, "origen": string(a.Origin), "remitente": nullable(string(a.Sender)), "estado": string(a.Status),
		"detalle": string(detail), "claveDeduplicacion": nullable(string(a.DeduplicationKey)),
		"fechaInicio": formatTime(a.StartAt), "fechaFin": formatTime(a.EndAt),
		"generadoEnFuente": formatTime(a.GeneratedAtSource), "recibidoEn": formatTime(a.ReceivedAt), "actualizadoEn": formatTime(a.UpdatedAt),
	})
	return err
}

// Delete borra el arreglo del viajero; ErrNotFound si no existía.
func (r *QueueArrangementRepository) Delete(ctx context.Context, travelerID, arrangementID string) error {
	res, err := r.data.Write(ctx, travelerID, resources.ArrangementDelete, map[string]any{"idArreglo": arrangementID, "idViajero": travelerID})
	if err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return application.ErrNotFound
	}
	return nil
}

func (r *QueueArrangementRepository) fromRow(row map[string]any) (domain.TravelArrangement, error) {
	t := domain.ArrangementType(str(row, "tipo"))
	// Lectura tolerante: el detalle persistido puede traer claves de proveedores (Fase 4).
	detail, err := r.registry.DecodeDetail(t, []byte(str(row, "detalle")), false)
	if err != nil {
		detail = nil
	}
	return domain.TravelArrangement{
		ArrangementID: str(row, "id_arreglo"), TripID: str(row, "id_viaje"), TravelerID: str(row, "id_viajero"),
		Type: t, Title: str(row, "titulo"), Origin: domain.Origin(str(row, "origen")),
		Sender: domain.Sender(str(row, "remitente")), Status: domain.Status(str(row, "estado")),
		Detail: detail, DeduplicationKey: domain.DeduplicationKey(str(row, "clave_deduplicacion")),
		StartAt: parseTime(str(row, "fecha_inicio")), EndAt: parseTime(str(row, "fecha_fin")),
		GeneratedAtSource: parseTime(str(row, "generado_en_fuente")), ReceivedAt: parseTime(str(row, "recibido_en")), UpdatedAt: parseTime(str(row, "actualizado_en")),
	}, nil
}

func tripFromRow(row map[string]any) domain.Trip {
	return domain.Trip{
		TripID: str(row, "id_viaje"), TravelerID: str(row, "id_viajero"), Name: str(row, "nombre"),
		Destination: str(row, "destino"), StartDate: parseTime(str(row, "fecha_inicio")),
		EndDate: parseTime(str(row, "fecha_fin")), Status: domain.TripStatus(str(row, "estado")),
	}
}

func str(row map[string]any, col string) string {
	s, _ := row[col].(string)
	return s
}

// nullable guarda NULL en vez de cadena vacía (columnas opcionales).
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func formatTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// parseTime acepta fechas completas (RFC 3339) y solo-fecha (YYYY-MM-DD); "" → tiempo cero.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

// QueueProviderNoticeRepository [ADR-07][ADR-09] Implementa ProviderNoticeRepository sobre la cola de prioridades,
// en la partición directory ◇ (los avisos son por proveedor, no por viajero) para que toda réplica los vea.
type QueueProviderNoticeRepository struct{ data ports.DataGateway }

var _ application.ProviderNoticeRepository = (*QueueProviderNoticeRepository)(nil)

// NewQueueProviderNoticeRepository crea el repositorio.
func NewQueueProviderNoticeRepository(data ports.DataGateway) *QueueProviderNoticeRepository {
	return &QueueProviderNoticeRepository{data: data}
}

// Save guarda (o reemplaza) el aviso del proveedor.
func (r *QueueProviderNoticeRepository) Save(ctx context.Context, n application.ProviderNoticeRecord) error {
	_, err := r.data.Write(ctx, "", resources.DirectoryProviderNoticeSave, map[string]any{
		"idProveedor": n.ProviderID, "idViajero": nullable(n.TravelerID), "mensaje": n.Message,
		"proximoIntento": n.RetryAt.UTC().Format(time.RFC3339Nano),
	})
	return err
}

// Clear retira el aviso del proveedor.
func (r *QueueProviderNoticeRepository) Clear(ctx context.Context, providerID string) error {
	_, err := r.data.Write(ctx, "", resources.DirectoryProviderNoticeClear, map[string]any{"idProveedor": providerID})
	return err
}

// List devuelve todos los avisos vigentes.
func (r *QueueProviderNoticeRepository) List(ctx context.Context) ([]application.ProviderNoticeRecord, error) {
	res, err := r.data.Read(ctx, "", resources.DirectoryProviderNoticeList, nil)
	if err != nil {
		return nil, err
	}
	out := make([]application.ProviderNoticeRecord, 0, len(res.Rows))
	for _, row := range res.Rows {
		out = append(out, application.ProviderNoticeRecord{
			ProviderID: str(row, "id_proveedor"), TravelerID: str(row, "id_viajero"),
			Message: str(row, "mensaje"), RetryAt: parseTime(str(row, "proximo_intento")),
		})
	}
	return out, nil
}
