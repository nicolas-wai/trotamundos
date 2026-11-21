package api

import "net/http"

// Itinerarios compartidos [ADR-13].

// GetItinerary atiende GET /itinerarios/{idItinerario}.
func (c *Controllers) GetItinerary(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Permissions.GetItinerary(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idItinerario"))
	respondSkeleton(w, err)
}

// CreateItinerary atiende POST /itinerarios.
func (c *Controllers) CreateItinerary(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Permissions.CreateItinerary(r.Context(), travelerIDFrom(r.Context()), "", nil)
	respondSkeleton(w, err)
}

// DeleteItinerary atiende DELETE /itinerarios/{id}.
func (c *Controllers) DeleteItinerary(w http.ResponseWriter, r *http.Request) {
	respondSkeleton(w, c.deps.Permissions.DeleteItinerary(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id")))
}

// AddItineraryArrangement atiende POST /itinerarios/{id}/arreglos.
func (c *Controllers) AddItineraryArrangement(w http.ResponseWriter, r *http.Request) {
	respondSkeleton(w, c.deps.Permissions.AddArrangement(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id"), ""))
}

// RemoveItineraryArrangement atiende DELETE /itinerarios/{id}/arreglos/{idArreglo}.
func (c *Controllers) RemoveItineraryArrangement(w http.ResponseWriter, r *http.Request) {
	respondSkeleton(w, c.deps.Permissions.RemoveArrangement(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id"), r.PathValue("idArreglo")))
}

// RequestItineraryAccess atiende POST /itinerarios/{id}/solicitudes.
func (c *Controllers) RequestItineraryAccess(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Permissions.RequestAccess(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id"))
	respondSkeleton(w, err)
}

// ListItineraryRequests atiende GET /itinerarios/{id}/solicitudes (solo el dueño).
func (c *Controllers) ListItineraryRequests(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Permissions.ListRequests(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id"))
	respondSkeleton(w, err)
}

// DecideItineraryRequest atiende POST /itinerarios/{id}/solicitudes/{idSolicitud}/decision (solo el dueño).
func (c *Controllers) DecideItineraryRequest(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Permissions.DecideRequest(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id"), r.PathValue("idSolicitud"), "")
	respondSkeleton(w, err)
}

// RevokeItineraryAccess atiende DELETE /itinerarios/{id}/accesos/{idViajero}.
func (c *Controllers) RevokeItineraryAccess(w http.ResponseWriter, r *http.Request) {
	respondSkeleton(w, c.deps.Permissions.Revoke(r.Context(), travelerIDFrom(r.Context()), r.PathValue("id"), r.PathValue("idViajero")))
}
