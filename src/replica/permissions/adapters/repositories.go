// Package adapters (permissions) implementa los repositorios del módulo sobre la cola de prioridades y el
// directorio de itinerarios ◇: lecturas con prioridad alta, escrituras con prioridad baja. [ADR-09][ADR-11]
package adapters

import (
	"context"
	"time"

	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
	"github.com/nicolas-wai/trotamundos/src/ports"
	"github.com/nicolas-wai/trotamundos/src/replica/permissions/application"
	"github.com/nicolas-wai/trotamundos/src/replica/permissions/domain"
)

// QueueItineraryDirectory [ADR-11] Implementa ItineraryDirectory en la partición global `directory` ◇:
// resuelve el dueño (y por tanto el shard) a partir de idItinerario, que el invitado conoce por el link.
type QueueItineraryDirectory struct{ data ports.DataGateway }

var _ application.ItineraryDirectory = (*QueueItineraryDirectory)(nil)

// NewQueueItineraryDirectory crea el directorio.
func NewQueueItineraryDirectory(data ports.DataGateway) *QueueItineraryDirectory {
	return &QueueItineraryDirectory{data: data}
}

// Register registra el itinerario con su dueño.
func (d *QueueItineraryDirectory) Register(ctx context.Context, itineraryID, ownerID string) error {
	_, err := d.data.Write(ctx, "", operations.DirectoryItineraryRegister, map[string]any{"idItinerario": itineraryID, "idDueno": ownerID})
	return err
}

// ResolveOwner devuelve el dueño del itinerario (ErrItineraryNotFound si no existe).
func (d *QueueItineraryDirectory) ResolveOwner(ctx context.Context, itineraryID string) (string, error) {
	res, err := d.data.Read(ctx, "", operations.DirectoryItineraryResolve, map[string]any{"idItinerario": itineraryID})
	if err != nil {
		return "", err
	}
	if len(res.Rows) == 0 {
		return "", application.ErrItineraryNotFound
	}
	return str(res.Rows[0], "id_viajero_dueno"), nil
}

// Remove elimina la entrada del directorio.
func (d *QueueItineraryDirectory) Remove(ctx context.Context, itineraryID, ownerID string) error {
	_, err := d.data.Write(ctx, "", operations.DirectoryItineraryRemove, map[string]any{"idItinerario": itineraryID, "idDueno": ownerID})
	return err
}

// QueueSharedItineraryRepository [ADR-09][ADR-11] Implementa SharedItineraryRepository sobre la cola de
// prioridades. El itinerario y todo lo suyo viven en el shard del dueño; el directorio ◇ lo resuelve.
type QueueSharedItineraryRepository struct {
	data      ports.DataGateway
	directory *QueueItineraryDirectory
}

var _ application.SharedItineraryRepository = (*QueueSharedItineraryRepository)(nil)

// NewQueueSharedItineraryRepository crea el repositorio.
func NewQueueSharedItineraryRepository(data ports.DataGateway, directory *QueueItineraryDirectory) *QueueSharedItineraryRepository {
	return &QueueSharedItineraryRepository{data: data, directory: directory}
}

// FindByID resuelve el dueño en el directorio y carga el agregado completo de su shard.
func (r *QueueSharedItineraryRepository) FindByID(ctx context.Context, itineraryID string) (domain.SharedItinerary, error) {
	ownerID, err := r.directory.ResolveOwner(ctx, itineraryID)
	if err != nil {
		return domain.SharedItinerary{}, err
	}
	key := map[string]any{"idItinerario": itineraryID, "idViajero": ownerID}
	res, err := r.data.Read(ctx, ownerID, operations.ItineraryFind, key)
	if err != nil {
		return domain.SharedItinerary{}, err
	}
	if len(res.Rows) == 0 {
		return domain.SharedItinerary{}, application.ErrItineraryNotFound
	}
	it := domain.SharedItinerary{
		ItineraryID: itineraryID, OwnerID: ownerID,
		Name: str(res.Rows[0], "nombre"), PublicLink: str(res.Rows[0], "link_publico"),
		ArrangementIDs: []string{},
	}
	if res, err = r.data.Read(ctx, ownerID, operations.ItineraryArrangementList, key); err != nil {
		return domain.SharedItinerary{}, err
	}
	for _, row := range res.Rows {
		it.ArrangementIDs = append(it.ArrangementIDs, str(row, "id_arreglo"))
	}
	if res, err = r.data.Read(ctx, ownerID, operations.AccessPermissionList, key); err != nil {
		return domain.SharedItinerary{}, err
	}
	for _, row := range res.Rows {
		it.Permissions = append(it.Permissions, domain.AccessPermission{
			AuthorizedTravelerID: str(row, "id_viajero_autorizado"), Active: str(row, "estado") == permissionActive,
		})
	}
	if res, err = r.data.Read(ctx, ownerID, operations.AccessRequestList, key); err != nil {
		return domain.SharedItinerary{}, err
	}
	for _, row := range res.Rows {
		requestedAt, _ := time.Parse(time.RFC3339Nano, str(row, "fecha_solicitud"))
		it.Requests = append(it.Requests, domain.AccessRequest{
			RequestID: str(row, "id_solicitud"), RequesterTravelerID: str(row, "id_viajero_solicitante"),
			Status: domain.RequestStatus(str(row, "estado")), RequestedAt: requestedAt,
		})
	}
	return it, nil
}

// Estados persistidos de un permiso.
const (
	permissionActive  = "ACTIVO"
	permissionRevoked = "REVOCADO"
)

// Create guarda el itinerario con sus arreglos y lo registra en el directorio. Si un paso falla compensa
// retirando lo ya escrito para no dejar un itinerario resoluble a medias.
func (r *QueueSharedItineraryRepository) Create(ctx context.Context, it domain.SharedItinerary) error {
	_, err := r.data.Write(ctx, it.OwnerID, operations.ItineraryCreate, map[string]any{
		"idItinerario": it.ItineraryID, "idViajero": it.OwnerID, "nombre": it.Name, "linkPublico": it.PublicLink,
	})
	if err != nil {
		return err
	}
	for _, id := range it.ArrangementIDs {
		if err = r.AddArrangement(ctx, it.OwnerID, it.ItineraryID, id); err != nil {
			break
		}
	}
	if err == nil {
		// El directorio va al final: hasta entonces nadie puede resolver el itinerario.
		err = r.directory.Register(ctx, it.ItineraryID, it.OwnerID)
	}
	if err != nil {
		_ = r.deleteShardRows(ctx, it.OwnerID, it.ItineraryID) // compensación
		return err
	}
	return nil
}

// Delete retira el itinerario del directorio (deja de ser accesible) y borra sus filas del shard del dueño.
func (r *QueueSharedItineraryRepository) Delete(ctx context.Context, ownerID, itineraryID string) error {
	if err := r.directory.Remove(ctx, itineraryID, ownerID); err != nil {
		return err
	}
	return r.deleteShardRows(ctx, ownerID, itineraryID)
}

func (r *QueueSharedItineraryRepository) deleteShardRows(ctx context.Context, ownerID, itineraryID string) error {
	key := map[string]any{"idItinerario": itineraryID, "idViajero": ownerID}
	for _, resource := range []string{
		operations.AccessRequestsClear, operations.AccessPermissionsClear,
		operations.ItineraryArrangementsClear, operations.ItineraryDelete,
	} {
		if _, err := r.data.Write(ctx, ownerID, resource, key); err != nil {
			return err
		}
	}
	return nil
}

// AddArrangement agrega la referencia a un arreglo propio (idempotente).
func (r *QueueSharedItineraryRepository) AddArrangement(ctx context.Context, ownerID, itineraryID, arrangementID string) error {
	_, err := r.data.Write(ctx, ownerID, operations.ItineraryArrangementAdd, map[string]any{
		"idItinerario": itineraryID, "idViajero": ownerID, "idArreglo": arrangementID,
	})
	return err
}

// RemoveArrangement quita la referencia a un arreglo.
func (r *QueueSharedItineraryRepository) RemoveArrangement(ctx context.Context, ownerID, itineraryID, arrangementID string) error {
	_, err := r.data.Write(ctx, ownerID, operations.ItineraryArrangementRemove, map[string]any{
		"idItinerario": itineraryID, "idViajero": ownerID, "idArreglo": arrangementID,
	})
	return err
}

// SavePermission inserta o actualiza el permiso (revocar = estado REVOCADO).
func (r *QueueSharedItineraryRepository) SavePermission(ctx context.Context, ownerID, itineraryID string, p domain.AccessPermission) error {
	state := permissionRevoked
	if p.Active {
		state = permissionActive
	}
	_, err := r.data.Write(ctx, ownerID, operations.AccessPermissionSave, map[string]any{
		"idItinerario": itineraryID, "idViajero": ownerID, "idViajeroAutorizado": p.AuthorizedTravelerID, "estado": state,
	})
	return err
}

// SaveRequest crea la solicitud si no existe o actualiza su estado.
func (r *QueueSharedItineraryRepository) SaveRequest(ctx context.Context, ownerID, itineraryID string, req domain.AccessRequest) error {
	res, err := r.data.Write(ctx, ownerID, operations.AccessRequestUpdateStatus, map[string]any{
		"estado": string(req.Status), "idSolicitud": req.RequestID, "idItinerario": itineraryID, "idViajero": ownerID,
	})
	if err != nil {
		return err
	}
	if res.RowsAffected > 0 {
		return nil
	}
	_, err = r.data.Write(ctx, ownerID, operations.AccessRequestCreate, map[string]any{
		"idSolicitud": req.RequestID, "idItinerario": itineraryID, "idViajero": ownerID,
		"idSolicitante": req.RequesterTravelerID, "estado": string(req.Status),
		"fechaSolicitud": req.RequestedAt.UTC().Format(time.RFC3339Nano),
	})
	return err
}

// FindItineraryIDByRequest busca la solicitud dentro del shard del dueño.
func (r *QueueSharedItineraryRepository) FindItineraryIDByRequest(ctx context.Context, ownerID, requestID string) (string, error) {
	res, err := r.data.Read(ctx, ownerID, operations.AccessRequestFindItinerary, map[string]any{"idSolicitud": requestID, "idViajero": ownerID})
	if err != nil {
		return "", err
	}
	if len(res.Rows) == 0 {
		return "", application.ErrItineraryNotFound
	}
	return str(res.Rows[0], "id_itinerario"), nil
}

func str(row map[string]any, col string) string {
	s, _ := row[col].(string)
	return s
}
