package web

// handlers_editor_api.go implements REST API endpoints for the YAML editor:
//   GET  /api/design/file     -- read raw YAML content for a design file
//   POST /api/design/validate -- validate YAML content against JSON Schema
//   POST /api/design/save     -- validate then write YAML content to disk
//   POST /api/config/validate -- validate monitor.yaml content via configparse
//   POST /api/config/save     -- validate and write monitor.yaml to disk
//
// PROTOTYPE-DEBT: [td-admin-121] Design file save does not refresh the design cache.
// PROTOTYPE-DEBT: [td-admin-122] Config save does not hot-reload the running monitor.
// PROTOTYPE-DEBT: [td-admin-124] Validation response transformation is per-request (not cached).

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/rustybrownlee/ot-simulator/admin/internal/configparse"
	"github.com/rustybrownlee/ot-simulator/admin/internal/schema"
)

// validationError is the JSON representation of a single schema validation error.
type validationError struct {
	Line    int    `json:"line"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

// validateResponse is the JSON body returned by /api/design/validate.
type validateResponse struct {
	Valid  bool              `json:"valid"`
	Errors []validationError `json:"errors,omitempty"`
}

// saveRequest is the JSON body accepted by /api/design/save.
type saveRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Force   bool   `json:"force"`
}

// saveResponse is the JSON body returned by /api/design/save.
type saveResponse struct {
	Saved  bool              `json:"saved"`
	Errors []validationError `json:"errors,omitempty"`
}

// contentRequest is the JSON body accepted by /api/design/validate.
type contentRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// designFileHandler handles GET /api/design/file?path=<relpath>.
// Returns 200 text/plain with YAML content, 403 on path traversal, 404 on missing file.
func (s *Server) designFileHandler(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	absPath, ok := ResolveSafePath(s.globals.DesignDir, relPath)
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		slog.Error("design file read error", "path", absPath, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data) // nolint:errcheck
}

// designValidateHandler handles POST /api/design/validate.
// Accepts JSON {path, content}, runs schema validation, returns JSON result.
func (s *Server) designValidateHandler(w http.ResponseWriter, r *http.Request) {
	var req contentRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	writeJSON(w, s.runDesignValidation(req.Path, req.Content))
}

// runDesignValidation validates YAML content against the schema inferred from relPath.
func (s *Server) runDesignValidation(relPath, content string) validateResponse {
	if s.schemas == nil {
		return validateResponse{
			Valid:  false,
			Errors: []validationError{{Line: 0, Path: "", Message: "schema set not loaded; cannot validate"}},
		}
	}

	// Build an absolute path for schema type inference without reading from disk.
	absPath := filepath.Join(s.globals.DesignDir, relPath)
	result, err := schema.ValidateContent(content, absPath, s.globals.DesignDir, s.schemas)
	if err != nil {
		return validateResponse{
			Valid:  false,
			Errors: []validationError{{Line: 0, Path: "", Message: err.Error()}},
		}
	}

	if result.OK() {
		return validateResponse{Valid: true}
	}
	return validateResponse{Valid: false, Errors: convertErrors(result.Errors)}
}

// designSaveHandler handles POST /api/design/save.
// Validates first (unless force=true), then writes atomically to disk.
func (s *Server) designSaveHandler(w http.ResponseWriter, r *http.Request) {
	var req saveRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	absPath, ok := ResolveSafePath(s.globals.DesignDir, req.Path)
	if !ok {
		writeJSON(w, saveResponse{
			Saved:  false,
			Errors: []validationError{{Line: 0, Path: "", Message: "path is outside the design directory"}},
		})
		return
	}

	if !req.Force {
		vr := s.runDesignValidation(req.Path, req.Content)
		if !vr.Valid {
			writeJSON(w, saveResponse{Saved: false, Errors: vr.Errors})
			return
		}
	}

	if err := atomicWrite(absPath, []byte(req.Content)); err != nil {
		slog.Error("design file save error", "path", absPath, "error", err)
		writeJSON(w, saveResponse{
			Saved:  false,
			Errors: []validationError{{Line: 0, Path: "", Message: fmt.Sprintf("write error: %v", err)}},
		})
		return
	}

	writeJSON(w, saveResponse{Saved: true})
}

// configValidateAPIHandler handles POST /api/config/validate.
// Validates the submitted YAML string using configparse.Parse and returns JSON.
func (s *Server) configValidateAPIHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	writeJSON(w, s.runConfigContentValidation(req.Content))
}

// runConfigContentValidation validates YAML string content via configparse.
func (s *Server) runConfigContentValidation(content string) validateResponse {
	if err := configparse.ParseContent(content); err != nil {
		return validateResponse{
			Valid:  false,
			Errors: []validationError{{Line: 0, Path: "", Message: err.Error()}},
		}
	}
	return validateResponse{Valid: true}
}

// configSaveHandler handles POST /api/config/save.
// Validates using configparse then atomically writes to globals.ConfigPath.
func (s *Server) configSaveHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	vr := s.runConfigContentValidation(req.Content)
	if !vr.Valid {
		writeJSON(w, saveResponse{Saved: false, Errors: vr.Errors})
		return
	}

	if err := atomicWrite(s.globals.ConfigPath, []byte(req.Content)); err != nil {
		slog.Error("config save error", "path", s.globals.ConfigPath, "error", err)
		writeJSON(w, saveResponse{
			Saved:  false,
			Errors: []validationError{{Line: 0, Path: "", Message: fmt.Sprintf("write error: %v", err)}},
		})
		return
	}

	writeJSON(w, saveResponse{Saved: true})
}

// atomicWrite writes data to path using a temp-file-then-rename pattern to
// prevent partial writes. The temp file is created in the same directory as
// path so that the rename is atomic on POSIX file systems.
func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".otsim-save-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // cleanup on error; no-op after successful rename

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	return os.Rename(tmpName, path)
}

// convertErrors maps schema.ValidationError slice to the API response format.
func convertErrors(errs []*schema.ValidationError) []validationError {
	out := make([]validationError, len(errs))
	for i, e := range errs {
		out[i] = validationError{Line: e.Line, Path: e.Path, Message: e.Message}
	}
	return out
}

// decodeJSONBody decodes the JSON request body into v.
// Writes 400 Bad Request and returns false on decode failure.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return false
	}
	return true
}

// writeJSON serialises v as JSON and writes it to w with application/json Content-Type.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("JSON encode error", "error", err)
	}
}
