package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// buildTestDesignRoot creates a minimal design root in a temp dir for environment testing.
// It returns the design root path.
func buildTestDesignRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, sub := range []string{"devices", "networks", "environments"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// writeDevice writes a device YAML into root/devices/<id>.yaml.
func writeDevice(t *testing.T, root, id, category, addressing string, maxH, maxC int) {
	t.Helper()
	content := fmt.Sprintf(`
schema_version: "0.1"
device:
  id: %q
  vendor: "TestVendor"
  model: "TestModel"
  category: %q
  vintage: 2020
  description: "test"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
registers:
  max_holding: %d
  max_coils: %d
  addressing: %q
  float_byte_order: "big-endian"
register_map_variants:
  default:
    holding:
      - { address: 0, name: "r1", unit: "%%", scale_min: 0, scale_max: 100, writable: false, description: "r1" }
`, id, category, maxH, maxC, addressing)
	writeTemp(t, filepath.Join(root, "devices"), id+".yaml", content)
}

// writeSerialDevice writes a device YAML with an rs485 port.
func writeSerialDevice(t *testing.T, root, id string) {
	t.Helper()
	content := fmt.Sprintf(`
schema_version: "0.1"
device:
  id: %q
  vendor: "TestVendor"
  model: "TestModel"
  category: "plc"
  vintage: 1990
  description: "serial test device"
connectivity:
  ports:
    - type: "rs485"
      protocols: ["modbus-rtu"]
  response_delay_ms: 50
registers:
  max_holding: 100
  max_coils: 100
  addressing: "one-based"
  float_byte_order: "big-endian"
register_map_variants:
  default:
    holding:
      - { address: 1, name: "r1", unit: "%%", scale_min: 0, scale_max: 100, writable: false, description: "r1" }
`, id)
	writeTemp(t, filepath.Join(root, "devices"), id+".yaml", content)
}

// writeGatewayDevice writes a gateway device YAML with both ethernet and rs485 ports.
func writeGatewayDevice(t *testing.T, root, id string) {
	t.Helper()
	content := fmt.Sprintf(`
schema_version: "0.1"
device:
  id: %q
  vendor: "Moxa"
  model: "NPort"
  category: "gateway"
  vintage: 2010
  description: "gateway test device"
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
    - type: "rs485"
      protocols: ["modbus-rtu"]
  response_delay_ms: 15
registers:
  max_holding: 5
  max_coils: 0
  addressing: "zero-based"
  float_byte_order: "big-endian"
register_map_variants:
  default:
    holding:
      - { address: 0, name: "status", unit: "enum", scale_min: 0, scale_max: 2, writable: false, description: "status" }
`, id)
	writeTemp(t, filepath.Join(root, "devices"), id+".yaml", content)
}

// writeEthernetNetwork writes an ethernet network YAML into root/networks/<id>.yaml.
func writeEthernetNetwork(t *testing.T, root, id, subnet string) {
	t.Helper()
	content := fmt.Sprintf(`
schema_version: "0.1"
network:
  id: %q
  name: "Test Network"
  type: "ethernet"
properties:
  subnet: %q
  managed_switch: false
  span_capable: false
`, id, subnet)
	writeTemp(t, filepath.Join(root, "networks"), id+".yaml", content)
}

// writeSerialNetwork writes a serial network YAML into root/networks/<id>.yaml.
func writeSerialNetwork(t *testing.T, root, id string) {
	t.Helper()
	content := fmt.Sprintf(`
schema_version: "0.1"
network:
  id: %q
  name: "Test Serial Bus"
  type: "serial-rs485"
properties:
  managed_switch: false
  span_capable: false
`, id)
	writeTemp(t, filepath.Join(root, "networks"), id+".yaml", content)
}

// writeEnvDir writes environment.yaml into root/environments/<envID>/ and returns the dir.
func writeEnvDir(t *testing.T, root, envID, content string) string {
	t.Helper()
	envDir := filepath.Join(root, "environments", envID)
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, envDir, "environment.yaml", content)
	return envDir
}

// validEnvContent builds a minimal valid environment YAML.
func validEnvContent(envID string) string {
	return fmt.Sprintf(`
schema_version: "0.1"
environment:
  id: %q
  name: "Test Environment"
  description: "test"
networks:
  - ref: "eth-net"
placements:
  - id: "plc-01"
    device: "test-plc"
    network: "eth-net"
    ip: "10.0.0.10"
    modbus_port: 5020
    role: "Test PLC"
    register_map_variant: "default"
`, envID)
}

// setupValidEnv creates a minimal valid design root with one ethernet placement.
func setupValidEnv(t *testing.T) (root string, envDir string) {
	t.Helper()
	root = buildTestDesignRoot(t)
	writeDevice(t, root, "test-plc", "plc", "zero-based", 100, 50)
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	envDir = writeEnvDir(t, root, "test-env", validEnvContent("test-env"))
	return root, envDir
}

func TestValidateEnvironment_Valid(t *testing.T) {
	_, envDir := setupValidEnv(t)
	result := ValidateEnvironment(envDir)
	if result.HasErrors() {
		t.Errorf("expected no errors for valid environment:\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV001_SchemaVersion(t *testing.T) {
	_, envDir := setupValidEnv(t)
	// Overwrite with bad schema version.
	writeTemp(t, envDir, "environment.yaml", `
schema_version: "0.2"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
placements: []
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-001") {
		t.Errorf("expected ENV-001\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV002_MissingID(t *testing.T) {
	_, envDir := setupValidEnv(t)
	writeTemp(t, envDir, "environment.yaml", `
schema_version: "0.1"
environment:
  name: "No ID"
networks: []
placements: []
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-002") {
		t.Errorf("expected ENV-002\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV003_MissingNetwork(t *testing.T) {
	_, envDir := setupValidEnv(t)
	writeTemp(t, envDir, "environment.yaml", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "nonexistent-net"
placements: []
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-003") {
		t.Errorf("expected ENV-003\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV004_MissingDevice(t *testing.T) {
	root := buildTestDesignRoot(t)
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
placements:
  - id: "p1"
    device: "nonexistent-device"
    network: "eth-net"
    ip: "10.0.0.10"
    modbus_port: 5020
    register_map_variant: "default"
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-004") {
		t.Errorf("expected ENV-004\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV005_MissingVariant(t *testing.T) {
	root := buildTestDesignRoot(t)
	writeDevice(t, root, "test-plc", "plc", "zero-based", 100, 50)
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
placements:
  - id: "p1"
    device: "test-plc"
    network: "eth-net"
    ip: "10.0.0.10"
    modbus_port: 5020
    register_map_variant: "nonexistent-variant"
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-005") {
		t.Errorf("expected ENV-005\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV006_DuplicatePlacementID(t *testing.T) {
	root := buildTestDesignRoot(t)
	writeDevice(t, root, "test-plc", "plc", "zero-based", 100, 50)
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
placements:
  - id: "dup-id"
    device: "test-plc"
    network: "eth-net"
    ip: "10.0.0.10"
    modbus_port: 5020
    register_map_variant: "default"
  - id: "dup-id"
    device: "test-plc"
    network: "eth-net"
    ip: "10.0.0.11"
    modbus_port: 5021
    register_map_variant: "default"
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-006") {
		t.Errorf("expected ENV-006\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV007_PlacementNetworkNotListed(t *testing.T) {
	root := buildTestDesignRoot(t)
	writeDevice(t, root, "test-plc", "plc", "zero-based", 100, 50)
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	writeEthernetNetwork(t, root, "other-net", "192.168.0.0/24")
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
placements:
  - id: "p1"
    device: "test-plc"
    network: "other-net"
    ip: "192.168.0.10"
    modbus_port: 5020
    register_map_variant: "default"
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-007") {
		t.Errorf("expected ENV-007\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV008_IPOutsideSubnet(t *testing.T) {
	root := buildTestDesignRoot(t)
	writeDevice(t, root, "test-plc", "plc", "zero-based", 100, 50)
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
placements:
  - id: "p1"
    device: "test-plc"
    network: "eth-net"
    ip: "192.168.1.10"
    modbus_port: 5020
    register_map_variant: "default"
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-008") {
		t.Errorf("expected ENV-008\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV009_DuplicateIP(t *testing.T) {
	root := buildTestDesignRoot(t)
	writeDevice(t, root, "test-plc", "plc", "zero-based", 100, 50)
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
placements:
  - id: "p1"
    device: "test-plc"
    network: "eth-net"
    ip: "10.0.0.10"
    modbus_port: 5020
    register_map_variant: "default"
  - id: "p2"
    device: "test-plc"
    network: "eth-net"
    ip: "10.0.0.10"
    modbus_port: 5021
    register_map_variant: "default"
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-009") {
		t.Errorf("expected ENV-009\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV010_PortCollision(t *testing.T) {
	root := buildTestDesignRoot(t)
	writeDevice(t, root, "test-plc", "plc", "zero-based", 100, 50)
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
placements:
  - id: "p1"
    device: "test-plc"
    network: "eth-net"
    ip: "10.0.0.10"
    modbus_port: 5020
    register_map_variant: "default"
  - id: "p2"
    device: "test-plc"
    network: "eth-net"
    ip: "10.0.0.11"
    modbus_port: 5020
    register_map_variant: "default"
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-010") {
		t.Errorf("expected ENV-010\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV011_MissingGateway(t *testing.T) {
	root := buildTestDesignRoot(t)
	writeSerialDevice(t, root, "serial-plc")
	writeSerialNetwork(t, root, "serial-bus")
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "serial-bus"
placements:
  - id: "plc-serial"
    device: "serial-plc"
    network: "serial-bus"
    serial_address: 1
    gateway: "nonexistent-gateway"
    register_map_variant: "default"
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-011") {
		t.Errorf("expected ENV-011\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV012_GatewayBridgeDirectionality(t *testing.T) {
	root := buildTestDesignRoot(t)
	writeGatewayDevice(t, root, "gw")
	writeSerialDevice(t, root, "serial-plc")
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	writeSerialNetwork(t, root, "serial-bus")
	// Bridge is reversed: from_network=serial-bus, to_network=eth-net (wrong direction).
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
  - ref: "serial-bus"
placements:
  - id: "gw-01"
    device: "gw"
    network: "eth-net"
    ip: "10.0.0.20"
    modbus_port: 5030
    register_map_variant: "default"
    bridges:
      - from_network: "serial-bus"
        to_network: "eth-net"
  - id: "serial-plc-01"
    device: "serial-plc"
    network: "serial-bus"
    serial_address: 1
    gateway: "gw-01"
    register_map_variant: "default"
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-012") {
		t.Errorf("expected ENV-012 for reversed bridge direction\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV013_DuplicateSerialAddress(t *testing.T) {
	root := buildTestDesignRoot(t)
	writeGatewayDevice(t, root, "gw")
	writeSerialDevice(t, root, "serial-plc")
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	writeSerialNetwork(t, root, "serial-bus")
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
  - ref: "serial-bus"
placements:
  - id: "gw-01"
    device: "gw"
    network: "eth-net"
    ip: "10.0.0.20"
    modbus_port: 5030
    register_map_variant: "default"
    bridges:
      - from_network: "eth-net"
        to_network: "serial-bus"
  - id: "plc-a"
    device: "serial-plc"
    network: "serial-bus"
    serial_address: 1
    gateway: "gw-01"
    register_map_variant: "default"
  - id: "plc-b"
    device: "serial-plc"
    network: "serial-bus"
    serial_address: 1
    gateway: "gw-01"
    register_map_variant: "default"
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-013") {
		t.Errorf("expected ENV-013 for duplicate serial address\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV017_SerialViaGatewayHasIP(t *testing.T) {
	root := buildTestDesignRoot(t)
	writeGatewayDevice(t, root, "gw")
	writeSerialDevice(t, root, "serial-plc")
	writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
	writeSerialNetwork(t, root, "serial-bus")
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
  - ref: "serial-bus"
placements:
  - id: "gw-01"
    device: "gw"
    network: "eth-net"
    ip: "10.0.0.20"
    modbus_port: 5030
    register_map_variant: "default"
    bridges:
      - from_network: "eth-net"
        to_network: "serial-bus"
  - id: "plc-serial"
    device: "serial-plc"
    network: "serial-bus"
    serial_address: 1
    gateway: "gw-01"
    ip: "10.0.0.99"
    register_map_variant: "default"
`)
	result := ValidateEnvironment(envDir)
	if !containsRule(result, "ENV-017") {
		t.Errorf("expected ENV-017 for serial device with ip field\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV018_DevicePortMismatch(t *testing.T) {
	root := buildTestDesignRoot(t)
	// Ethernet-only device placed on a serial network.
	writeDevice(t, root, "ethernet-plc", "plc", "zero-based", 100, 50)
	writeSerialNetwork(t, root, "serial-bus")
	envDir := writeEnvDir(t, root, "test-env", `
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "serial-bus"
placements:
  - id: "p1"
    device: "ethernet-plc"
    network: "serial-bus"
    serial_address: 1
    gateway: "fake-gw"
    register_map_variant: "default"
`)
	result := ValidateEnvironment(envDir)
	// ENV-011 (missing gateway) fires before ENV-018 in the second pass,
	// but ENV-018 check also fires.
	if !containsRule(result, "ENV-018") {
		t.Errorf("expected ENV-018 for device port type mismatch\n%s", result.String(envDir))
	}
}

func TestValidateEnvironment_ENV019_SerialAddressOutOfRange(t *testing.T) {
	tests := []struct {
		name    string
		addr    int
		wantErr bool
	}{
		{name: "valid min", addr: 1, wantErr: false},
		{name: "valid max", addr: 247, wantErr: false},
		{name: "broadcast", addr: 0, wantErr: true},
		{name: "reserved", addr: 248, wantErr: true},
		{name: "reserved max", addr: 255, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := buildTestDesignRoot(t)
			writeGatewayDevice(t, root, "gw")
			writeSerialDevice(t, root, "serial-plc")
			writeEthernetNetwork(t, root, "eth-net", "10.0.0.0/24")
			writeSerialNetwork(t, root, "serial-bus")
			envContent := fmt.Sprintf(`
schema_version: "0.1"
environment:
  id: "test-env"
networks:
  - ref: "eth-net"
  - ref: "serial-bus"
placements:
  - id: "gw-01"
    device: "gw"
    network: "eth-net"
    ip: "10.0.0.20"
    modbus_port: 5030
    register_map_variant: "default"
    bridges:
      - from_network: "eth-net"
        to_network: "serial-bus"
  - id: "plc-01"
    device: "serial-plc"
    network: "serial-bus"
    serial_address: %d
    gateway: "gw-01"
    register_map_variant: "default"
`, tc.addr)
			envDir := writeEnvDir(t, root, "test-env", envContent)
			result := ValidateEnvironment(envDir)
			gotErr := containsRule(result, "ENV-019")
			if gotErr != tc.wantErr {
				t.Errorf("addr=%d: wantErr=%v gotErr=%v\n%s", tc.addr, tc.wantErr, gotErr, result.String(envDir))
			}
		})
	}
}

// TestValidateEnvironment_Greenfield validates the real greenfield-water-mfg environment.
func TestValidateEnvironment_Greenfield(t *testing.T) {
	designRoot := findDesignRootForTests(t)
	envDir := filepath.Join(designRoot, "environments", "greenfield-water-mfg")
	result := ValidateEnvironment(envDir)
	if result.HasErrors() {
		t.Errorf("greenfield-water-mfg has validation errors:\n%s", result.String(envDir))
	}
}
