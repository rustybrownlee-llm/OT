package schema

import (
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ResolveLineNumber maps a JSON Pointer path (e.g., "/placements/2/modbus_port")
// back to a YAML source line number by walking the yaml.Node tree.
// Returns 0 if the path cannot be resolved to a specific node.
func ResolveLineNumber(root *yaml.Node, jsonPointer string) int {
	if root == nil {
		return 0
	}
	// yaml.Node wraps the document root in a DocumentNode.
	doc := root
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		doc = doc.Content[0]
	}

	segments := parseJSONPointer(jsonPointer)
	if len(segments) == 0 {
		return root.Line
	}

	node, found := walkYAMLNode(doc, segments)
	if !found || node == nil {
		return 0
	}
	return node.Line
}

// parseJSONPointer splits an RFC 6901 JSON Pointer into path segments.
// "/foo/bar/0" -> ["foo", "bar", "0"]. An empty string returns nil.
func parseJSONPointer(pointer string) []string {
	if pointer == "" || pointer == "/" {
		return nil
	}
	// Strip leading slash.
	if strings.HasPrefix(pointer, "/") {
		pointer = pointer[1:]
	}
	parts := strings.Split(pointer, "/")
	// Unescape RFC 6901 escape sequences: ~1 -> /, ~0 -> ~.
	for i, p := range parts {
		p = strings.ReplaceAll(p, "~1", "/")
		p = strings.ReplaceAll(p, "~0", "~")
		parts[i] = p
	}
	return parts
}

// walkYAMLNode traverses the yaml.Node tree following path segments.
// Returns the terminal node and true if found, nil and false otherwise.
func walkYAMLNode(node *yaml.Node, segments []string) (*yaml.Node, bool) {
	if len(segments) == 0 {
		return node, true
	}

	seg := segments[0]
	rest := segments[1:]

	switch node.Kind {
	case yaml.MappingNode:
		return walkMappingNode(node, seg, rest)
	case yaml.SequenceNode:
		return walkSequenceNode(node, seg, rest)
	case yaml.AliasNode:
		if node.Alias != nil {
			return walkYAMLNode(node.Alias, segments)
		}
	}
	// Cannot descend further; return this node as the closest match.
	return node, true
}

// walkMappingNode finds a key in a YAML mapping node and descends into its value.
// When the key is missing (e.g., a required field that is absent), returns the
// mapping node itself so the error can be associated with the parent object.
func walkMappingNode(node *yaml.Node, key string, rest []string) (*yaml.Node, bool) {
	// Mapping nodes store key-value pairs as consecutive Content entries.
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		if keyNode.Value == key {
			return walkYAMLNode(valNode, rest)
		}
	}
	// Key not found -- return the mapping node line as fallback.
	return node, true
}

// walkSequenceNode finds an element by integer index in a YAML sequence node
// and descends into it.
func walkSequenceNode(node *yaml.Node, seg string, rest []string) (*yaml.Node, bool) {
	idx, err := strconv.Atoi(seg)
	if err != nil {
		return node, true
	}
	if idx < 0 || idx >= len(node.Content) {
		return node, true
	}
	return walkYAMLNode(node.Content[idx], rest)
}
