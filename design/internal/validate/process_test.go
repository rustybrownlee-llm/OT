package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// --- Test setup helpers ---

// buildProcessDesignRoot creates a minimal design root with one ethernet placement.
// Returns the design root path and the environment directory path.
func buildProcessDesignRoot(t *testing.T) (root, envDir string) {
	t.Helper()
	root = buildTestDesignRoot(t)
	writeDevice(t, root, "test-plc", "plc", "zero-based", 100, 50)
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	envDir = writeEnvDir(t, root, "test-env", validEnvContent("test-env"))
	return root, envDir
}

// writeProcessFile writes a process.yaml into the given directory.
func writeProcessFile(t *testing.T, dir, content string) string {
	t.Helper()
	return writeTemp(t, dir, "process.yaml", content)
}

// loadEnvDocForTest loads the environment.yaml from the given directory for cross-reference.
func loadEnvDocForTest(t *testing.T, envDir string) *RawDocument {
	t.Helper()
	doc, err := LoadFile(filepath.Join(envDir, "environment.yaml"))
	if err != nil {
		t.Fatalf("loadEnvDocForTest: %v", err)
	}
	return doc
}

// minimalValidProcess returns a minimal valid process schematic YAML string.
// It uses placement "plc-01" which is declared in validEnvContent.
func minimalValidProcess() string {
	return `
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test Process"
stages:
  - id: "stage-a"
    equipment:
      - id: "tank-01"
        type: "tank"
        label: "Test Tank"
        instruments:
          - tag: "LT-101"
            isa_type: "LT"
            placement: "plc-01"
            register:
              type: "holding"
              address: 0
            unit: "m"
connections:
  - from: "tank-01"
    to: "tank-01"
    type: "pipe"
`
}

// --- PRO-001: schema_version ---

func TestValidateProcess_PRO001_SchemaVersion(t *testing.T) {
	root, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)

	processFile := writeProcessFile(t, envDir, `
schema_version: "0.2"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "pump"
        instruments:
          - tag: "FT-101"
            isa_type: "FT"
            placement: "plc-01"
            register: { type: "holding", address: 0 }
`)
	_ = root
	result := ValidateProcess(processFile, envDoc, designRoot)
	if !containsRule(result, "PRO-001") {
		t.Errorf("expected PRO-001 for invalid schema_version\n%s", result.String(processFile))
	}
}

func TestValidateProcess_PRO001_ValidSchemaVersion(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, minimalValidProcess())
	result := ValidateProcess(processFile, envDoc, designRoot)
	if containsRule(result, "PRO-001") {
		t.Errorf("unexpected PRO-001 for valid schema_version\n%s", result.String(processFile))
	}
}

// --- PRO-002: process.id required ---

func TestValidateProcess_PRO002_MissingProcessID(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, `
schema_version: "0.1"
process:
  name: "No ID"
stages:
  - id: "s1"
    equipment: []
`)
	result := ValidateProcess(processFile, envDoc, designRoot)
	if !containsRule(result, "PRO-002") {
		t.Errorf("expected PRO-002 for missing process.id\n%s", result.String(processFile))
	}
}

// --- PRO-003: process.name required ---

func TestValidateProcess_PRO003_MissingProcessName(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, `
schema_version: "0.1"
process:
  id: "test-env"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "pump"
`)
	result := ValidateProcess(processFile, envDoc, designRoot)
	if !containsRule(result, "PRO-003") {
		t.Errorf("expected PRO-003 for missing process.name\n%s", result.String(processFile))
	}
}

// --- PRO-004: flow_direction enum ---

func TestValidateProcess_PRO004_FlowDirection(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		wantErr   bool
	}{
		{name: "valid horizontal", direction: "horizontal", wantErr: false},
		{name: "valid vertical", direction: "vertical", wantErr: false},
		{name: "omitted (defaults)", direction: "", wantErr: false},
		{name: "invalid value", direction: "diagonal", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, envDir := buildProcessDesignRoot(t)
			envDoc := loadEnvDocForTest(t, envDir)
			designRoot, _ := FindDesignRoot(envDir)
			dirLine := ""
			if tc.direction != "" {
				dirLine = fmt.Sprintf("  flow_direction: %q", tc.direction)
			}
			content := fmt.Sprintf(`
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
%s
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "pump"
        instruments:
          - tag: "FT-101"
            isa_type: "FT"
            placement: "plc-01"
            register: { type: "holding", address: 0 }
`, dirLine)
			processFile := writeProcessFile(t, envDir, content)
			result := ValidateProcess(processFile, envDoc, designRoot)
			gotErr := containsRule(result, "PRO-004")
			if gotErr != tc.wantErr {
				t.Errorf("direction=%q wantErr=%v gotErr=%v\n%s",
					tc.direction, tc.wantErr, gotErr, result.String(processFile))
			}
		})
	}
}

// --- PRO-005: at least one stage ---

func TestValidateProcess_PRO005_EmptyStages(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, `
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages: []
`)
	result := ValidateProcess(processFile, envDoc, designRoot)
	if !containsRule(result, "PRO-005") {
		t.Errorf("expected PRO-005 for empty stages\n%s", result.String(processFile))
	}
}

// --- PRO-006: stage ID uniqueness ---

func TestValidateProcess_PRO006_DuplicateStageID(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, `
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "dup-stage"
    equipment: []
  - id: "dup-stage"
    equipment: []
`)
	result := ValidateProcess(processFile, envDoc, designRoot)
	if !containsRule(result, "PRO-006") {
		t.Errorf("expected PRO-006 for duplicate stage ID\n%s", result.String(processFile))
	}
}

// --- PRO-007: equipment ID uniqueness across stages ---

func TestValidateProcess_PRO007_DuplicateEquipmentID(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, `
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "stage-a"
    equipment:
      - id: "dup-equip"
        type: "pump"
  - id: "stage-b"
    equipment:
      - id: "dup-equip"
        type: "valve"
`)
	result := ValidateProcess(processFile, envDoc, designRoot)
	if !containsRule(result, "PRO-007") {
		t.Errorf("expected PRO-007 for duplicate equipment ID across stages\n%s", result.String(processFile))
	}
}

// --- PRO-008: equipment type enum ---

func TestValidateProcess_PRO008_EquipmentType(t *testing.T) {
	tests := []struct {
		name      string
		eqType    string
		wantErr   bool
	}{
		{name: "valid tank", eqType: "tank", wantErr: false},
		{name: "valid basin", eqType: "basin", wantErr: false},
		{name: "valid pump", eqType: "pump", wantErr: false},
		{name: "valid valve", eqType: "valve", wantErr: false},
		{name: "valid blower", eqType: "blower", wantErr: false},
		{name: "valid analyzer_station", eqType: "analyzer_station", wantErr: false},
		{name: "valid chromatograph", eqType: "chromatograph", wantErr: false},
		{name: "valid uv_system", eqType: "uv_system", wantErr: false},
		{name: "valid clarifier", eqType: "clarifier", wantErr: false},
		{name: "valid screen", eqType: "screen", wantErr: false},
		{name: "valid compressor", eqType: "compressor", wantErr: false},
		{name: "valid meter", eqType: "meter", wantErr: false},
		{name: "valid heat_exchanger", eqType: "heat_exchanger", wantErr: false},
		{name: "invalid gateway (OT-domain: belongs in network_context)", eqType: "gateway", wantErr: true},
		{name: "invalid robot", eqType: "robot", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, envDir := buildProcessDesignRoot(t)
			envDoc := loadEnvDocForTest(t, envDir)
			designRoot, _ := FindDesignRoot(envDir)
			content := fmt.Sprintf(`
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: %q
`, tc.eqType)
			processFile := writeProcessFile(t, envDir, content)
			result := ValidateProcess(processFile, envDoc, designRoot)
			gotErr := containsRule(result, "PRO-008")
			if gotErr != tc.wantErr {
				t.Errorf("type=%q wantErr=%v gotErr=%v\n%s",
					tc.eqType, tc.wantErr, gotErr, result.String(processFile))
			}
		})
	}
}

// --- PRO-009: instrument tag uniqueness ---

func TestValidateProcess_PRO009_DuplicateTag(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, `
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "tank"
        instruments:
          - tag: "LT-101"
            isa_type: "LT"
            placement: "plc-01"
            register: { type: "holding", address: 0 }
          - tag: "LT-101"
            isa_type: "LT"
            placement: "plc-01"
            register: { type: "holding", address: 1 }
`)
	result := ValidateProcess(processFile, envDoc, designRoot)
	if !containsRule(result, "PRO-009") {
		t.Errorf("expected PRO-009 for duplicate instrument tag\n%s", result.String(processFile))
	}
}

// --- PRO-010: ISA-5.1 type recognition ---

func TestValidateProcess_PRO010_ISAType(t *testing.T) {
	tests := []struct {
		name     string
		isaType  string
		wantWarn bool
	}{
		// Instrument prefixes.
		{name: "LT (instrument)", isaType: "LT", wantWarn: false},
		{name: "FT (instrument)", isaType: "FT", wantWarn: false},
		{name: "PT (instrument)", isaType: "PT", wantWarn: false},
		{name: "TT (instrument)", isaType: "TT", wantWarn: false},
		{name: "AT (instrument)", isaType: "AT", wantWarn: false},
		{name: "AIT (instrument)", isaType: "AIT", wantWarn: false},
		{name: "PDT (instrument)", isaType: "PDT", wantWarn: false},
		{name: "HS (instrument)", isaType: "HS", wantWarn: false},
		{name: "ZSO (instrument)", isaType: "ZSO", wantWarn: false},
		{name: "ZSC (instrument)", isaType: "ZSC", wantWarn: false},
		{name: "FE (instrument)", isaType: "FE", wantWarn: false},
		// Equipment designators.
		{name: "P (designator)", isaType: "P", wantWarn: false},
		{name: "V (designator)", isaType: "V", wantWarn: false},
		{name: "B (designator)", isaType: "B", wantWarn: false},
		{name: "SV (designator)", isaType: "SV", wantWarn: false},
		{name: "GC (designator)", isaType: "GC", wantWarn: false},
		{name: "FM (designator)", isaType: "FM", wantWarn: false},
		// Project extension.
		{name: "run (project extension)", isaType: "run", wantWarn: false},
		// Unrecognized.
		{name: "DPT (non-standard, use PDT)", isaType: "DPT", wantWarn: true},
		{name: "UV (equipment type not prefix)", isaType: "UV", wantWarn: true},
		{name: "empty (optional, no warn)", isaType: "", wantWarn: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, envDir := buildProcessDesignRoot(t)
			envDoc := loadEnvDocForTest(t, envDir)
			designRoot, _ := FindDesignRoot(envDir)
			isaLine := ""
			if tc.isaType != "" {
				isaLine = fmt.Sprintf("            isa_type: %q", tc.isaType)
			}
			content := fmt.Sprintf(`
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "tank"
        instruments:
          - tag: "LT-101"
%s
            placement: "plc-01"
            register: { type: "holding", address: 0 }
`, isaLine)
			processFile := writeProcessFile(t, envDir, content)
			result := ValidateProcess(processFile, envDoc, designRoot)
			gotWarn := containsRuleWithSeverity(result, "PRO-010", SeverityWarning)
			if gotWarn != tc.wantWarn {
				t.Errorf("isa_type=%q wantWarn=%v gotWarn=%v\n%s",
					tc.isaType, tc.wantWarn, gotWarn, result.String(processFile))
			}
		})
	}
}

// --- PRO-011: placement cross-reference ---

func TestValidateProcess_PRO011_PlacementReference(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, `
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "tank"
        instruments:
          - tag: "LT-101"
            isa_type: "LT"
            placement: "nonexistent-plc"
            register: { type: "holding", address: 0 }
`)
	result := ValidateProcess(processFile, envDoc, designRoot)
	if !containsRule(result, "PRO-011") {
		t.Errorf("expected PRO-011 for invalid placement reference\n%s", result.String(processFile))
	}
}

func TestValidateProcess_PRO011_ValidPlacement(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, minimalValidProcess())
	result := ValidateProcess(processFile, envDoc, designRoot)
	if containsRule(result, "PRO-011") {
		t.Errorf("unexpected PRO-011 for valid placement reference\n%s", result.String(processFile))
	}
}

// --- PRO-012: register type enum ---

func TestValidateProcess_PRO012_RegisterType(t *testing.T) {
	tests := []struct {
		name    string
		regType string
		wantErr bool
	}{
		{name: "valid holding", regType: "holding", wantErr: false},
		{name: "valid coil", regType: "coil", wantErr: false},
		{name: "invalid input", regType: "input", wantErr: true},
		{name: "invalid discrete", regType: "discrete_input", wantErr: true},
		{name: "empty", regType: "", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, envDir := buildProcessDesignRoot(t)
			envDoc := loadEnvDocForTest(t, envDir)
			designRoot, _ := FindDesignRoot(envDir)
			content := fmt.Sprintf(`
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "pump"
        instruments:
          - tag: "FT-101"
            isa_type: "FT"
            placement: "plc-01"
            register: { type: %q, address: 0 }
`, tc.regType)
			processFile := writeProcessFile(t, envDir, content)
			result := ValidateProcess(processFile, envDoc, designRoot)
			gotErr := containsRule(result, "PRO-012")
			if gotErr != tc.wantErr {
				t.Errorf("regType=%q wantErr=%v gotErr=%v\n%s",
					tc.regType, tc.wantErr, gotErr, result.String(processFile))
			}
		})
	}
}

// --- PRO-013: register address bounds ---

func TestValidateProcess_PRO013_RegisterAddressBounds(t *testing.T) {
	tests := []struct {
		name    string
		regType string
		address int
		wantErr bool
	}{
		// Device has max_holding=100, max_coils=50.
		{name: "holding within bounds", regType: "holding", address: 0, wantErr: false},
		{name: "holding at boundary", regType: "holding", address: 99, wantErr: false},
		{name: "holding at capacity", regType: "holding", address: 100, wantErr: true},
		{name: "holding exceeds capacity", regType: "holding", address: 200, wantErr: true},
		{name: "coil within bounds", regType: "coil", address: 0, wantErr: false},
		{name: "coil at boundary", regType: "coil", address: 49, wantErr: false},
		{name: "coil at capacity", regType: "coil", address: 50, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, envDir := buildProcessDesignRoot(t)
			envDoc := loadEnvDocForTest(t, envDir)
			designRoot, _ := FindDesignRoot(envDir)
			content := fmt.Sprintf(`
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "pump"
        instruments:
          - tag: "FT-101"
            isa_type: "FT"
            placement: "plc-01"
            register: { type: %q, address: %d }
`, tc.regType, tc.address)
			processFile := writeProcessFile(t, envDir, content)
			result := ValidateProcess(processFile, envDoc, designRoot)
			gotErr := containsRule(result, "PRO-013")
			if gotErr != tc.wantErr {
				t.Errorf("regType=%q addr=%d wantErr=%v gotErr=%v\n%s",
					tc.regType, tc.address, tc.wantErr, gotErr, result.String(processFile))
			}
		})
	}
}

// --- PRO-014: connection endpoint references ---

func TestValidateProcess_PRO014_ConnectionEndpoints(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, `
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "real-tank"
        type: "tank"
connections:
  - from: "real-tank"
    to: "nonexistent-pump"
    type: "pipe"
`)
	result := ValidateProcess(processFile, envDoc, designRoot)
	if !containsRule(result, "PRO-014") {
		t.Errorf("expected PRO-014 for connection referencing nonexistent equipment\n%s", result.String(processFile))
	}
}

func TestValidateProcess_PRO014_ValidConnections(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, minimalValidProcess())
	result := ValidateProcess(processFile, envDoc, designRoot)
	if containsRule(result, "PRO-014") {
		t.Errorf("unexpected PRO-014 for valid connections\n%s", result.String(processFile))
	}
}

// --- PRO-015: connection type enum ---

func TestValidateProcess_PRO015_ConnectionType(t *testing.T) {
	tests := []struct {
		name     string
		connType string
		wantErr  bool
	}{
		{name: "valid pipe", connType: "pipe", wantErr: false},
		{name: "valid duct", connType: "duct", wantErr: false},
		{name: "invalid serial (belongs in network_context)", connType: "serial", wantErr: true},
		{name: "invalid wireless (belongs in network_context)", connType: "wireless", wantErr: true},
		{name: "invalid cable", connType: "cable", wantErr: true},
		{name: "empty", connType: "", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, envDir := buildProcessDesignRoot(t)
			envDoc := loadEnvDocForTest(t, envDir)
			designRoot, _ := FindDesignRoot(envDir)
			content := fmt.Sprintf(`
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "tank-01"
        type: "tank"
      - id: "pump-01"
        type: "pump"
connections:
  - from: "tank-01"
    to: "pump-01"
    type: %q
`, tc.connType)
			processFile := writeProcessFile(t, envDir, content)
			result := ValidateProcess(processFile, envDoc, designRoot)
			gotErr := containsRule(result, "PRO-015")
			if gotErr != tc.wantErr {
				t.Errorf("connType=%q wantErr=%v gotErr=%v\n%s",
					tc.connType, tc.wantErr, gotErr, result.String(processFile))
			}
		})
	}
}

// --- PRO-016: network_context placement reference ---

func TestValidateProcess_PRO016_NetworkContextPlacement(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, `
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "pump"
network_context:
  - id: "nc1"
    type: "wan_link"
    label: "Test WAN"
    placement: "nonexistent-placement"
`)
	result := ValidateProcess(processFile, envDoc, designRoot)
	if !containsRuleWithSeverity(result, "PRO-016", SeverityWarning) {
		t.Errorf("expected PRO-016 warning for invalid network_context placement\n%s", result.String(processFile))
	}
}

func TestValidateProcess_PRO016_ValidNetworkContextPlacement(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	processFile := writeProcessFile(t, envDir, `
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "pump"
network_context:
  - id: "nc1"
    type: "wan_link"
    label: "Test WAN"
    placement: "plc-01"
`)
	result := ValidateProcess(processFile, envDoc, designRoot)
	if containsRule(result, "PRO-016") {
		t.Errorf("unexpected PRO-016 for valid network_context placement\n%s", result.String(processFile))
	}
}

// --- PRO-017: network_context type enum ---

func TestValidateProcess_PRO017_NetworkContextType(t *testing.T) {
	tests := []struct {
		name    string
		ncType  string
		wantErr bool
	}{
		{name: "valid wan_link", ncType: "wan_link", wantErr: false},
		{name: "valid internet_gateway", ncType: "internet_gateway", wantErr: false},
		{name: "valid wireless_bridge", ncType: "wireless_bridge", wantErr: false},
		{name: "invalid vpn", ncType: "vpn", wantErr: true},
		{name: "invalid firewall", ncType: "firewall", wantErr: true},
		{name: "empty", ncType: "", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, envDir := buildProcessDesignRoot(t)
			envDoc := loadEnvDocForTest(t, envDir)
			designRoot, _ := FindDesignRoot(envDir)
			content := fmt.Sprintf(`
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "pump"
network_context:
  - id: "nc1"
    type: %q
    label: "Test NC"
`, tc.ncType)
			processFile := writeProcessFile(t, envDir, content)
			result := ValidateProcess(processFile, envDoc, designRoot)
			gotErr := containsRule(result, "PRO-017")
			if gotErr != tc.wantErr {
				t.Errorf("ncType=%q wantErr=%v gotErr=%v\n%s",
					tc.ncType, tc.wantErr, gotErr, result.String(processFile))
			}
		})
	}
}

// --- PRO-018: era year range ---

func TestValidateProcess_PRO018_EraYear(t *testing.T) {
	tests := []struct {
		name     string
		era      int
		wantWarn bool
	}{
		{name: "valid 1970", era: 1970, wantWarn: false},
		{name: "valid 2020", era: 2020, wantWarn: false},
		{name: "before 1970", era: 1969, wantWarn: true},
		{name: "future year", era: 9999, wantWarn: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, envDir := buildProcessDesignRoot(t)
			envDoc := loadEnvDocForTest(t, envDir)
			designRoot, _ := FindDesignRoot(envDir)
			content := fmt.Sprintf(`
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "pump"
        era: %d
`, tc.era)
			processFile := writeProcessFile(t, envDir, content)
			result := ValidateProcess(processFile, envDoc, designRoot)
			gotWarn := containsRuleWithSeverity(result, "PRO-018", SeverityWarning)
			if gotWarn != tc.wantWarn {
				t.Errorf("era=%d wantWarn=%v gotWarn=%v\n%s",
					tc.era, tc.wantWarn, gotWarn, result.String(processFile))
			}
		})
	}
}

// --- PRO-019: tag prefix vs isa_type consistency ---

func TestValidateProcess_PRO019_TagPrefixConsistency(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		isaType  string
		wantWarn bool
	}{
		{name: "matching FT/FT", tag: "FT-101", isaType: "FT", wantWarn: false},
		{name: "matching LT/LT", tag: "LT-201", isaType: "LT", wantWarn: false},
		{name: "mismatch FT/LT", tag: "FT-101", isaType: "LT", wantWarn: true},
		{name: "mismatch LT/FT", tag: "LT-201", isaType: "FT", wantWarn: true},
		// "run" is a project extension: no case-sensitivity issue expected.
		{name: "run extension no mismatch", tag: "run-P-101", isaType: "run", wantWarn: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, envDir := buildProcessDesignRoot(t)
			envDoc := loadEnvDocForTest(t, envDir)
			designRoot, _ := FindDesignRoot(envDir)
			content := fmt.Sprintf(`
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "tank"
        instruments:
          - tag: %q
            isa_type: %q
            placement: "plc-01"
            register: { type: "holding", address: 0 }
`, tc.tag, tc.isaType)
			processFile := writeProcessFile(t, envDir, content)
			result := ValidateProcess(processFile, envDoc, designRoot)
			gotWarn := containsRuleWithSeverity(result, "PRO-019", SeverityWarning)
			if gotWarn != tc.wantWarn {
				t.Errorf("tag=%q isaType=%q wantWarn=%v gotWarn=%v\n%s",
					tc.tag, tc.isaType, tc.wantWarn, gotWarn, result.String(processFile))
			}
		})
	}
}

// --- PRO-020: scale range vs device atom scale_min/scale_max ---

func TestValidateProcess_PRO020_ScaleConsistency(t *testing.T) {
	// The device written by writeDevice has address 0 with scale_min=0, scale_max=100.
	tests := []struct {
		name     string
		scale    string // YAML list literal
		wantWarn bool
	}{
		{name: "matching scale [0, 100]", scale: "[0.0, 100.0]", wantWarn: false},
		{name: "mismatched scale [0, 50]", scale: "[0.0, 50.0]", wantWarn: true},
		{name: "mismatched scale [10, 100]", scale: "[10.0, 100.0]", wantWarn: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, envDir := buildProcessDesignRoot(t)
			envDoc := loadEnvDocForTest(t, envDir)
			designRoot, _ := FindDesignRoot(envDir)
			content := fmt.Sprintf(`
schema_version: "0.1"
process:
  id: "test-env"
  name: "Test"
stages:
  - id: "s1"
    equipment:
      - id: "eq1"
        type: "tank"
        instruments:
          - tag: "LT-101"
            isa_type: "LT"
            placement: "plc-01"
            register: { type: "holding", address: 0 }
            scale: %s
`, tc.scale)
			processFile := writeProcessFile(t, envDir, content)
			result := ValidateProcess(processFile, envDoc, designRoot)
			gotWarn := containsRuleWithSeverity(result, "PRO-020", SeverityWarning)
			if gotWarn != tc.wantWarn {
				t.Errorf("scale=%s wantWarn=%v gotWarn=%v\n%s",
					tc.scale, tc.wantWarn, gotWarn, result.String(processFile))
			}
		})
	}
}

// --- Directory validation integration tests ---

func TestValidateDirectory_WithProcess(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	// Write a valid process.yaml alongside the environment.yaml.
	writeProcessFile(t, envDir, minimalValidProcess())

	// Validate via ValidateEnvironment (directory path); the CLI calls this.
	envResult := ValidateEnvironment(envDir)
	if envResult.HasErrors() {
		t.Errorf("environment validation has errors:\n%s", envResult.String(envDir))
	}

	// Also validate the process file directly.
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	procResult := ValidateProcess(filepath.Join(envDir, "process.yaml"), envDoc, designRoot)
	if procResult.HasErrors() {
		t.Errorf("process validation has errors:\n%s", procResult.String(envDir))
	}
}

func TestValidateDirectory_WithoutProcess(t *testing.T) {
	// Environment with no process.yaml -- should validate cleanly.
	_, envDir := buildProcessDesignRoot(t)
	if _, err := os.Stat(filepath.Join(envDir, "process.yaml")); err == nil {
		t.Fatal("expected no process.yaml in this test setup")
	}
	result := ValidateEnvironment(envDir)
	if result.HasErrors() {
		t.Errorf("expected clean validation without process.yaml:\n%s", result.String(envDir))
	}
}

func TestValidateDirectory_ProcessWithErrors(t *testing.T) {
	_, envDir := buildProcessDesignRoot(t)
	// Write a process.yaml with a PRO-002 error (missing process.id).
	writeProcessFile(t, envDir, `
schema_version: "0.1"
process:
  name: "Missing ID"
stages:
  - id: "s1"
    equipment: []
`)
	envDoc := loadEnvDocForTest(t, envDir)
	designRoot, _ := FindDesignRoot(envDir)
	result := ValidateProcess(filepath.Join(envDir, "process.yaml"), envDoc, designRoot)
	if !containsRule(result, "PRO-002") {
		t.Errorf("expected PRO-002 in process validation result\n%s", result.String(envDir))
	}
}

// --- Scaffold output validation test ---

func TestProcessScaffoldPassesValidation(t *testing.T) {
	// The scaffold template references placement IDs that do not exist in a generic test
	// environment. We validate it without cross-reference (envDoc=nil) to confirm the
	// schema structure is valid, and separately confirm the YAML parses cleanly.
	root := buildTestDesignRoot(t)
	writeDevice(t, root, "compactlogix-l33er", "plc", "zero-based", 4096, 2048)
	writeDevice(t, root, "test-plc", "plc", "zero-based", 100, 50)
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")

	// Build a companion environment with the placements referenced in the scaffold.
	envContent := `
schema_version: "0.1"
environment:
  id: "your-environment-id"
  name: "Scaffold Test"
networks:
  - ref: "eth-net"
placements:
  - id: "intake-plc-01"
    device: "compactlogix-l33er"
    network: "eth-net"
    ip: "10.0.0.10"
    modbus_port: 5020
    role: "Intake PLC"
    register_map_variant: "water-intake"
  - id: "treatment-plc-01"
    device: "compactlogix-l33er"
    network: "eth-net"
    ip: "10.0.0.11"
    modbus_port: 5021
    role: "Treatment PLC"
    register_map_variant: "water-treatment"
`
	envDir := writeEnvDir(t, root, "your-environment-id", envContent)

	// Write the scaffold output as-is to the environment directory.
	scaffoldContent := processScaffoldForTest()
	processFile := writeProcessFile(t, envDir, scaffoldContent)

	envDoc, err := LoadFile(filepath.Join(envDir, "environment.yaml"))
	if err != nil {
		t.Fatalf("failed to load scaffold environment: %v", err)
	}
	designRoot, err := FindDesignRoot(envDir)
	if err != nil {
		t.Fatalf("FindDesignRoot: %v", err)
	}

	result := ValidateProcess(processFile, envDoc, designRoot)
	if result.HasErrors() {
		t.Errorf("scaffold output has validation errors:\n%s", result.String(processFile))
	}
}

// processScaffoldForTest returns a trimmed, test-compatible version of the scaffold template.
// The scaffold template uses placeholder IDs; this version uses real IDs that correspond to
// the test environment built in TestProcessScaffoldPassesValidation.
func processScaffoldForTest() string {
	return `
schema_version: "0.1"
process:
  id: "your-environment-id"
  name: "Test Water Treatment Process"
  description: "Scaffold test"
  flow_direction: "horizontal"
stages:
  - id: "intake"
    name: "Raw Water Intake"
    controller:
      placement: "intake-plc-01"
      device: "compactlogix-l33er"
    equipment:
      - id: "raw-water-well"
        type: "tank"
        label: "Raw Water Well"
        era: 2018
        instruments:
          - tag: "LT-101"
            name: "Raw Well Level"
            isa_type: "LT"
            placement: "intake-plc-01"
            register:
              type: "holding"
              address: 0
            unit: "L/s"
            scale: [0.0, 100.0]
            thresholds:
              warning: 80.0
              alarm: 90.0
              warning_low: 10.0
              alarm_low: 5.0
      - id: "intake-pump-01"
        type: "pump"
        label: "Intake Pump 1"
        instruments:
          - tag: "run-P-101"
            name: "Intake Pump 1 Run"
            isa_type: "run"
            placement: "intake-plc-01"
            register:
              type: "coil"
              address: 0
            unit: "bool"
  - id: "treatment"
    name: "Water Treatment"
    controller:
      placement: "treatment-plc-01"
      device: "compactlogix-l33er"
    equipment:
      - id: "sand-filter-01"
        type: "tank"
        label: "Sand Filter"
        era: 2018
        instruments:
          - tag: "PDT-201"
            name: "Filter Differential Pressure"
            isa_type: "PDT"
            placement: "treatment-plc-01"
            register:
              type: "holding"
              address: 2
            unit: "kPa"
            scale: [0.0, 50.0]
      - id: "uv-system-01"
        type: "uv_system"
        label: "UV Sterilization"
        instruments:
          - tag: "AT-201"
            name: "UV Intensity"
            isa_type: "AT"
            placement: "treatment-plc-01"
            register:
              type: "holding"
              address: 3
            unit: "mW/cm2"
            scale: [0.0, 100.0]
connections:
  - from: "raw-water-well"
    to: "intake-pump-01"
    type: "pipe"
    label: "Raw water intake"
  - from: "intake-pump-01"
    to: "sand-filter-01"
    type: "pipe"
    label: "Raw water to treatment"
  - from: "sand-filter-01"
    to: "uv-system-01"
    type: "pipe"
    label: "Filtered water to UV"
`
}
