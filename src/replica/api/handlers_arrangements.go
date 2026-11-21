package api

import (
	"net/http"

	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
)

// Dashboard y arreglos [ADR-17][ADR-18][ADR-20].

// Dashboard atiende GET /dashboard (HTML en web, JSON con Accept: application/json).
func (c *Controllers) Dashboard(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Arrangements.GetDashboard(r.Context(), travelerIDFrom(r.Context()))
	respondSkeleton(w, err)
}

// GetArrangement atiende GET /arreglos/{idArreglo}.
func (c *Controllers) GetArrangement(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Arrangements.GetArrangementDetail(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idArreglo"))
	respondSkeleton(w, err)
}

// CreateArrangement atiende POST /arreglos (escritura por la cola de prioridades).
func (c *Controllers) CreateArrangement(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Arrangements.CreateArrangement(r.Context(), travelerIDFrom(r.Context()), application.ArrangementInput{})
	respondSkeleton(w, err)
}

// UpdateArrangement atiende PUT /arreglos/{idArreglo}.
func (c *Controllers) UpdateArrangement(w http.ResponseWriter, r *http.Request) {
	err := c.deps.Arrangements.UpdateArrangement(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idArreglo"), application.ArrangementInput{})
	respondSkeleton(w, err)
}

// DeleteArrangement atiende DELETE /arreglos/{idArreglo}.
func (c *Controllers) DeleteArrangement(w http.ResponseWriter, r *http.Request) {
	err := c.deps.Arrangements.DeleteArrangement(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idArreglo"))
	respondSkeleton(w, err)
}
