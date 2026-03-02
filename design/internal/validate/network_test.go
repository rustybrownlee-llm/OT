package validate

import (
	"path/filepath"
	"testing"
)

const validEthernetNetworkYAML = `
schema_version: "0.1"
network:
  id: "test-net"
  name: "Test Network"
  type: "ethernet"
  description: "A test network"
properties:
  subnet: "10.10.10.0/24"
  vlan: 10
  managed_switch: true
  span_capable: true
`

const validSerialNetworkYAML = `
schema_version: "0.1"
network:
  id: "test-serial"
  name: "Test Serial Bus"
  type: "serial-rs485"
  description: "A serial bus"
properties:
  managed_switch: false
  span_capable: false
`

func writeNetworkYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	return writeTemp(t, dir, "network.yaml", content)
}

func TestValidateNetwork_ValidEthernet(t *testing.T) {
	path := writeNetworkYAML(t, validEthernetNetworkYAML)
	result := ValidateNetwork(path)
	if result.HasErrors() {
		t.Errorf("expected no errors for valid ethernet network:\n%s", result.String(path))
	}
}

func TestValidateNetwork_ValidSerial(t *testing.T) {
	path := writeNetworkYAML(t, validSerialNetworkYAML)
	result := ValidateNetwork(path)
	if result.HasErrors() {
		t.Errorf("expected no errors for valid serial network:\n%s", result.String(path))
	}
}

func TestValidateNetwork_NET001_SchemaVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "valid", version: `"0.1"`, wantErr: false},
		{name: "missing", version: `""`, wantErr: true},
		{name: "wrong version", version: `"1.0"`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := `
schema_version: ` + tc.version + `
network:
  id: "x"
  name: "X"
  type: "ethernet"
properties:
  subnet: "10.0.0.0/24"
`
			path := writeNetworkYAML(t, yaml)
			result := ValidateNetwork(path)
			gotErr := containsRule(result, "NET-001")
			if gotErr != tc.wantErr {
				t.Errorf("NET-001: wantErr=%v gotErr=%v\n%s", tc.wantErr, gotErr, result.String(path))
			}
		})
	}
}

func TestValidateNetwork_NET002_MissingID(t *testing.T) {
	yaml := `
schema_version: "0.1"
network:
  name: "X"
  type: "ethernet"
properties:
  subnet: "10.0.0.0/24"
`
	path := writeNetworkYAML(t, yaml)
	result := ValidateNetwork(path)
	if !containsRule(result, "NET-002") {
		t.Errorf("expected NET-002 for missing id\n%s", result.String(path))
	}
}

func TestValidateNetwork_NET003_InvalidType(t *testing.T) {
	tests := []struct {
		netType string
		wantErr bool
	}{
		{"ethernet", false},
		{"serial-rs485", false},
		{"serial-rs232", false},
		{"serial-rs422", false},
		{"canbus", true},
		{"", true},
	}
	for _, tc := range tests {
		t.Run(tc.netType, func(t *testing.T) {
			yaml := buildNetworkWithType(tc.netType)
			path := writeNetworkYAML(t, yaml)
			result := ValidateNetwork(path)
			gotErr := containsRule(result, "NET-003")
			if gotErr != tc.wantErr {
				t.Errorf("type=%q: wantErr=%v gotErr=%v", tc.netType, tc.wantErr, gotErr)
			}
		})
	}
}

func TestValidateNetwork_NET004_SubnetRequired(t *testing.T) {
	tests := []struct {
		name    string
		subnet  string
		wantErr bool
	}{
		{name: "valid CIDR", subnet: "10.10.10.0/24", wantErr: false},
		{name: "missing subnet", subnet: "", wantErr: true},
		{name: "invalid CIDR", subnet: "not-a-cidr", wantErr: true},
		{name: "host address not CIDR", subnet: "10.10.10.1", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := `
schema_version: "0.1"
network:
  id: "x"
  name: "X"
  type: "ethernet"
properties:
  subnet: "` + tc.subnet + `"
`
			path := writeNetworkYAML(t, yaml)
			result := ValidateNetwork(path)
			gotErr := containsRule(result, "NET-004")
			if gotErr != tc.wantErr {
				t.Errorf("subnet=%q: wantErr=%v gotErr=%v\n%s", tc.subnet, tc.wantErr, gotErr, result.String(path))
			}
		})
	}
}

func TestValidateNetwork_NET005_SerialNoSubnet(t *testing.T) {
	yaml := `
schema_version: "0.1"
network:
  id: "x"
  name: "X"
  type: "serial-rs485"
properties:
  subnet: "10.0.0.0/24"
  managed_switch: false
`
	path := writeNetworkYAML(t, yaml)
	result := ValidateNetwork(path)
	if !containsRule(result, "NET-005") {
		t.Errorf("expected NET-005 for serial network with subnet\n%s", result.String(path))
	}
}

// TestValidateNetwork_ExistingNetworks validates all 6 existing network atoms.
func TestValidateNetwork_ExistingNetworks(t *testing.T) {
	designRoot := findDesignRootForTests(t)
	networks := []string{
		"wt-level1.yaml",
		"wt-level2.yaml",
		"wt-level3.yaml",
		"mfg-flat.yaml",
		"mfg-serial-bus.yaml",
		"cross-plant.yaml",
	}
	for _, name := range networks {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(designRoot, "networks", name)
			result := ValidateNetwork(path)
			if result.HasErrors() {
				t.Errorf("existing network %s has validation errors:\n%s", name, result.String(path))
			}
		})
	}
}

// --- Helpers ---

func buildNetworkWithType(netType string) string {
	subnet := ""
	if netType == "ethernet" {
		subnet = "\n  subnet: \"10.0.0.0/24\""
	}
	return `
schema_version: "0.1"
network:
  id: "x"
  name: "X"
  type: "` + netType + `"
properties:` + subnet + `
  managed_switch: false
`
}
