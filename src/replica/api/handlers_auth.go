package api

import "net/http"

// Autenticación [ADR-14][ADR-16].

// Login atiende POST /login.
func (c *Controllers) Login(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Authentication.Login(r.Context(), "", "")
	respondSkeleton(w, err)
}

// RefreshToken atiende POST /token/refresh.
func (c *Controllers) RefreshToken(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Authentication.Refresh(r.Context(), "")
	respondSkeleton(w, err)
}

// Logout atiende POST /logout.
func (c *Controllers) Logout(w http.ResponseWriter, r *http.Request) {
	respondSkeleton(w, c.deps.Authentication.Logout(r.Context(), ""))
}

// Register atiende POST /registro [QA-05].
func (c *Controllers) Register(w http.ResponseWriter, r *http.Request) {
	_, err := c.deps.Authentication.Register(r.Context(), "", "")
	respondSkeleton(w, err)
}
