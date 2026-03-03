// Package web provides the HTTP server for the admin web dashboard.
// The server follows the monitoring dashboard pattern: chi router, go:embed templates,
// render/renderPartial helpers, and buildXxxData functions per page section.
//
// PROTOTYPE-DEBT: [td-admin-112] Health card port probing runs on every HTMX refresh.
// See SOW-037.0 technical debt table for full context.
// PROTOTYPE-DEBT: [td-admin-114] Bootstrap 5 and HTMX CDN versions not centrally enforced.
package web

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/rustybrownlee/ot-simulator/admin/internal/schema"
	"github.com/rustybrownlee/ot-simulator/admin/internal/templates"
)

// adminFuncMap provides arithmetic helpers used by admin templates.
// Go html/template has no built-in arithmetic; divFloat64 converts int64 bytes
// to a float64 for MB display in the DB stats partial.
var adminFuncMap = template.FuncMap{
	"divFloat64": func(numerator int64, denominator float64) float64 {
		return float64(numerator) / denominator
	},
}

// Globals holds resolved global configuration passed to web handlers.
// This mirrors cli.Globals to break the import cycle between the cli and web packages.
// The fields must stay in sync with cli.Globals.
type Globals struct {
	DesignDir  string
	ConfigPath string
	DBPath     string
	APIAddr    string
	PlantPorts string
}

// Server holds the configured HTTP server for the admin dashboard.
// Dependencies are provided at construction time via the Globals struct.
type Server struct {
	addr    string
	globals Globals
	router  chi.Router
	base    *template.Template
	design  *DesignCache
	schemas *schema.SchemaSet // compiled JSON Schemas; nil if design/schemas/ is missing
}

// New creates a Server listening on addr (e.g., ":8095").
// The Globals struct provides all paths and addresses needed by handlers.
// The design library and JSON Schemas are loaded once at startup.
func New(addr string, globals Globals) *Server {
	s := &Server{addr: addr, globals: globals}
	s.design = loadDesignCache(globals.DesignDir)
	schemasDir := filepath.Join(globals.DesignDir, "schemas")
	ss, err := schema.Load(schemasDir)
	if err != nil {
		slog.Warn("JSON schemas not loaded; validation endpoints will return error",
			"schemas_dir", schemasDir, "error", err)
	} else {
		s.schemas = ss
		slog.Info("JSON schemas loaded", "schemas_dir", schemasDir)
	}
	s.base = template.Must(template.New("").Funcs(adminFuncMap).ParseFS(
		templates.FS,
		"layout.html",
		"partials/nav.html",
		"partials/health_cards.html",
		"partials/db_stats.html",
		"partials/baseline_table.html",
	))
	s.router = s.buildRouter()
	return s
}

// Start starts the HTTP server and blocks until it exits.
// Returns an error if the server cannot bind to the configured address.
func (s *Server) Start() error {
	fmt.Printf("Admin dashboard listening on http://localhost%s\n", s.addr)
	return http.ListenAndServe(s.addr, s.router)
}

// buildRouter registers all admin dashboard routes on a new chi router.
func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/", s.dashboardHandler)
	r.Get("/partials/health-cards", s.healthCardsHandler)

	r.Get("/db", s.dbHandler)
	r.Get("/partials/db-stats", s.dbStatsHandler)
	r.Get("/db/export", s.dbExportHandler)
	r.Post("/db/prune", s.dbPruneHandler)

	r.Get("/config", s.configHandler)
	r.Post("/config/validate", s.configValidateHandler)
	r.Post("/api/config/validate", s.configValidateAPIHandler)
	r.Post("/api/config/save", s.configSaveHandler)

	r.Get("/design", s.designHandler)
	r.Get("/design/edit", s.editorHandler)

	r.Get("/api/design/file", s.designFileHandler)
	r.Post("/api/design/validate", s.designValidateHandler)
	r.Post("/api/design/save", s.designSaveHandler)

	r.Get("/baseline", s.baselineHandler)
	r.Get("/partials/baseline-table", s.baselineTableHandler)
	r.Post("/baseline/reset", s.baselineResetHandler)
	r.Post("/baseline/reset-all", s.baselineResetAllHandler)

	return r
}

// render clones the base template set, parses the named page template, and
// executes the "layout" template. Cloning prevents named-template conflicts
// between pages that each define their own "content" block.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	cloned := template.Must(s.base.Clone())
	t, err := cloned.ParseFS(templates.FS, name)
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

// renderPartial clones the base set and executes a named partial template.
// Used for HTMX partial responses. The named template must be in the base set.
func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	t, err := s.base.Clone()
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
