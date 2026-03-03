package web

import (
	"html/template"
	"testing"

	"github.com/rustybrownlee/ot-simulator/admin/internal/templates"
)

// TestTemplatesParse verifies that all page templates parse without errors
// when combined with the shared base template set and funcmap.
// This catches template syntax errors and missing template definition errors
// at test time rather than at request time.
func TestTemplatesParse(t *testing.T) {
	base := template.Must(template.New("").Funcs(adminFuncMap).ParseFS(
		templates.FS,
		"layout.html",
		"partials/nav.html",
		"partials/health_cards.html",
		"partials/db_stats.html",
		"partials/baseline_table.html",
	))

	pages := []string{
		"dashboard.html",
		"db.html",
		"config.html",
		"design.html",
		"baseline.html",
		"editor.html",
	}
	for _, p := range pages {
		t.Run(p, func(t *testing.T) {
			_, err := template.Must(base.Clone()).ParseFS(templates.FS, p)
			if err != nil {
				t.Errorf("template %s: parse error: %v", p, err)
			}
		})
	}
}

// TestLoadDesignCacheEmpty verifies that loadDesignCache returns a non-nil
// empty cache when the design directory is empty string.
func TestLoadDesignCacheEmpty(t *testing.T) {
	cache := loadDesignCache("")
	if cache == nil {
		t.Fatal("loadDesignCache should return non-nil cache")
	}
	if len(cache.Devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(cache.Devices))
	}
	if len(cache.Networks) != 0 {
		t.Errorf("expected 0 networks, got %d", len(cache.Networks))
	}
}

// TestHighlightYAML verifies that YAML syntax highlighting produces HTML with
// the expected CSS class spans for comments, keys, values, and list markers.
func TestHighlightYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"comment", "# This is a comment", `class="yaml-comment"`},
		{"key-value", "key: value", `class="yaml-key"`},
		{"list-marker", "- item", `class="yaml-list-marker"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := string(highlightYAML(tt.input))
			if len(out) == 0 {
				t.Error("highlightYAML returned empty string")
			}
			// Verify the span class appears in the output.
			found := false
			for i := 0; i+len(tt.contains) <= len(out); i++ {
				if out[i:i+len(tt.contains)] == tt.contains {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("highlightYAML(%q): expected %q in output %q", tt.input, tt.contains, out)
			}
		})
	}
}
