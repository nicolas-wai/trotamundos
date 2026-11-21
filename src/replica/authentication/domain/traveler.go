// Package domain (authentication) modela el agregado "Cuenta del viajero" (AD §2.2).
package domain

import "time"

// Traveler es la raíz del agregado Cuenta del viajero.
// Invariantes: email único; credenciales siempre cifradas con bcrypt [ADR-14];
// nunca se guarda usuario/contraseña del correo, solo tokens OAuth [ADR-15].
// Clave de partición: TravelerID [ADR-11].
type Traveler struct {
	TravelerID        string
	Email             string
	EncryptedPassword string // hash bcrypt, nunca texto plano
	CreatedAt         time.Time
}

// RefreshToken es el token de renovación persistido y revocable. [ADR-16]
// Pertenece al agregado Traveler.
type RefreshToken struct {
	TokenID    string
	TravelerID string
	ExpiresAt  time.Time
	Revoked    bool
}

// TravelerPreferences son los parámetros que alimentan las recomendaciones (AD §2.3).
// Distintas de las preferencias visuales, que viven en LocalStorage [ADR-19].
type TravelerPreferences struct {
	TravelerID               string
	RecommendationParams     map[string]any
	AccommodationPreferences map[string]any
	TransportPreferences     map[string]any
	RoutePreferences         map[string]any
}

// EmailAccountStatus es el estado del vínculo OAuth de una cuenta de correo.
type EmailAccountStatus string

// Estados del vínculo OAuth.
const (
	EmailAccountLinked  EmailAccountStatus = "VINCULADA"
	EmailAccountRevoked EmailAccountStatus = "REVOCADA"
)

// LinkedEmailAccount es un vínculo OAuth 2.0 revocable con un proveedor de correo. [ADR-15]
// Los tokens se guardan cifrados en reposo.
type LinkedEmailAccount struct {
	AccountID             string
	TravelerID            string
	Provider              string
	EncryptedAccessToken  []byte
	EncryptedRefreshToken []byte
	Status                EmailAccountStatus
}

// TrustedSender es un remitente de la whitelist de correo. ◇ US03 (sin lógica en el prototipo).
type TrustedSender struct {
	TravelerID    string
	SenderAddress string
}
