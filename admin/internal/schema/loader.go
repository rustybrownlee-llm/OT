// Package schema loads and compiles JSON Schemas for the design layer and provides
// YAML validation, line-number mapping, and cross-reference checking.
package schema

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// SchemaSet holds compiled JSON Schemas, one per design layer type.
// Schemas are compiled once per invocation and reused across all file validations.
type SchemaSet struct {
	DeviceAtom  *jsonschema.Schema
	NetworkAtom *jsonschema.Schema
	Environment *jsonschema.Schema
	Process     *jsonschema.Schema
}

// schemaFile maps a schema logical name to its filename within the schemas directory.
type schemaFile struct {
	name string
	file string
}

var schemaFiles = []schemaFile{
	{"device-atom", "device-atom.schema.json"},
	{"network-atom", "network-atom.schema.json"},
	{"environment", "environment.schema.json"},
	{"process", "process.schema.json"},
}

// Load reads and compiles all four JSON Schema files from schemasDir.
// Returns an error if any schema file is missing or fails to compile.
// All schemas are compiled eagerly to fail fast on missing or invalid files.
func Load(schemasDir string) (*SchemaSet, error) {
	compiler := jsonschema.NewCompiler()

	urls := make(map[string]string)
	for _, sf := range schemaFiles {
		path := filepath.Join(schemasDir, sf.file)
		url, err := addSchemaFile(compiler, path)
		if err != nil {
			return nil, fmt.Errorf("loading schema %s: %w", sf.file, err)
		}
		urls[sf.name] = url
	}

	set := &SchemaSet{}
	var err error

	set.DeviceAtom, err = compiler.Compile(urls["device-atom"])
	if err != nil {
		return nil, fmt.Errorf("compiling device-atom schema: %w", err)
	}

	set.NetworkAtom, err = compiler.Compile(urls["network-atom"])
	if err != nil {
		return nil, fmt.Errorf("compiling network-atom schema: %w", err)
	}

	set.Environment, err = compiler.Compile(urls["environment"])
	if err != nil {
		return nil, fmt.Errorf("compiling environment schema: %w", err)
	}

	set.Process, err = compiler.Compile(urls["process"])
	if err != nil {
		return nil, fmt.Errorf("compiling process schema: %w", err)
	}

	return set, nil
}

// addSchemaFile reads a JSON schema file, registers it with the compiler, and
// returns the URL used to reference it for compilation.
func addSchemaFile(compiler *jsonschema.Compiler, path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path %s: %w", path, err)
	}

	f, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", absPath, err)
	}
	defer f.Close()

	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		return "", fmt.Errorf("parsing JSON in %s: %w", absPath, err)
	}

	url := "file://" + absPath
	if err := compiler.AddResource(url, doc); err != nil {
		return "", fmt.Errorf("registering schema %s: %w", absPath, err)
	}

	return url, nil
}
