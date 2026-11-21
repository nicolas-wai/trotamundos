package api

import (
	"io/fs"
	"net/http"

	"github.com/nicolas-wai/trotamundos/src/client/web"
	"github.com/nicolas-wai/trotamundos/src/replica/tokenvalidator"
)

// NewRouter registra todas las rutas de AD §6.2 (patrones Go 1.22+, rutas en español según el contrato).
// [Prototipo] HTTP local simulando HTTPS/TLS [ADR-12].
// Las rutas de autenticación y el shell web estático son públicos; el resto exige token válido
// (tokenvalidator [ADR-16]) y, hasta que cada módulo se implemente, responde 501 NO_IMPLEMENTADO.
func NewRouter(c *Controllers, v *tokenvalidator.SignedTokenValidator) http.Handler {
	mux := http.NewServeMux()
	registerWebShell(mux)
	protected := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, v.Middleware(h))
	}

	// Autenticación (ADR-14, ADR-16)
	mux.HandleFunc("POST /login", c.Login)
	mux.HandleFunc("POST /token/refresh", c.RefreshToken)
	mux.HandleFunc("POST /logout", c.Logout)
	mux.HandleFunc("POST /registro", c.Register)

	// Dashboard y arreglos (ADR-17, ADR-18, ADR-20)
	protected("GET /dashboard", c.Dashboard)
	protected("GET /arreglos/{idArreglo}", c.GetArrangement)
	protected("POST /arreglos", c.CreateArrangement)
	protected("PUT /arreglos/{idArreglo}", c.UpdateArrangement)
	protected("DELETE /arreglos/{idArreglo}", c.DeleteArrangement)

	// Recomendaciones (ADR-23, ADR-25)
	protected("GET /recomendaciones", c.Recommendations)
	protected("POST /recomendaciones/{idRecomendacion}/feedback", c.RecommendationFeedback)
	protected("PUT /perfil/preferencias", c.UpdatePreferences)

	// Vinculación de correo (ADR-15)
	protected("POST /correos", c.LinkEmail)
	protected("DELETE /correos/{idCuenta}", c.UnlinkEmail)

	// Itinerarios compartidos (ADR-13)
	protected("GET /itinerarios/{idItinerario}", c.GetItinerary)
	protected("POST /itinerarios", c.CreateItinerary)
	protected("DELETE /itinerarios/{id}", c.DeleteItinerary)
	protected("POST /itinerarios/{id}/arreglos", c.AddItineraryArrangement)
	protected("DELETE /itinerarios/{id}/arreglos/{idArreglo}", c.RemoveItineraryArrangement)
	protected("POST /itinerarios/{id}/solicitudes", c.RequestItineraryAccess)
	protected("GET /itinerarios/{id}/solicitudes", c.ListItineraryRequests)
	protected("POST /itinerarios/{id}/solicitudes/{idSolicitud}/decision", c.DecideItineraryRequest)
	protected("DELETE /itinerarios/{id}/accesos/{idViajero}", c.RevokeItineraryAccess)

	return mux
}

// registerWebShell sirve el shell del cliente web (AD §4.1), embebido en el binario: GET / y /static/.
// Es contenido estático sin datos de viajero; el runtime obtiene el dashboard con Bearer token. [ADR-20]
func registerWebShell(mux *http.ServeMux) {
	static, err := fs.Sub(web.Static, "static")
	if err != nil {
		panic(err) // error de programación: el directorio está embebido
	}
	mux.Handle("GET /static/", noCache(http.StripPrefix("/static/", http.FileServerFS(static))))
	mux.Handle("GET /{$}", noCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index, err := fs.ReadFile(static, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})))
}

// noCache evita servir JS/CSS obsoletos tras reconstruir el binario del prototipo.
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}
