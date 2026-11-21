// Package httpdto contiene los requests/responses JSON de AD §6.2 (nombres JSON exactos del contrato).
package httpdto

// --- Autenticación [ADR-14, ADR-16] ---

// LoginRequest es el body de POST /login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse es la respuesta 200 de POST /login.
type LoginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
	TokenType    string `json:"tokenType"`
}

// RefreshRequest es el body de POST /token/refresh y de POST /logout.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// RefreshResponse es la respuesta 200 de POST /token/refresh.
type RefreshResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
	TokenType   string `json:"tokenType"`
}

// LogoutRequest es el body de POST /logout.
type LogoutRequest = RefreshRequest

// RegisterRequest es el body de POST /registro [QA-05].
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterResponse es la respuesta 201 de POST /registro.
type RegisterResponse struct {
	IDViajero string `json:"idViajero"`
}
