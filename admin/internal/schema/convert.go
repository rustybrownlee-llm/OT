package schema

import (
	"strconv"

	"gopkg.in/yaml.v3"
)

// nodeToInterface converts a yaml.Node tree into a Go interface{} tree that
// is compatible with JSON types (string, int64, float64, bool, nil,
// map[string]interface{}, []interface{}).
// This conversion is required because santhosh-tekuri/jsonschema validates
// Go interface{} values, not raw YAML or JSON bytes.
func nodeToInterface(node *yaml.Node) interface{} {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) > 0 {
			return nodeToInterface(node.Content[0])
		}
		return nil
	case yaml.MappingNode:
		return mappingToInterface(node)
	case yaml.SequenceNode:
		return sequenceToInterface(node)
	case yaml.AliasNode:
		if node.Alias != nil {
			return nodeToInterface(node.Alias)
		}
		return nil
	case yaml.ScalarNode:
		return scalarToInterface(node)
	}
	return nil
}

// mappingToInterface converts a YAML mapping node to map[string]interface{}.
// Key nodes are always converted to their string value regardless of tag.
func mappingToInterface(node *yaml.Node) map[string]interface{} {
	m := make(map[string]interface{}, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := nodeToInterface(node.Content[i+1])
		m[key] = val
	}
	return m
}

// sequenceToInterface converts a YAML sequence node to []interface{}.
func sequenceToInterface(node *yaml.Node) []interface{} {
	items := make([]interface{}, len(node.Content))
	for i, child := range node.Content {
		items[i] = nodeToInterface(child)
	}
	return items
}

// scalarToInterface converts a YAML scalar node to the appropriate Go type
// based on the node's tag. Correct type mapping is critical: schema validation
// will reject a boolean "true" stored as a string, or an integer stored as float.
func scalarToInterface(node *yaml.Node) interface{} {
	if node.Tag == "!!null" || node.Value == "null" || node.Value == "~" {
		return nil
	}

	switch node.Tag {
	case "!!bool":
		return parseBool(node.Value)
	case "!!int":
		return parseInt(node.Value)
	case "!!float":
		return parseFloat(node.Value)
	case "!!str":
		return node.Value
	}

	// Fallback: attempt type inference by value for untagged scalars.
	return inferScalarType(node.Value)
}

// parseBool converts YAML boolean strings to Go bool.
func parseBool(v string) interface{} {
	switch v {
	case "true", "True", "TRUE", "yes", "Yes", "YES", "on", "On", "ON":
		return true
	case "false", "False", "FALSE", "no", "No", "NO", "off", "Off", "OFF":
		return false
	}
	return v
}

// parseInt converts a YAML integer string to int64.
func parseInt(v string) interface{} {
	n, err := strconv.ParseInt(v, 0, 64)
	if err != nil {
		return v
	}
	return n
}

// parseFloat converts a YAML float string to float64.
func parseFloat(v string) interface{} {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v
	}
	return f
}

// inferScalarType attempts numeric or boolean parsing for untagged scalars.
// Falls back to string when no numeric or boolean interpretation applies.
func inferScalarType(v string) interface{} {
	switch v {
	case "true", "True", "TRUE":
		return true
	case "false", "False", "FALSE":
		return false
	case "null", "~", "":
		return nil
	}
	if n, err := strconv.ParseInt(v, 0, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return v
}
