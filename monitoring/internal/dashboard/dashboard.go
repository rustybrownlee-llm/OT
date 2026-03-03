package dashboard

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/templates"
)

// Dashboard serves the web UI on port 8090. It reads live monitoring data
// from the APIClient (port 8091 loopback) and design-layer YAML from the
// DesignLibrary. All pages are server-rendered HTML; no JavaScript framework.
//
// PROTOTYPE-DEBT: [td-dashboard-029] CDN dependencies require internet access.
// Training labs without internet cannot load Bootstrap CSS or HTMX JS.
// PROTOTYPE-DEBT: [td-dashboard-030] Asset-to-device-atom mapping uses heuristic.
// PROTOTYPE-DEBT: [td-dashboard-031] No pagination on asset or alert pages.
// PROTOTYPE-DEBT: [td-dashboard-032] Design library loaded once; no refresh mechanism.
// PROTOTYPE-DEBT: [td-dashboard-033] Register scaling convention is hardcoded.
// PROTOTYPE-DEBT: [td-dashboard-034] No dark mode support.
type Dashboard struct {
	api    *APIClient
	lib    *DesignLibrary
	router chi.Router
	// base holds the layout, nav, badge, and shared partial templates.
	// Page-specific templates are added per-render to avoid name conflicts.
	base *template.Template
}

// svgFuncMap provides arithmetic and formatting functions used by SVG templates.
// Go html/template has no built-in arithmetic; these are required for computing
// SVG coordinates (left, top, right, bottom) from center + dimensions.
var svgFuncMap = template.FuncMap{
	"add":     func(a, b float64) float64 { return a + b },
	"sub":     func(a, b float64) float64 { return a - b },
	"mul":     func(a, b float64) float64 { return a * b },
	"div":     func(a, b float64) float64 { return a / b },
	"f1":      func(v float64) string { return fmt.Sprintf("%.1f", v) },
	"f0":      func(v float64) string { return fmt.Sprintf("%.0f", v) },
	"half":    func(v float64) float64 { return v / 2.0 },
	"float64": func(v int) float64 { return float64(v) },
}

// NewDashboard creates a Dashboard, parsing shared templates eagerly.
// If template parsing fails, the program exits (template.Must pattern).
func NewDashboard(api *APIClient, lib *DesignLibrary) *Dashboard {
	d := &Dashboard{
		api: api,
		lib: lib,
	}
	// Parse only the shared templates (layout, nav, badge, shared partials).
	// Page-specific templates are cloned and extended per render.
	// svgFuncMap is registered for all templates including process view SVG helpers.
	d.base = template.Must(template.New("").Funcs(svgFuncMap).ParseFS(templates.FS,
		"layout.html",
		"partials/nav.html",
		"partials/badge.html",
		"yaml_raw.html",
		"overview_cards.html",
		"assets_table.html",
		"asset_registers.html",
		"alerts_table.html",
		"partials/topology_view.html",
		"partials/process_symbols.html",
		"partials/process_instruments.html",
		"partials/process_connections.html",
		"partials/process_network_context.html",
	))
	d.router = d.buildRouter()
	return d
}

// Routes returns the chi router for all dashboard routes.
func (d *Dashboard) Routes() http.Handler {
	return d.router
}

// buildRouter registers all dashboard routes.
func (d *Dashboard) buildRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	// Observed section.
	r.Get("/", d.overviewHandler)
	r.Get("/partials/overview-cards", d.overviewCardsHandler)
	r.Get("/assets", d.assetsHandler)
	r.Get("/partials/assets-table", d.assetsTableHandler)
	r.Get("/assets/{id}", d.assetDetailHandler)
	r.Get("/partials/asset-registers/{id}", d.assetRegistersHandler)
	r.Get("/alerts", d.alertsHandler)
	r.Get("/partials/alerts-table", d.alertsTableHandler)
	r.Post("/alerts/{id}/acknowledge", d.alertAckHandler)

	// Topology section.
	r.Get("/topology", d.topologyHandler)
	r.Get("/topology/{env-id}", d.topologyEnvHandler)
	r.Get("/partials/topology-view/{env-id}", d.topologyPartialHandler)

	// Reference section.
	r.Get("/design/devices", d.designDevicesHandler)
	r.Get("/design/devices/{id}", d.designDeviceDetailHandler)
	r.Get("/design/networks", d.designNetworksHandler)
	r.Get("/design/networks/{id}", d.designNetworkDetailHandler)
	r.Get("/design/environments", d.designEnvsHandler)
	r.Get("/design/environments/{id}", d.designEnvDetailHandler)

	return r
}

// render parses the named page template into a clone of the base template set
// and executes the "layout" template. Cloning avoids named-template conflicts
// between pages that each define their own "content" block.
// Clone() preserves the FuncMap registered in NewDashboard.
func (d *Dashboard) render(w http.ResponseWriter, name string, data any) {
	t, err := template.Must(d.base.Clone()).ParseFS(templates.FS, name)
	if err != nil {
		slog.Error("template parse error", "template", name, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("template render error", "template", name, "error", err)
	}
}

// renderPartial executes a named template block without the layout wrapper.
// Used for HTMX partial responses. The named template must be in the base set.
// Clones the base template to avoid marking it as executed, which would prevent
// future Clone() calls in render().
func (d *Dashboard) renderPartial(w http.ResponseWriter, name string, data any) {
	t, err := d.base.Clone()
	if err != nil {
		slog.Error("partial clone error", "template", name, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("partial render error", "template", name, "error", err)
	}
}

// templateFS returns the embedded filesystem for testing.
func templateFS() fs.FS {
	return templates.FS
}
