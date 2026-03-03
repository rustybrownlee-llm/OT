package web

import (
	"fmt"
	"html/template"
	"net/http"
	"os"

	"github.com/rustybrownlee/ot-simulator/admin/internal/configparse"
)

// configPageData is the template data for the Configuration (/config) page.
type configPageData struct {
	Title       string
	ActivePage  string
	Config      *configparse.Config
	RawYAML     template.HTML // HTML-highlighted version (legacy; kept for future use)
	RawYAMLText string        // raw unescaped text for CodeMirror editor
	ConfigPath  string
	LoadError   string
}

// configValidateResult is the template data for the validation result partial.
type configValidateResult struct {
	Valid  bool
	Errors []string
}

// configHandler renders the full Configuration (/config) page.
func (s *Server) configHandler(w http.ResponseWriter, r *http.Request) {
	data := s.buildConfigData()
	s.render(w, "config.html", data)
}

// buildConfigData assembles the data for the Configuration page.
func (s *Server) buildConfigData() configPageData {
	data := configPageData{
		Title:      "Configuration",
		ActivePage: "config",
		ConfigPath: s.globals.ConfigPath,
	}

	raw, err := os.ReadFile(s.globals.ConfigPath)
	if err != nil {
		data.LoadError = fmt.Sprintf("cannot read config file: %v", err)
		return data
	}
	data.RawYAML = highlightYAML(string(raw))
	data.RawYAMLText = string(raw)

	cfg, err := configparse.ParseLenient(s.globals.ConfigPath)
	if err != nil {
		data.LoadError = fmt.Sprintf("cannot parse config file: %v", err)
		return data
	}
	data.Config = cfg
	return data
}

// configValidateHandler handles POST /config/validate.
// Runs strict validation using configparse.Parse() and returns an HTMX partial.
func (s *Server) configValidateHandler(w http.ResponseWriter, r *http.Request) {
	result := s.runConfigValidation()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if result.Valid {
		fmt.Fprintf(w, `<span class="badge bg-success fs-6">Valid</span>`)
		return
	}
	fmt.Fprintf(w, `<div class="alert alert-danger mb-0"><strong>Validation errors:</strong><ul class="mb-0 mt-2">`)
	for _, e := range result.Errors {
		fmt.Fprintf(w, `<li>%s</li>`, template.HTMLEscapeString(e))
	}
	fmt.Fprintf(w, `</ul></div>`)
}

// runConfigValidation runs configparse.Parse() and returns the result.
func (s *Server) runConfigValidation() configValidateResult {
	_, err := configparse.Parse(s.globals.ConfigPath)
	if err == nil {
		return configValidateResult{Valid: true}
	}
	return configValidateResult{Valid: false, Errors: []string{err.Error()}}
}
