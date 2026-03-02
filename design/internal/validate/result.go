// Package validate implements design layer YAML validation for the ot-design CLI.
// It checks device atoms, network atoms, and environment definitions against
// the v0.1 schema rules defined in milestone-b0.2.md.
package validate

import (
	"fmt"
	"strings"
)

// Severity indicates how serious a validation finding is.
type Severity int

const (
	// SeverityError means the file will not work correctly and must be fixed.
	SeverityError Severity = iota
	// SeverityWarning means the file may cause confusion and should be reviewed.
	SeverityWarning
)

// ValidationError represents a single validation finding.
type ValidationError struct {
	File     string   // File path where the error was found
	Field    string   // YAML field path (e.g., "register_map_variants.water-intake.holding[2].scale_min")
	Message  string   // Human-readable error description
	Severity Severity // Error or Warning
	RuleID   string   // e.g., "DEV-012" for traceability
}

// String formats a single validation error for human-readable output.
// Format: "<file>: <field>: <message> [<rule-id>]"
func (e ValidationError) String() string {
	prefix := "ERROR"
	if e.Severity == SeverityWarning {
		prefix = "WARN"
	}
	if e.Field != "" {
		return fmt.Sprintf("[%s] %s: %s: %s [%s]", prefix, e.File, e.Field, e.Message, e.RuleID)
	}
	return fmt.Sprintf("[%s] %s: %s [%s]", prefix, e.File, e.Message, e.RuleID)
}

// ValidationResult collects all findings from a validation run.
type ValidationResult struct {
	Errors []ValidationError
}

// Add appends a validation finding to the result.
func (r *ValidationResult) Add(err ValidationError) {
	r.Errors = append(r.Errors, err)
}

// HasErrors returns true if any findings have SeverityError.
func (r *ValidationResult) HasErrors() bool {
	for _, e := range r.Errors {
		if e.Severity == SeverityError {
			return true
		}
	}
	return false
}

// String formats the full validation result for human-readable output.
// If no errors, prints "Validation passed: <target>".
// Otherwise prints all findings followed by a summary line.
func (r *ValidationResult) String(target string) string {
	if len(r.Errors) == 0 {
		return fmt.Sprintf("Validation passed: %s", target)
	}

	var sb strings.Builder
	errCount := 0
	warnCount := 0
	for _, e := range r.Errors {
		sb.WriteString(e.String())
		sb.WriteByte('\n')
		if e.Severity == SeverityError {
			errCount++
		} else {
			warnCount++
		}
	}
	sb.WriteString(fmt.Sprintf("Validation complete: %d error(s), %d warning(s)", errCount, warnCount))
	return sb.String()
}
