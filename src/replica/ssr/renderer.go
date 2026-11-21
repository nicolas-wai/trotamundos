// Package ssr es el componente "Renderizador SSR" de la réplica (AD §4.4).
package ssr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"time"

	"github.com/nicolas-wai/trotamundos/src/client/web"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/application"
	"github.com/nicolas-wai/trotamundos/src/replica/travelarrangements/domain"
)

// SSRRenderer [ADR-17][ADR-20] Genera el HTML del dashboard con payload reducido usando html/template.
// El HTML es un fragmento que el runtime del cliente inyecta en el shell web; html/template escapa
// todo dato del viajero.
type SSRRenderer struct {
	tpl *template.Template
}

// NewSSRRenderer carga las plantillas embebidas de src/client/web/templates. Falla con panic si no compilan
// (error de programación detectado al arrancar, no en tiempo de request).
func NewSSRRenderer() *SSRRenderer {
	return &SSRRenderer{tpl: template.Must(template.ParseFS(web.Templates, "templates/*.html"))}
}

type dashboardView struct {
	Trips   []tripView
	Notices []noticeView
}

type noticeView struct{ Provider, Message string }

type tripView struct {
	ID, Name, Destination, Dates string
	Arrangements                 []arrangementView
}

// arrangementView es el arreglo con payload reducido: nunca incluye el detalle. [ADR-17]
type arrangementView struct {
	ID, Type, TypeLabel, Title string
	StartISO, StartText        string
	EndISO                     string // para precargar la edición; solo fecha, no es detalle
	Status, StatusLabel        string
	Origin, OriginLabel        string
}

type detailView struct {
	arrangementView
	Sender    string
	Updated   string
	UpdatedAt string
	Fields    []fieldView
}

type fieldView struct{ Label, Value string }

// RenderDashboard devuelve el HTML del dashboard con solo datos críticos.
func (r *SSRRenderer) RenderDashboard(_ context.Context, dashboard application.Dashboard) ([]byte, error) {
	view := dashboardView{}
	for _, t := range dashboard.Trips {
		tv := tripView{ID: t.TripID, Name: t.Name, Destination: t.Destination, Dates: dateRange(t.StartDate, t.EndDate)}
		for _, a := range t.Arrangements {
			tv.Arrangements = append(tv.Arrangements, summaryView(a))
		}
		view.Trips = append(view.Trips, tv)
	}
	for _, n := range dashboard.ProviderNotices {
		view.Notices = append(view.Notices, noticeView{Provider: n.Provider, Message: n.Message})
	}
	return r.execute("dashboard", view)
}

// RenderArrangementDetail devuelve el fragmento HTML con el detalle completo de un arreglo (carga bajo demanda).
func (r *SSRRenderer) RenderArrangementDetail(_ context.Context, a domain.TravelArrangement) ([]byte, error) {
	view := detailView{
		arrangementView: summaryView(a), Sender: string(a.Sender),
		Updated: a.UpdatedAt.UTC().Format("02/01/2006 15:04"), UpdatedAt: a.UpdatedAt.UTC().Format(time.RFC3339),
		Fields: detailFields(a.Detail),
	}
	return r.execute("arrangement_detail", view)
}

func (r *SSRRenderer) execute(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.tpl.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("renderizando %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

func summaryView(a domain.TravelArrangement) arrangementView {
	return arrangementView{
		ID: a.ArrangementID, Type: string(a.Type), TypeLabel: label(typeLabels, string(a.Type)), Title: a.Title,
		StartISO: a.StartAt.UTC().Format(time.RFC3339), StartText: a.StartAt.UTC().Format("02/01/2006 15:04"),
		EndISO: endISO(a.EndAt),
		Status: string(a.Status), StatusLabel: label(statusLabels, string(a.Status)),
		Origin: string(a.Origin), OriginLabel: label(originLabels, string(a.Origin)),
	}
}

func endISO(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func dateRange(start, end time.Time) string {
	const layout = "02/01/2006"
	switch {
	case start.IsZero():
		return ""
	case end.IsZero():
		return start.Format(layout)
	}
	return start.Format(layout) + " – " + end.Format(layout)
}

// detailFields aplana el detalle tipado a pares etiqueta/valor, ordenados por etiqueta.
func detailFields(d domain.ArrangementDetail) []fieldView {
	if d == nil {
		return nil
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	fields := make([]fieldView, 0, len(m))
	for k, v := range m {
		fields = append(fields, fieldView{Label: label(fieldLabels, k), Value: fmt.Sprint(v)})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Label < fields[j].Label })
	return fields
}

func label(table map[string]string, key string) string {
	if l, ok := table[key]; ok {
		return l
	}
	return key
}

var typeLabels = map[string]string{
	"VUELO": "Vuelo", "ALOJAMIENTO": "Alojamiento", "AUTO": "Auto", "TREN": "Tren", "BUS": "Bus",
}

var statusLabels = map[string]string{
	"CONFIRMADO": "Confirmado", "CANCELADO": "Cancelado", "MODIFICADO": "Modificado",
}

var originLabels = map[string]string{
	"MANUAL": "Manual", "EMAIL": "Correo", "AGENCIA": "Agencia", "VIAJES": "Sistema de viajes",
}

var fieldLabels = map[string]string{
	"aerolinea": "Aerolínea", "numeroVuelo": "N.º de vuelo", "origenIATA": "Origen (IATA)", "destinoIATA": "Destino (IATA)",
	"hotel": "Hotel", "direccion": "Dirección", "estrellas": "Estrellas",
	"empresa": "Empresa", "retiro": "Retiro", "devolucion": "Devolución",
	"operador": "Operador", "origen": "Origen", "destino": "Destino",
}
