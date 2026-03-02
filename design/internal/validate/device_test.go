package validate

import (
	"fmt"
	"path/filepath"
	"testing"
)

// buildDeviceYAML produces a minimal valid device atom YAML with optional overrides.
const validDeviceYAML = `
schema_version: "0.1"
device:
  id: "test-plc"
  vendor: "Allen-Bradley"
  model: "TestPLC 1000"
  category: "plc"
  vintage: 2020
  description: "Test device"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
  response_delay_ms: 5
registers:
  max_holding: 100
  max_coils: 50
  max_input_registers: 100
  max_discrete_inputs: 50
  addressing: "zero-based"
  float_byte_order: "big-endian"
  max_registers_per_read: 125
register_map_variants:
  default:
    holding:
      - { address: 0, name: "tank_level", unit: "%", scale_min: 0, scale_max: 100, writable: false, description: "Tank level" }
    coils:
      - { address: 0, name: "pump_run", writable: true, description: "Pump run command" }
`

func writeDeviceYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	return writeTemp(t, dir, "device.yaml", content)
}

func TestValidateDevice_ValidDevice(t *testing.T) {
	path := writeDeviceYAML(t, validDeviceYAML)
	result := ValidateDevice(path)
	if result.HasErrors() {
		t.Errorf("expected no errors for valid device, got:\n%s", result.String(path))
	}
}

func TestValidateDevice_DEV001_SchemaVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "valid", version: `"0.1"`, wantErr: false},
		{name: "missing", version: `""`, wantErr: true},
		{name: "wrong version", version: `"0.2"`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := `
schema_version: ` + tc.version + `
device:
  id: "x"
  vendor: "V"
  model: "M"
  category: "plc"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
registers:
  max_holding: 10
  max_coils: 10
  addressing: "zero-based"
  float_byte_order: "big-endian"
`
			path := writeDeviceYAML(t, yaml)
			result := ValidateDevice(path)
			gotErr := containsRule(result, "DEV-001")
			if gotErr != tc.wantErr {
				t.Errorf("DEV-001: wantErr=%v, gotErr=%v\n%s", tc.wantErr, gotErr, result.String(path))
			}
		})
	}
}

func TestValidateDevice_DEV002_MissingID(t *testing.T) {
	yaml := `
schema_version: "0.1"
device:
  vendor: "V"
  model: "M"
  category: "plc"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
registers:
  max_holding: 10
  max_coils: 10
  addressing: "zero-based"
  float_byte_order: "big-endian"
`
	path := writeDeviceYAML(t, yaml)
	result := ValidateDevice(path)
	if !containsRule(result, "DEV-002") {
		t.Errorf("expected DEV-002 for missing id\n%s", result.String(path))
	}
}

func TestValidateDevice_DEV005_InvalidCategory(t *testing.T) {
	tests := []struct {
		category string
		wantErr  bool
	}{
		{"plc", false},
		{"gateway", false},
		{"hmi", false},
		{"sensor", false},
		{"relay", false},
		{"safety-controller", false},
		{"industrial-pc", true},
		{"", true},
	}
	for _, tc := range tests {
		t.Run(tc.category, func(t *testing.T) {
			yaml := buildDeviceWithCategory(tc.category)
			path := writeDeviceYAML(t, yaml)
			result := ValidateDevice(path)
			gotErr := containsRule(result, "DEV-005")
			if gotErr != tc.wantErr {
				t.Errorf("category=%q: wantErr=%v, gotErr=%v", tc.category, tc.wantErr, gotErr)
			}
		})
	}
}

func TestValidateDevice_DEV006_NoProtocol(t *testing.T) {
	yaml := `
schema_version: "0.1"
device:
  id: "x"
  vendor: "V"
  model: "M"
  category: "plc"
connectivity:
  ports:
    - type: "ethernet"
      protocols: []
registers:
  max_holding: 10
  max_coils: 10
  addressing: "zero-based"
  float_byte_order: "big-endian"
`
	path := writeDeviceYAML(t, yaml)
	result := ValidateDevice(path)
	if !containsRule(result, "DEV-006") {
		t.Errorf("expected DEV-006 for port with no protocols\n%s", result.String(path))
	}
}

func TestValidateDevice_DEV007_InvalidAddressing(t *testing.T) {
	tests := []struct {
		addressing string
		wantErr    bool
	}{
		{"zero-based", false},
		{"one-based", false},
		{"none", true},
		{"", true},
	}
	for _, tc := range tests {
		t.Run(tc.addressing, func(t *testing.T) {
			yaml := buildDeviceWithAddressing(tc.addressing)
			path := writeDeviceYAML(t, yaml)
			result := ValidateDevice(path)
			gotErr := containsRule(result, "DEV-007")
			if gotErr != tc.wantErr {
				t.Errorf("addressing=%q: wantErr=%v, gotErr=%v", tc.addressing, tc.wantErr, gotErr)
			}
		})
	}
}

func TestValidateDevice_DEV010_DEV016_HoldingAddressBounds(t *testing.T) {
	tests := []struct {
		name       string
		addressing string
		maxHolding int
		address    int
		wantErr    bool
		wantRule   string
	}{
		{
			name: "zero-based valid max-1",
			addressing: "zero-based", maxHolding: 10, address: 9,
			wantErr: false,
		},
		{
			name: "zero-based at max triggers error",
			addressing: "zero-based", maxHolding: 10, address: 10,
			wantErr: true, wantRule: "DEV-010",
		},
		{
			name: "one-based valid",
			addressing: "one-based", maxHolding: 10, address: 1,
			wantErr: false,
		},
		{
			name: "one-based address 0 invalid",
			addressing: "one-based", maxHolding: 10, address: 0,
			wantErr: true, wantRule: "DEV-016",
		},
		{
			name: "one-based exceeds max",
			addressing: "one-based", maxHolding: 10, address: 11,
			wantErr: true, wantRule: "DEV-016",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := buildDeviceWithHoldingAddress(tc.addressing, tc.maxHolding, tc.address)
			path := writeDeviceYAML(t, yaml)
			result := ValidateDevice(path)
			gotErr := result.HasErrors()
			if gotErr != tc.wantErr {
				t.Errorf("wantErr=%v, gotErr=%v\n%s", tc.wantErr, gotErr, result.String(path))
			}
			if tc.wantErr && tc.wantRule != "" && !containsRule(result, tc.wantRule) {
				t.Errorf("expected rule %s in errors\n%s", tc.wantRule, result.String(path))
			}
		})
	}
}

func TestValidateDevice_DEV012_ScaleMinMax(t *testing.T) {
	yaml := `
schema_version: "0.1"
device:
  id: "x"
  vendor: "V"
  model: "M"
  category: "plc"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
registers:
  max_holding: 10
  max_coils: 10
  addressing: "zero-based"
  float_byte_order: "big-endian"
register_map_variants:
  default:
    holding:
      - { address: 0, name: "bad", unit: "%", scale_min: 100, scale_max: 50, writable: false, description: "bad" }
`
	path := writeDeviceYAML(t, yaml)
	result := ValidateDevice(path)
	if !containsRule(result, "DEV-012") {
		t.Errorf("expected DEV-012 for inverted scale\n%s", result.String(path))
	}
}

func TestValidateDevice_DEV013_EmptyUnit(t *testing.T) {
	yaml := `
schema_version: "0.1"
device:
  id: "x"
  vendor: "V"
  model: "M"
  category: "plc"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
registers:
  max_holding: 10
  max_coils: 10
  addressing: "zero-based"
  float_byte_order: "big-endian"
register_map_variants:
  default:
    holding:
      - { address: 0, name: "r1", unit: "", scale_min: 0, scale_max: 100, writable: false, description: "r1" }
`
	path := writeDeviceYAML(t, yaml)
	result := ValidateDevice(path)
	if !containsRule(result, "DEV-013") {
		t.Errorf("expected DEV-013 for empty unit\n%s", result.String(path))
	}
}

func TestValidateDevice_DEV014_DEV015_DuplicateNamesAddresses(t *testing.T) {
	yaml := `
schema_version: "0.1"
device:
  id: "x"
  vendor: "V"
  model: "M"
  category: "plc"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
registers:
  max_holding: 20
  max_coils: 20
  addressing: "zero-based"
  float_byte_order: "big-endian"
register_map_variants:
  default:
    holding:
      - { address: 0, name: "dup_name", unit: "%", scale_min: 0, scale_max: 100, writable: false, description: "r1" }
      - { address: 0, name: "dup_name", unit: "%", scale_min: 0, scale_max: 100, writable: false, description: "r2" }
`
	path := writeDeviceYAML(t, yaml)
	result := ValidateDevice(path)
	if !containsRule(result, "DEV-014") {
		t.Errorf("expected DEV-014 for duplicate name\n%s", result.String(path))
	}
	if !containsRule(result, "DEV-015") {
		t.Errorf("expected DEV-015 for duplicate address\n%s", result.String(path))
	}
}

func TestValidateDevice_DEV017_ResponseDelayWarning(t *testing.T) {
	tests := []struct {
		name        string
		portType    string
		delayMs     int
		wantWarning bool
	}{
		{name: "ethernet in range", portType: "ethernet", delayMs: 5, wantWarning: false},
		{name: "ethernet below min", portType: "ethernet", delayMs: 1, wantWarning: true},
		{name: "ethernet above max", portType: "ethernet", delayMs: 2001, wantWarning: true},
		{name: "serial in range", portType: "rs485", delayMs: 50, wantWarning: false},
		{name: "serial below min", portType: "rs485", delayMs: 5, wantWarning: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := buildDeviceWithDelay(tc.portType, tc.delayMs)
			path := writeDeviceYAML(t, yaml)
			result := ValidateDevice(path)
			gotWarning := containsRuleWithSeverity(result, "DEV-017", SeverityWarning)
			if gotWarning != tc.wantWarning {
				t.Errorf("wantWarning=%v, gotWarning=%v\n%s", tc.wantWarning, gotWarning, result.String(path))
			}
		})
	}
}

func TestValidateDevice_DEV018_FloatByteOrder(t *testing.T) {
	tests := []struct {
		name         string
		vendor       string
		byteOrder    string
		wantError    bool
		wantWarning  bool
	}{
		{name: "valid big-endian", vendor: "Generic", byteOrder: "big-endian", wantError: false, wantWarning: false},
		{name: "invalid value", vendor: "Generic", byteOrder: "middle-endian", wantError: true, wantWarning: false},
		{
			name: "Schneider big-endian warns",
			vendor: "Schneider Electric", byteOrder: "big-endian",
			wantError: false, wantWarning: true,
		},
		{
			name: "Allen-Bradley little-endian warns",
			vendor: "Allen-Bradley", byteOrder: "little-endian",
			wantError: false, wantWarning: true,
		},
		{
			name: "Allen-Bradley big-endian ok",
			vendor: "Allen-Bradley", byteOrder: "big-endian",
			wantError: false, wantWarning: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := buildDeviceWithByteOrder(tc.vendor, tc.byteOrder)
			path := writeDeviceYAML(t, yaml)
			result := ValidateDevice(path)
			gotError := containsRule(result, "DEV-018") && containsRuleWithSeverity(result, "DEV-018", SeverityError)
			gotWarning := containsRuleWithSeverity(result, "DEV-018", SeverityWarning)
			if gotError != tc.wantError {
				t.Errorf("wantError=%v, gotError=%v\n%s", tc.wantError, gotError, result.String(path))
			}
			if gotWarning != tc.wantWarning {
				t.Errorf("wantWarning=%v, gotWarning=%v\n%s", tc.wantWarning, gotWarning, result.String(path))
			}
		})
	}
}

func TestValidateDevice_DEV019_WritableMeasurement(t *testing.T) {
	yaml := `
schema_version: "0.1"
device:
  id: "x"
  vendor: "V"
  model: "M"
  category: "sensor"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
registers:
  max_holding: 10
  max_coils: 10
  addressing: "zero-based"
  float_byte_order: "big-endian"
register_map_variants:
  default:
    holding:
      - { address: 0, name: "water_flow", unit: "L/s", scale_min: 0, scale_max: 100, writable: true, description: "Sensor measurement" }
`
	path := writeDeviceYAML(t, yaml)
	result := ValidateDevice(path)
	if !containsRuleWithSeverity(result, "DEV-019", SeverityWarning) {
		t.Errorf("expected DEV-019 warning for writable measurement\n%s", result.String(path))
	}
}

// TestValidateDevice_ExistingDevices validates all 4 existing device atoms.
func TestValidateDevice_ExistingDevices(t *testing.T) {
	designRoot := findDesignRootForTests(t)
	devices := []string{
		"compactlogix-l33er.yaml",
		"slc-500-05.yaml",
		"moxa-nport-5150.yaml",
		"modicon-984.yaml",
	}
	for _, name := range devices {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(designRoot, "devices", name)
			result := ValidateDevice(path)
			if result.HasErrors() {
				t.Errorf("existing device %s has validation errors:\n%s", name, result.String(path))
			}
		})
	}
}

// --- Helpers ---

func containsRule(r *ValidationResult, ruleID string) bool {
	for _, e := range r.Errors {
		if e.RuleID == ruleID {
			return true
		}
	}
	return false
}

func containsRuleWithSeverity(r *ValidationResult, ruleID string, sev Severity) bool {
	for _, e := range r.Errors {
		if e.RuleID == ruleID && e.Severity == sev {
			return true
		}
	}
	return false
}

func findDesignRootForTests(t *testing.T) string {
	t.Helper()
	// Walk up from the test file location to find the design root.
	root, err := FindDesignRoot(".")
	if err != nil {
		t.Skipf("design root not found (run tests from within design/ directory): %v", err)
	}
	return root
}

func buildDeviceWithCategory(category string) string {
	return `
schema_version: "0.1"
device:
  id: "x"
  vendor: "V"
  model: "M"
  category: "` + category + `"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
registers:
  max_holding: 10
  max_coils: 10
  addressing: "zero-based"
  float_byte_order: "big-endian"
`
}

func buildDeviceWithAddressing(addressing string) string {
	return `
schema_version: "0.1"
device:
  id: "x"
  vendor: "V"
  model: "M"
  category: "plc"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
registers:
  max_holding: 10
  max_coils: 10
  addressing: "` + addressing + `"
  float_byte_order: "big-endian"
`
}

func buildDeviceWithHoldingAddress(addressing string, maxHolding, address int) string {
	return `
schema_version: "0.1"
device:
  id: "x"
  vendor: "V"
  model: "M"
  category: "plc"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
registers:
  max_holding: ` + itoa(maxHolding) + `
  max_coils: 10
  addressing: "` + addressing + `"
  float_byte_order: "big-endian"
register_map_variants:
  v1:
    holding:
      - { address: ` + itoa(address) + `, name: "r1", unit: "%", scale_min: 0, scale_max: 100, writable: false, description: "r1" }
`
}

func buildDeviceWithDelay(portType string, delayMs int) string {
	return `
schema_version: "0.1"
device:
  id: "x"
  vendor: "V"
  model: "M"
  category: "plc"
connectivity:
  ports:
    - type: "` + portType + `"
      protocols: ["modbus-rtu"]
  response_delay_ms: ` + itoa(delayMs) + `
registers:
  max_holding: 10
  max_coils: 10
  addressing: "zero-based"
  float_byte_order: "big-endian"
`
}

func buildDeviceWithByteOrder(vendor, byteOrder string) string {
	return `
schema_version: "0.1"
device:
  id: "x"
  vendor: "` + vendor + `"
  model: "M"
  category: "plc"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
registers:
  max_holding: 10
  max_coils: 10
  addressing: "zero-based"
  float_byte_order: "` + byteOrder + `"
`
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
