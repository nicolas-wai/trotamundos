// Package web es el contenedor "Cliente web" (AD §4.1), embebido en el binario.
package web

import "embed"

// Templates contiene las plantillas html/template del dashboard. [ADR-20] SSR con payload reducido.
//
//go:embed templates
var Templates embed.FS

// Static contiene el JS vanilla mínimo del cliente: runtime, estado optimista [ADR-18],
// preferencias visuales en LocalStorage [ADR-19] y panel de recomendaciones asíncrono [ADR-23].
//
//go:embed static
var Static embed.FS
