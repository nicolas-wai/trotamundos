package collector

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/datanode/operations"
	"github.com/nicolas-wai/trotamundos/src/ports"
	auth "github.com/nicolas-wai/trotamundos/src/replica/authentication/domain"
)

// barrier libera a todos solo cuando llegaron n lectores: si el recolector leyera en serie, nadie llegaría
// a la vez y la lectura vencería por timeout. Es el espía que prueba el paralelismo [ADR-24].
type barrier struct {
	mu      sync.Mutex
	arrived int
	n       int
	open    chan struct{}
}

func newBarrier(n int) *barrier { return &barrier{n: n, open: make(chan struct{})} }

func (b *barrier) wait(ctx context.Context) error {
	b.mu.Lock()
	b.arrived++
	if b.arrived == b.n {
		close(b.open)
	}
	b.mu.Unlock()
	select {
	case <-b.open:
		return nil
	case <-time.After(2 * time.Second):
		return errors.New("las lecturas no se ejecutaron en paralelo")
	case <-ctx.Done():
		return ctx.Err()
	}
}

type spyGateway struct {
	barrier *barrier
	failOn  string
	mu      sync.Mutex
	reads   []string
}

func (g *spyGateway) Read(ctx context.Context, travelerID, resource string, _ map[string]any) (ports.DataResult, error) {
	g.mu.Lock()
	g.reads = append(g.reads, resource)
	g.mu.Unlock()
	if err := g.barrier.wait(ctx); err != nil {
		return ports.DataResult{}, err
	}
	if resource == g.failOn {
		return ports.DataResult{}, errors.New("falla de lectura de " + resource)
	}
	switch resource {
	case operations.TripList:
		return ports.DataResult{Rows: []map[string]any{{"id_viaje": "t1", "destino": "Lisboa", "estado": "ACTIVO"}}}, nil
	case operations.ArrangementListSummaries:
		return ports.DataResult{Rows: []map[string]any{{"id_arreglo": "a1", "tipo": "VUELO", "titulo": "Vuelo a Lisboa"}}}, nil
	case operations.RecommendationFeedbackList:
		return ports.DataResult{Rows: []map[string]any{{"id_recomendacion": "r1", "accion": "RECHAZAR", "parametros_ajustados": "{}", "fecha": "2026-01-01T00:00:00Z"}}}, nil
	}
	return ports.DataResult{}, nil
}

func (g *spyGateway) Write(context.Context, string, string, map[string]any) (ports.DataResult, error) {
	return ports.DataResult{}, errors.New("el recolector no escribe")
}

type spyPreferences struct{ barrier *barrier }

func (p spyPreferences) Get(ctx context.Context, travelerID string) (auth.TravelerPreferences, error) {
	if err := p.barrier.wait(ctx); err != nil {
		return auth.TravelerPreferences{}, err
	}
	return auth.TravelerPreferences{TravelerID: travelerID, RecommendationParams: map[string]any{"presupuestoMax": 120.0}}, nil
}

func TestCollectorReadsInParallelAndUnifies(t *testing.T) {
	b := newBarrier(4) // viajes, arreglos, feedback y perfil
	c := NewConcurrentCollector(&spyGateway{barrier: b}, spyPreferences{barrier: b})

	h, err := c.Get(context.Background(), "v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Trips) != 1 || h.Trips[0].Destination != "Lisboa" {
		t.Fatalf("viajes: %+v", h.Trips)
	}
	if len(h.Arrangements) != 1 || h.Arrangements[0].Title != "Vuelo a Lisboa" {
		t.Fatalf("arreglos: %+v", h.Arrangements)
	}
	if len(h.Feedback) != 1 || h.Feedback[0].RecommendationID != "r1" || h.Feedback[0].Action != "RECHAZAR" {
		t.Fatalf("feedback: %+v", h.Feedback)
	}
	if h.Preferences.RecommendationParams["presupuestoMax"] != 120.0 {
		t.Fatalf("perfil: %+v", h.Preferences)
	}
}

// Una falla parcial se propaga: no se recomienda con un historial incompleto.
func TestCollectorPropagatesPartialFailure(t *testing.T) {
	b := newBarrier(4)
	c := NewConcurrentCollector(&spyGateway{barrier: b, failOn: operations.RecommendationFeedbackList}, spyPreferences{barrier: b})
	_, err := c.Get(context.Background(), "v1")
	if err == nil || err.Error() != "falla de lectura de "+operations.RecommendationFeedbackList {
		t.Fatalf("se esperaba la falla de la lectura de feedback, hubo %v", err)
	}
}

func TestCollectorHonorsContextCancellation(t *testing.T) {
	b := newBarrier(5) // nunca se completa: el contexto cancelado debe cortar la espera
	c := NewConcurrentCollector(&spyGateway{barrier: b}, spyPreferences{barrier: b})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.Get(ctx, "v1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("se esperaba DeadlineExceeded, hubo %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("la cancelación no cortó las lecturas a tiempo")
	}
}
