package api

import "net/http"

// Vinculación de correo [ADR-15].

// LinkEmail atiende POST /correos.
func (c *Controllers) LinkEmail(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.EmailLinking.Link(r.Context(), travelerIDFrom(r.Context()), "", "", "")
	respondSkeleton(w, err)
}

// UnlinkEmail atiende DELETE /correos/{idCuenta}.
func (c *Controllers) UnlinkEmail(w http.ResponseWriter, r *http.Request) {
	respondSkeleton(w, c.deps.EmailLinking.Unlink(r.Context(), travelerIDFrom(r.Context()), r.PathValue("idCuenta")))
}
