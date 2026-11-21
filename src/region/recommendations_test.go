package region_test

import (
	"errors"
	"testing"
	"time"

	"github.com/nicolas-wai/trotamundos/src/config"
	"github.com/nicolas-wai/trotamundos/src/contracts/events"
)

// recommendationItems devuelve los ítems de GET /recomendaciones por id y el cuerpo crudo.
func (e *env) recommendations(t *testing.T, token string) (map[string]map[string]any, []string) {
	t.Helper()
	resp, body := e.do(t, "GET", "/recomendaciones", token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /recomendaciones: %d %v", resp.StatusCode, body)
	}
	items := map[string]map[string]any{}
	var order []string
	for _, raw := range body["recomendaciones"].([]any) {
		item := raw.(map[string]any)
		id := item["idRecomendacion"].(string)
		items[id] = item
		order = append(order, id)
	}
	return items, order
}

func priceOf(item map[string]any) float64 {
	return item["metadatos"].(map[string]any)["precio"].(float64)
}

// Flujos AD 5.1 (2ª mitad) y 5.8: recomendaciones < 3 s y el feedback HITL se usa en la SIGUIENTE (QA17, QA18).
func TestFlow51And58RecommendationsAndHITLFeedback(t *testing.T) {
	e := newEnv(t)
	_, token := e.signUp(t, "ana@example.com")

	// QA17: la generación responde en menos de 3 s y trae los campos del contrato.
	start := time.Now()
	items, _ := e.recommendations(t, token)
	if elapsed := time.Since(start); elapsed >= 3*time.Second {
		t.Fatalf("GET /recomendaciones tardó %s (QA17 < 3 s)", elapsed)
	}
	if len(items) != 12 {
		t.Fatalf("el catálogo tiene 12 ítems y no hay feedback, llegaron %d", len(items))
	}
	var expensive int
	for id, it := range items {
		for _, field := range []string{"idRecomendacion", "categoria", "titulo", "metadatos", "proveedor"} {
			if it[field] == nil || it[field] == "" {
				t.Fatalf("%s sin %s: %v", id, field, it)
			}
		}
		if priceOf(it) > 120 {
			expensive++
		}
	}
	if expensive == 0 {
		t.Fatal("el catálogo debe tener ítems sobre 120 para probar QA18")
	}

	// Lectura = LECTURA; el feedback = ESCRITURA (todo pasa por la cola de prioridades).
	mark := e.spy.mark()
	e.recommendations(t, token)
	for _, op := range e.spy.since(mark) {
		if op.Tipo != events.OperationRead {
			t.Fatalf("generar recomendaciones solo debe leer, encoló %s %s", op.Tipo, op.Recurso)
		}
	}

	// Feedback AJUSTAR presupuestoMax 120 → 200 { registrado: true } y escrituras encoladas.
	mark = e.spy.mark()
	resp, body := e.do(t, "POST", "/recomendaciones/alo-hotel-lisboa-plaza/feedback", token,
		map[string]any{"accion": "AJUSTAR", "parametrosAjustados": map[string]any{"presupuestoMax": 120}})
	if resp.StatusCode != 200 || body["registrado"] != true {
		t.Fatalf("feedback AJUSTAR: %d %v", resp.StatusCode, body)
	}
	var writes int
	for _, op := range e.spy.since(mark) {
		if op.Tipo == events.OperationWrite {
			writes++
		}
	}
	if writes < 2 { // el feedback y las preferencias
		t.Fatalf("el feedback debía encolar escrituras (feedback + preferencias), encoló %d", writes)
	}

	// QA18: la siguiente recomendación excluye lo que supera el presupuesto, sin importar qué réplica atienda.
	for i := 0; i < 4; i++ {
		after, _ := e.recommendations(t, token)
		if len(after) != 12-expensive {
			t.Fatalf("tras AJUSTAR debían quedar %d ítems, hay %d", 12-expensive, len(after))
		}
		for id, it := range after {
			if priceOf(it) > 120 {
				t.Fatalf("%s (%v) supera el presupuesto ajustado", id, priceOf(it))
			}
		}
	}

	// RECHAZAR excluye el ítem; ACEPTAR lo lleva al tope del ranking de la siguiente recomendación.
	if resp, _ = e.do(t, "POST", "/recomendaciones/rut-alfama/feedback", token, map[string]any{"accion": "RECHAZAR"}); resp.StatusCode != 200 {
		t.Fatalf("RECHAZAR: %d", resp.StatusCode)
	}
	if resp, _ = e.do(t, "POST", "/recomendaciones/tra-bus-lisboa-oporto/feedback", token, map[string]any{"accion": "ACEPTAR"}); resp.StatusCode != 200 {
		t.Fatalf("ACEPTAR: %d", resp.StatusCode)
	}
	after, order := e.recommendations(t, token)
	if _, still := after["rut-alfama"]; still {
		t.Fatal("el ítem rechazado no debe volver a recomendarse")
	}
	if order[0] != "tra-bus-lisboa-oporto" {
		t.Fatalf("el ítem aceptado debía liderar el ranking: %v", order)
	}

	// Las preferencias son por viajero: otro viajero sigue viendo todo el catálogo.
	_, other := e.signUp(t, "beto@example.com")
	if all, _ := e.recommendations(t, other); len(all) != 12 {
		t.Fatalf("el feedback de ana no debe afectar a beto: %d ítems", len(all))
	}
}

func TestPUTPreferencesFeedsTheNextRecommendation(t *testing.T) {
	e := newEnv(t)
	_, token := e.signUp(t, "ana@example.com")

	if resp, body := e.do(t, "PUT", "/perfil/preferencias", token, map[string]any{"parametros": map[string]any{"presupuestoMax": 60}}); resp.StatusCode != 200 {
		t.Fatalf("PUT /perfil/preferencias: %d %v", resp.StatusCode, body)
	}
	items, _ := e.recommendations(t, token)
	if len(items) == 0 {
		t.Fatal("debía haber ítems ≤ 60")
	}
	for id, it := range items {
		if priceOf(it) > 60 {
			t.Fatalf("%s (%v) supera el presupuesto del perfil", id, priceOf(it))
		}
	}
	// PUT reemplaza los parámetros: sin presupuesto vuelve todo el catálogo.
	e.do(t, "PUT", "/perfil/preferencias", token, map[string]any{"parametros": map[string]any{}})
	if items, _ = e.recommendations(t, token); len(items) != 12 {
		t.Fatalf("sin presupuesto debían volver los 12 ítems: %d", len(items))
	}
	if resp, _ := e.do(t, "PUT", "/perfil/preferencias", token, map[string]any{}); resp.StatusCode != 422 {
		t.Fatalf("sin parametros: %d", resp.StatusCode)
	}
	if resp, _ := e.do(t, "PUT", "/perfil/preferencias", "", map[string]any{"parametros": map[string]any{}}); resp.StatusCode != 401 {
		t.Fatalf("sin token: %d", resp.StatusCode)
	}
}

func TestFeedbackValidation(t *testing.T) {
	e := newEnv(t)
	_, token := e.signUp(t, "ana@example.com")
	cases := []struct {
		name, path string
		body       map[string]any
		want       int
	}{
		{"acción inválida", "/recomendaciones/rut-alfama/feedback", map[string]any{"accion": "TAL_VEZ"}, 422},
		{"AJUSTAR sin parámetros", "/recomendaciones/rut-alfama/feedback", map[string]any{"accion": "AJUSTAR"}, 422},
		{"recomendación inexistente", "/recomendaciones/no-existe/feedback", map[string]any{"accion": "ACEPTAR"}, 404},
	}
	for _, c := range cases {
		if resp, body := e.do(t, "POST", c.path, token, c.body); resp.StatusCode != c.want {
			t.Errorf("%s: %d %v, se esperaba %d", c.name, resp.StatusCode, body, c.want)
		}
	}
	if resp, _ := e.do(t, "POST", "/recomendaciones/rut-alfama/feedback", "", map[string]any{"accion": "ACEPTAR"}); resp.StatusCode != 401 {
		t.Fatalf("sin token: %d", resp.StatusCode)
	}
}

// ADR23: el panel de recomendaciones es independiente del dashboard. Si el recomendador falla o demora,
// GET /recomendaciones responde error y las reservas siguen visibles.
func TestADR23RecommenderFailureDoesNotAffectDashboard(t *testing.T) {
	e := newEnvWith(t, func(c *config.RegionConfig) { c.RecommendationTimeout = 150 * time.Millisecond })
	_, token := e.signUp(t, "ana@example.com")

	// Falla del catálogo → 503 MODULO_NO_DISPONIBLE; el dashboard sigue 200.
	e.reg.ProviderCatalog.SetFailure(errors.New("catálogo caído"))
	resp, body := e.do(t, "GET", "/recomendaciones", token, nil)
	if resp.StatusCode != 503 || body["codigo"] != "MODULO_NO_DISPONIBLE" {
		t.Fatalf("recomendador fallando: %d %v", resp.StatusCode, body)
	}
	if d := e.dashboardJSON(t, token); len(d.Viajes) == 0 {
		t.Fatal("el dashboard debe seguir mostrando los viajes")
	}
	if resp, _ := e.raw(t, "GET", "/dashboard", token, "text/html", nil); resp.StatusCode != 200 {
		t.Fatalf("dashboard HTML: %d", resp.StatusCode)
	}

	// Demora por encima del límite → corta por timeout sin esperar al catálogo.
	e.reg.ProviderCatalog.SetFailure(nil)
	e.reg.ProviderCatalog.SetDelay(2 * time.Second)
	start := time.Now()
	resp, body = e.do(t, "GET", "/recomendaciones", token, nil)
	if elapsed := time.Since(start); resp.StatusCode != 503 || body["codigo"] != "MODULO_NO_DISPONIBLE" || elapsed > time.Second {
		t.Fatalf("recomendador lento: %d %v tras %s", resp.StatusCode, body, elapsed)
	}
	if e.dashboardJSON(t, token).Viajes == nil {
		t.Fatal("el dashboard debe seguir 200 mientras el panel falla")
	}

	// Al recuperarse el catálogo, el panel vuelve a funcionar sin reiniciar nada.
	e.reg.ProviderCatalog.SetDelay(0)
	if items, _ := e.recommendations(t, token); len(items) != 12 {
		t.Fatalf("recuperación: %d ítems", len(items))
	}
}
