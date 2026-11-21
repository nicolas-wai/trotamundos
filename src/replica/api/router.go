package api

import "net/http"

// NewRouter registra todas las rutas de AD §6.2 (patrones Go 1.22+, rutas en español según el contrato).
// [Prototipo] HTTP local simulando HTTPS/TLS.
// Fase 1: todas responden 501 NO_IMPLEMENTADO. El tokenvalidator se engancha en la Fase 2 [ADR-16].
func NewRouter(c *Controllers) http.Handler {
	mux := http.NewServeMux()

	// Autenticación (ADR-14, ADR-16)
	mux.HandleFunc("POST /login", c.Login)
	mux.HandleFunc("POST /token/refresh", c.RefreshToken)
	mux.HandleFunc("POST /logout", c.Logout)
	mux.HandleFunc("POST /registro", c.Register)

	// Dashboard y arreglos (ADR-17, ADR-18, ADR-20)
	mux.HandleFunc("GET /dashboard", c.Dashboard)
	mux.HandleFunc("GET /arreglos/{idArreglo}", c.GetArrangement)
	mux.HandleFunc("POST /arreglos", c.CreateArrangement)
	mux.HandleFunc("PUT /arreglos/{idArreglo}", c.UpdateArrangement)
	mux.HandleFunc("DELETE /arreglos/{idArreglo}", c.DeleteArrangement)

	// Recomendaciones (ADR-23, ADR-25)
	mux.HandleFunc("GET /recomendaciones", c.Recommendations)
	mux.HandleFunc("POST /recomendaciones/{idRecomendacion}/feedback", c.RecommendationFeedback)
	mux.HandleFunc("PUT /perfil/preferencias", c.UpdatePreferences)

	// Vinculación de correo (ADR-15)
	mux.HandleFunc("POST /correos", c.LinkEmail)
	mux.HandleFunc("DELETE /correos/{idCuenta}", c.UnlinkEmail)

	// Itinerarios compartidos (ADR-13)
	mux.HandleFunc("GET /itinerarios/{idItinerario}", c.GetItinerary)
	mux.HandleFunc("POST /itinerarios", c.CreateItinerary)
	mux.HandleFunc("DELETE /itinerarios/{id}", c.DeleteItinerary)
	mux.HandleFunc("POST /itinerarios/{id}/arreglos", c.AddItineraryArrangement)
	mux.HandleFunc("DELETE /itinerarios/{id}/arreglos/{idArreglo}", c.RemoveItineraryArrangement)
	mux.HandleFunc("POST /itinerarios/{id}/solicitudes", c.RequestItineraryAccess)
	mux.HandleFunc("GET /itinerarios/{id}/solicitudes", c.ListItineraryRequests)
	mux.HandleFunc("POST /itinerarios/{id}/solicitudes/{idSolicitud}/decision", c.DecideItineraryRequest)
	mux.HandleFunc("DELETE /itinerarios/{id}/accesos/{idViajero}", c.RevokeItineraryAccess)

	return mux
}
