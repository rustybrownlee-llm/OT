package dashboard_test

import (
	"strings"
	"testing"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/dashboard"
)

func TestHighlightYAML_CommentLine(t *testing.T) {
	out := string(dashboard.HighlightYAML("# This is a comment"))
	if !strings.Contains(out, "yaml-comment") {
		t.Errorf("comment line should have yaml-comment class, got: %s", out)
	}
	if !strings.Contains(out, "This is a comment") {
		t.Errorf("comment text should appear in output")
	}
}

func TestHighlightYAML_KeyValueLine(t *testing.T) {
	out := string(dashboard.HighlightYAML("vendor: TestCorp"))
	if !strings.Contains(out, "yaml-key") {
		t.Errorf("key-value line should have yaml-key class, got: %s", out)
	}
	if !strings.Contains(out, "yaml-value") {
		t.Errorf("key-value line should have yaml-value class, got: %s", out)
	}
	if !strings.Contains(out, "vendor:") {
		t.Errorf("key should appear in output")
	}
	if !strings.Contains(out, "TestCorp") {
		t.Errorf("value should appear in output")
	}
}

func TestHighlightYAML_ListMarker(t *testing.T) {
	out := string(dashboard.HighlightYAML("- some item"))
	if !strings.Contains(out, "yaml-list-marker") {
		t.Errorf("list line should have yaml-list-marker class, got: %s", out)
	}
	if !strings.Contains(out, "some item") {
		t.Errorf("list item text should appear in output")
	}
}

func TestHighlightYAML_InlineComment(t *testing.T) {
	out := string(dashboard.HighlightYAML("key: value # inline comment"))
	if !strings.Contains(out, "yaml-comment") {
		t.Errorf("inline comment should have yaml-comment class, got: %s", out)
	}
	if !strings.Contains(out, "yaml-value") {
		t.Errorf("value before inline comment should have yaml-value class, got: %s", out)
	}
}

func TestHighlightYAML_HTMLEscaping(t *testing.T) {
	out := string(dashboard.HighlightYAML("description: a < b & c > d"))
	if strings.Contains(out, "<b") && !strings.Contains(out, "&lt;b") {
		t.Error("HTML entities should be escaped to prevent XSS")
	}
	// After escaping, the output should contain the escaped form.
	if !strings.Contains(out, "&lt;") {
		t.Errorf("< should be escaped to &lt; in output: %s", out)
	}
}

func TestHighlightYAML_MultilineString(t *testing.T) {
	yaml := "line1: value1\nline2: value2\nline3: value3"
	out := string(dashboard.HighlightYAML(yaml))
	if !strings.Contains(out, "line1") {
		t.Error("multiline: line1 should appear in output")
	}
	if !strings.Contains(out, "line2") {
		t.Error("multiline: line2 should appear in output")
	}
	if !strings.Contains(out, "line3") {
		t.Error("multiline: line3 should appear in output")
	}
}

func TestHighlightYAML_IndentedKey(t *testing.T) {
	out := string(dashboard.HighlightYAML("  nested_key: nested_value"))
	if !strings.Contains(out, "yaml-key") {
		t.Errorf("indented key-value should have yaml-key class, got: %s", out)
	}
}

func TestHighlightYAML_EmptyInput(t *testing.T) {
	out := string(dashboard.HighlightYAML(""))
	if out != "" {
		t.Errorf("empty input should produce empty output, got: %q", out)
	}
}

func TestHighlightYAML_KeyOnly(t *testing.T) {
	// A key with no value (block scalar header)
	out := string(dashboard.HighlightYAML("register_map_variants:"))
	if !strings.Contains(out, "yaml-key") {
		t.Errorf("key-only line should have yaml-key class, got: %s", out)
	}
}
