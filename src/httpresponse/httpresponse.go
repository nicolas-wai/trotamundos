// Package httpresponse escribe respuestas HTTP/JSON siguiendo las convenciones de errores de AD §6.1.
// Es el adaptador HTTP de contracts/apierror: aquí vive la dependencia de net/http para que el contrato de
// errores (y, con él, el núcleo domain/application) siga libre de tecnología. [ADR-02]
package httpresponse

import (
	"encoding/json"
	"net/http"

	"github.com/nicolas-wai/trotamundos/src/contracts/apierror"
)

// WriteError escribe el cuerpo de error con el código HTTP indicado.
func WriteError(w http.ResponseWriter, status int, codigo, mensaje string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apierror.ErrorResponse{Codigo: codigo, Mensaje: mensaje})
}

// WriteNotImplemented responde 501 NO_IMPLEMENTADO.
func WriteNotImplemented(w http.ResponseWriter) {
	WriteError(w, http.StatusNotImplemented, apierror.NoImplementado, "Funcionalidad no implementada en esta fase del prototipo")
}

// WriteJSON escribe una respuesta JSON con el código HTTP indicado.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
