package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"
)

// englishPrinter is a package-level printer for localizing schema error messages.
// LocalizedString panics when passed a nil printer; this avoids that.
var englishPrinter = message.NewPrinter(language.English)

// SchemaType identifies which JSON Schema applies to a given YAML file.
type SchemaType int

const (
	SchemaTypeUnknown SchemaType = iota
	SchemaTypeDeviceAtom
	SchemaTypeNetworkAtom
	SchemaTypeEnvironment
	SchemaTypeProcess
)

// ValidationError represents a single schema validation failure with file location.
type ValidationError struct {
	File    string // Source file path
	Line    int    // YAML source line number (0 = unknown)
	Path    string // JSON Pointer path to the failing field
	Message string // Human-readable error description
}

// String formats the error as "file:line: path: message".
func (e *ValidationError) String() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s: %s", e.File, e.Line, e.Path, e.Message)
	}
	return fmt.Sprintf("%s: %s: %s", e.File, e.Path, e.Message)
}

// ValidateResult holds the outcome of validating a single YAML file.
type ValidateResult struct {
	File   string
	Errors []*ValidationError
}

// OK returns true when no validation errors were found.
func (r *ValidateResult) OK() bool {
	return len(r.Errors) == 0
}

// InferSchemaType determines which JSON Schema applies to a YAML file based on
// its location relative to the design directory. Returns SchemaTypeUnknown when
// the file is not in a recognized design subdirectory.
func InferSchemaType(filePath, designDir string) (SchemaType, error) {
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return SchemaTypeUnknown, fmt.Errorf("resolving path: %w", err)
	}
	absDesign, err := filepath.Abs(designDir)
	if err != nil {
		return SchemaTypeUnknown, fmt.Errorf("resolving design dir: %w", err)
	}

	rel, err := filepath.Rel(absDesign, absFile)
	if err != nil || strings.HasPrefix(rel, "..") {
		return SchemaTypeUnknown, fmt.Errorf(
			"cannot determine schema type for %s: file is not in design/devices/, design/networks/, or design/environments/",
			filePath,
		)
	}

	parts := strings.SplitN(filepath.ToSlash(rel), "/", 3)
	if len(parts) < 2 {
		return SchemaTypeUnknown, fmt.Errorf(
			"cannot determine schema type for %s: file is not in design/devices/, design/networks/, or design/environments/",
			filePath,
		)
	}

	switch parts[0] {
	case "devices":
		return SchemaTypeDeviceAtom, nil
	case "networks":
		return SchemaTypeNetworkAtom, nil
	case "environments":
		if len(parts) >= 2 {
			base := filepath.Base(absFile)
			switch base {
			case "environment.yaml":
				return SchemaTypeEnvironment, nil
			case "process.yaml":
				return SchemaTypeProcess, nil
			}
		}
	}

	return SchemaTypeUnknown, fmt.Errorf(
		"cannot determine schema type for %s: file is not in design/devices/, design/networks/, or design/environments/",
		filePath,
	)
}

// ValidateContent validates a YAML string against the schema indicated by filePath.
// The filePath is used only for schema type inference (via InferSchemaType) and is
// included in returned ValidationError.File fields; the YAML is not read from disk.
// This is the web API variant of ValidateFile -- the editor submits content directly.
func ValidateContent(content, filePath, designDir string, schemas *SchemaSet) (*ValidateResult, error) {
	schemaType, err := InferSchemaType(filePath, designDir)
	if err != nil {
		return nil, err
	}

	var s *jsonschema.Schema
	switch schemaType {
	case SchemaTypeDeviceAtom:
		s = schemas.DeviceAtom
	case SchemaTypeNetworkAtom:
		s = schemas.NetworkAtom
	case SchemaTypeEnvironment:
		s = schemas.Environment
	case SchemaTypeProcess:
		s = schemas.Process
	default:
		return nil, fmt.Errorf("no schema for file %s", filePath)
	}

	return validateContentAgainstSchema(filePath, []byte(content), s)
}

// validateContentAgainstSchema parses YAML bytes and validates against a schema.
// It is the content-string counterpart to validateAgainstSchema.
func validateContentAgainstSchema(filePath string, data []byte, s *jsonschema.Schema) (*ValidateResult, error) {
	root, iface, err := parseYAMLFile(data)
	if err != nil {
		return &ValidateResult{
			File: filePath,
			Errors: []*ValidationError{{
				File:    filePath,
				Line:    1,
				Path:    "/",
				Message: fmt.Sprintf("YAML parse error: %v", err),
			}},
		}, nil
	}

	result := &ValidateResult{File: filePath}
	valErr, ok := s.Validate(iface).(*jsonschema.ValidationError)
	if ok && valErr != nil {
		leaves := collectLeafErrors(valErr)
		for _, leaf := range leaves {
			result.Errors = append(result.Errors, formatLeafError(filePath, root, leaf))
		}
	}
	return result, nil
}

// ValidateFile validates a single YAML file against the appropriate JSON Schema.
// The file's schema type is inferred from its path relative to the design directory.
func ValidateFile(filePath, designDir string, schemas *SchemaSet) (*ValidateResult, error) {
	schemaType, err := InferSchemaType(filePath, designDir)
	if err != nil {
		return nil, err
	}

	var schema *jsonschema.Schema
	switch schemaType {
	case SchemaTypeDeviceAtom:
		schema = schemas.DeviceAtom
	case SchemaTypeNetworkAtom:
		schema = schemas.NetworkAtom
	case SchemaTypeEnvironment:
		schema = schemas.Environment
	case SchemaTypeProcess:
		schema = schemas.Process
	default:
		return nil, fmt.Errorf("no schema for file %s", filePath)
	}

	return validateAgainstSchema(filePath, schema)
}

// ValidateFileWithSchema validates a YAML file against an explicitly provided schema,
// bypassing schema type inference. Used when the caller knows the schema to apply.
func ValidateFileWithSchema(filePath string, schema *jsonschema.Schema) (*ValidateResult, error) {
	return validateAgainstSchema(filePath, schema)
}

// validateAgainstSchema parses a YAML file and validates it against the given schema.
func validateAgainstSchema(filePath string, schema *jsonschema.Schema) (*ValidateResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filePath, err)
	}

	root, iface, err := parseYAMLFile(data)
	if err != nil {
		return &ValidateResult{
			File: filePath,
			Errors: []*ValidationError{{
				File:    filePath,
				Line:    1,
				Path:    "/",
				Message: fmt.Sprintf("YAML parse error: %v", err),
			}},
		}, nil
	}

	result := &ValidateResult{File: filePath}
	valErr, ok := schema.Validate(iface).(*jsonschema.ValidationError)
	if ok && valErr != nil {
		leaves := collectLeafErrors(valErr)
		for _, leaf := range leaves {
			result.Errors = append(result.Errors, formatLeafError(filePath, root, leaf))
		}
	}
	return result, nil
}

// parseYAMLFile parses YAML data into both a yaml.Node tree (for line numbers)
// and a Go interface{} tree (for schema validation).
func parseYAMLFile(data []byte) (*yaml.Node, interface{}, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, nil, err
	}
	iface := nodeToInterface(&root)
	return &root, iface, nil
}

// formatLeafError converts a jsonschema leaf ValidationError into our ValidationError.
func formatLeafError(filePath string, root *yaml.Node, leaf *jsonschema.ValidationError) *ValidationError {
	pointer := "/" + strings.Join(leaf.InstanceLocation, "/")
	line := ResolveLineNumber(root, pointer)
	return &ValidationError{
		File:    filePath,
		Line:    line,
		Path:    pointer,
		Message: leaf.ErrorKind.LocalizedString(englishPrinter),
	}
}

// collectLeafErrors recursively collects leaf ValidationErrors from an error tree.
// Leaf errors are those with no Causes -- they carry the specific validation message.
func collectLeafErrors(err *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(err.Causes) == 0 {
		return []*jsonschema.ValidationError{err}
	}
	var leaves []*jsonschema.ValidationError
	for _, cause := range err.Causes {
		leaves = append(leaves, collectLeafErrors(cause)...)
	}
	return leaves
}
