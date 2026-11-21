package application

import "errors"

// Errores del módulo Arreglos de viaje que la API traduce a códigos HTTP (AD §6.1).
var (
	// ErrValidation → 422 VALIDACION_FALLIDA.
	ErrValidation = errors.New("validación fallida")
	// ErrNotFound → 404 NO_ENCONTRADO (también cuando el arreglo pertenece a otro viajero: no se revela su existencia).
	ErrNotFound = errors.New("no encontrado")
)
