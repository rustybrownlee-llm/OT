package validate

import (
	"strings"
	"testing"
)

func TestValidationError_String(t *testing.T) {
	tests := []struct {
		name     string
		err      ValidationError
		wantSubs []string
	}{
		{
			name: "error with field",
			err: ValidationError{
				File:     "design/devices/foo.yaml",
				Field:    "register_map_variants.v1.holding[2].scale_min",
				Message:  "scale_min (100) must be less than scale_max (50)",
				Severity: SeverityError,
				RuleID:   "DEV-012",
			},
			wantSubs: []string{"ERROR", "foo.yaml", "scale_min", "DEV-012"},
		},
		{
			name: "warning without field",
			err: ValidationError{
				File:     "design/devices/foo.yaml",
				Field:    "",
				Message:  "response_delay_ms: value 1 is outside expected range",
				Severity: SeverityWarning,
				RuleID:   "DEV-017",
			},
			wantSubs: []string{"WARN", "foo.yaml", "DEV-017"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.String()
			for _, sub := range tc.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("String() = %q, want substring %q", got, sub)
				}
			}
		})
	}
}

func TestValidationResult_HasErrors(t *testing.T) {
	tests := []struct {
		name   string
		errors []ValidationError
		want   bool
	}{
		{
			name:   "empty result",
			errors: nil,
			want:   false,
		},
		{
			name: "only warnings",
			errors: []ValidationError{
				{Severity: SeverityWarning, RuleID: "DEV-017"},
			},
			want: false,
		},
		{
			name: "has error",
			errors: []ValidationError{
				{Severity: SeverityWarning, RuleID: "DEV-017"},
				{Severity: SeverityError, RuleID: "DEV-002"},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &ValidationResult{Errors: tc.errors}
			if got := r.HasErrors(); got != tc.want {
				t.Errorf("HasErrors() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidationResult_Add(t *testing.T) {
	r := &ValidationResult{}
	if len(r.Errors) != 0 {
		t.Fatal("expected empty result initially")
	}
	r.Add(ValidationError{RuleID: "DEV-001", Severity: SeverityError})
	r.Add(ValidationError{RuleID: "DEV-017", Severity: SeverityWarning})
	if len(r.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(r.Errors))
	}
}

func TestValidationResult_String(t *testing.T) {
	t.Run("passed", func(t *testing.T) {
		r := &ValidationResult{}
		s := r.String("design/devices/foo.yaml")
		if !strings.Contains(s, "Validation passed") {
			t.Errorf("expected pass message, got %q", s)
		}
		if !strings.Contains(s, "foo.yaml") {
			t.Errorf("expected target in pass message, got %q", s)
		}
	})

	t.Run("with errors and warnings", func(t *testing.T) {
		r := &ValidationResult{}
		r.Add(ValidationError{File: "f.yaml", Message: "bad", Severity: SeverityError, RuleID: "DEV-002"})
		r.Add(ValidationError{File: "f.yaml", Message: "warn", Severity: SeverityWarning, RuleID: "DEV-017"})
		s := r.String("f.yaml")
		if !strings.Contains(s, "1 error(s)") {
			t.Errorf("expected 1 error in summary, got %q", s)
		}
		if !strings.Contains(s, "1 warning(s)") {
			t.Errorf("expected 1 warning in summary, got %q", s)
		}
	})
}
