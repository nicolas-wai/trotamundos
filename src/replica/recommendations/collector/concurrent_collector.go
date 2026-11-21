// Package collector es el componente "Recolector concurrente" de Recomendaciones (AD §4.4).
package collector

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
	"github.com/nicolas-wai/trotamundos/src/ports"
	auth "github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
	"github.com/nicolas-wai/trotamundos/src/replica/recommendations/domain"
	arrangements "github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// PreferencesReader lee las preferencias del perfil (lo satisface el repositorio de preferencias de Autenticación).
type PreferencesReader interface {
	Get(ctx context.Context, travelerID string) (auth.TravelerPreferences, error)
}

// ConcurrentCollector [ADR-24] Consulta en paralelo el historial (viajes, arreglos y feedback, una lectura por
// tabla) y el perfil; cada lectura pasa por la cola de prioridades. Las unifica al terminar todas. [QA-17]
type ConcurrentCollector struct {
	data        ports.DataGateway
	preferences PreferencesReader
}

var _ ports.HistoryPort = (*ConcurrentCollector)(nil)

// NewConcurrentCollector crea el recolector.
func NewConcurrentCollector(data ports.DataGateway, preferences PreferencesReader) *ConcurrentCollector {
	return &ConcurrentCollector{data: data, preferences: preferences}
}

// Get lanza las cuatro lecturas en paralelo y devuelve el historial unificado. Si alguna falla, la falla se
// propaga (no se recomienda con datos parciales) y se cancelan las demás.
func (c *ConcurrentCollector) Get(ctx context.Context, travelerID string) (ports.TravelerHistory, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		history  ports.TravelerHistory
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)
	// Cada lectura escribe en un campo distinto de history: no hay carrera entre gorutinas.
	run := func(read func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := read(ctx); err != nil {
				errOnce.Do(func() { firstErr = err; cancel() })
			}
		}()
	}
	key := map[string]any{"idViajero": travelerID}
	run(func(ctx context.Context) error {
		res, err := c.data.Read(ctx, travelerID, operations.TripList, key)
		for _, row := range res.Rows {
			history.Trips = append(history.Trips, arrangements.Trip{
				TripID: str(row, "id_viaje"), TravelerID: travelerID, Name: str(row, "nombre"),
				Destination: str(row, "destino"), Status: arrangements.TripStatus(str(row, "estado")),
			})
		}
		return err
	})
	run(func(ctx context.Context) error {
		res, err := c.data.Read(ctx, travelerID, operations.ArrangementListSummaries, key)
		for _, row := range res.Rows {
			history.Arrangements = append(history.Arrangements, arrangements.TravelArrangement{
				ArrangementID: str(row, "id_arreglo"), TripID: str(row, "id_viaje"), TravelerID: travelerID,
				Type: arrangements.ArrangementType(str(row, "tipo")), Title: str(row, "titulo"),
				Origin: arrangements.Origin(str(row, "origen")), Status: arrangements.Status(str(row, "estado")),
			})
		}
		return err
	})
	run(func(ctx context.Context) error {
		res, err := c.data.Read(ctx, travelerID, operations.RecommendationFeedbackList, key)
		for _, row := range res.Rows {
			params := map[string]any{}
			_ = json.Unmarshal([]byte(str(row, "parametros_ajustados")), &params)
			at, _ := time.Parse(time.RFC3339Nano, str(row, "fecha"))
			history.Feedback = append(history.Feedback, domain.RecommendationFeedback{
				RecommendationID: str(row, "id_recomendacion"), Action: domain.FeedbackAction(str(row, "accion")),
				AdjustedParams: params, At: at,
			})
		}
		return err
	})
	run(func(ctx context.Context) error {
		prefs, err := c.preferences.Get(ctx, travelerID)
		history.Preferences = prefs
		return err
	})

	wg.Wait()
	if firstErr != nil {
		return ports.TravelerHistory{}, firstErr
	}
	return history, nil
}

func str(row map[string]any, col string) string {
	s, _ := row[col].(string)
	return s
}
