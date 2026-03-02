package scaffold

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// parsesAsYAML returns true if the given string is valid YAML.
func parsesAsYAML(s string) bool {
	var out interface{}
	return yaml.Unmarshal([]byte(s), &out) == nil
}

// hasTopLevelKey checks whether a YAML string contains the given top-level key.
func hasTopLevelKey(s, key string) bool {
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(s), &doc); err != nil {
		return false
	}
	_, ok := doc[key]
	return ok
}

func TestDevice_ValidYAML(t *testing.T) {
	out := Device()
	if !parsesAsYAML(out) {
		t.Error("Device() output is not valid YAML")
	}
}

func TestDevice_RequiredTopLevelKeys(t *testing.T) {
	out := Device()
	requiredKeys := []string{
		"schema_version",
		"device",
		"connectivity",
		"registers",
		"register_map_variants",
		"diagnostics",
	}
	for _, k := range requiredKeys {
		if !hasTopLevelKey(out, k) {
			t.Errorf("Device() scaffold missing required top-level key %q", k)
		}
	}
}

func TestDevice_StartsWithComment(t *testing.T) {
	out := Device()
	first := strings.TrimSpace(out)
	if !strings.HasPrefix(first, "#") {
		t.Errorf("Device() scaffold should start with a comment line, got: %q", first[:min(50, len(first))])
	}
}

func TestDevice_NonSequentialAddresses(t *testing.T) {
	out := Device()
	// Verify the scaffold includes comments about non-sequential addresses.
	if !strings.Contains(out, "gap") {
		t.Error("Device() scaffold should mention address gaps to teach realistic PLC register maps")
	}
}

func TestNetwork_ValidYAML(t *testing.T) {
	out := Network()
	if !parsesAsYAML(out) {
		t.Error("Network() output is not valid YAML")
	}
}

func TestNetwork_RequiredTopLevelKeys(t *testing.T) {
	out := Network()
	requiredKeys := []string{
		"schema_version",
		"network",
		"properties",
	}
	for _, k := range requiredKeys {
		if !hasTopLevelKey(out, k) {
			t.Errorf("Network() scaffold missing required top-level key %q", k)
		}
	}
}

func TestNetwork_StartsWithComment(t *testing.T) {
	out := Network()
	first := strings.TrimSpace(out)
	if !strings.HasPrefix(first, "#") {
		t.Errorf("Network() scaffold should start with a comment line, got: %q", first[:min(50, len(first))])
	}
}

func TestEnvironment_ValidYAML(t *testing.T) {
	out := Environment()
	if !parsesAsYAML(out) {
		t.Error("Environment() output is not valid YAML")
	}
}

func TestEnvironment_RequiredTopLevelKeys(t *testing.T) {
	out := Environment()
	requiredKeys := []string{
		"schema_version",
		"environment",
		"networks",
		"placements",
	}
	for _, k := range requiredKeys {
		if !hasTopLevelKey(out, k) {
			t.Errorf("Environment() scaffold missing required top-level key %q", k)
		}
	}
}

func TestEnvironment_StartsWithComment(t *testing.T) {
	out := Environment()
	first := strings.TrimSpace(out)
	if !strings.HasPrefix(first, "#") {
		t.Errorf("Environment() scaffold should start with a comment line, got: %q", first[:min(50, len(first))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
