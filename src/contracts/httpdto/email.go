package httpdto

// --- Vinculación de correo [ADR-15] ---

// LinkEmailRequest es el body de POST /correos.
type LinkEmailRequest struct {
	Proveedor         string `json:"proveedor"`
	AuthorizationCode string `json:"authorizationCode"`
	RedirectURI       string `json:"redirectUri"`
}

// LinkEmailResponse es la respuesta 201 de POST /correos.
type LinkEmailResponse struct {
	IDCuenta  string `json:"idCuenta"`
	Proveedor string `json:"proveedor"`
	Estado    string `json:"estado"`
}
