package config_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rustybrownlee/ot-simulator/plant/internal/config"
)

// designRoot returns the absolute path to the design/ directory.
// runtime.Caller(0) returns the compile-time source path of this file:
//   .../OT/plant/internal/config/config_test.go
// design/ is at .../OT/design/ -- three levels above this file's directory.
func designRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile dir: .../OT/plant/internal/config/
	// up 1:         .../OT/plant/internal/
	// up 2:         .../OT/plant/
	// up 3:         .../OT/           (project root)
	// + "design":   .../OT/design/
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "design")
	return filepath.Clean(root)
}

// TestLoadEnvironment_Greenfield validates success criterion 11:
// the greenfield-water-mfg environment loads, resolves all refs, and validates.
func TestLoadEnvironment_Greenfield(t *testing.T) {
	envDir := filepath.Join(designRoot(t), "environments", "greenfield-water-mfg")

	resolved, err := config.LoadEnvironment(envDir)
	if err != nil {
		t.Fatalf("LoadEnvironment(%q) error: %v", envDir, err)
	}

	wantName := "Greenfield Water and Manufacturing Facility"
	if resolved.Env.Environment.Name != wantName {
		t.Errorf("environment name = %q, want %q", resolved.Env.Environment.Name, wantName)
	}

	if len(resolved.Env.Placements) == 0 {
		t.Error("expected at least one placement, got zero")
	}

	if len(resolved.Networks) == 0 {
		t.Error("expected at least one resolved network, got zero")
	}

	if len(resolved.Devices) == 0 {
		t.Error("expected at least one resolved device, got zero")
	}
}

// TestLoadDevice_CompactLogix validates success criterion 9:
// the compactlogix-l33er device atom loads and parses without error.
func TestLoadDevice_CompactLogix(t *testing.T) {
	path := filepath.Join(designRoot(t), "devices", "compactlogix-l33er.yaml")

	dev, err := config.LoadDevice(path)
	if err != nil {
		t.Fatalf("LoadDevice(%q) error: %v", path, err)
	}

	if dev.Device.ID != "compactlogix-l33er" {
		t.Errorf("device.id = %q, want %q", dev.Device.ID, "compactlogix-l33er")
	}

	if dev.SchemaVersion != "0.1" {
		t.Errorf("schema_version = %q, want %q", dev.SchemaVersion, "0.1")
	}

	if _, ok := dev.RegisterMapVariants["water-intake"]; !ok {
		t.Error("expected register_map_variants to include water-intake")
	}

	if _, ok := dev.RegisterMapVariants["water-distribution"]; !ok {
		t.Error("expected register_map_variants to include water-distribution")
	}
}

// TestLoadNetwork_WTLevel1 validates success criterion 10 (partial):
// a single network atom loads and parses without error.
func TestLoadNetwork_WTLevel1(t *testing.T) {
	path := filepath.Join(designRoot(t), "networks", "wt-level1.yaml")

	net, err := config.LoadNetwork(path)
	if err != nil {
		t.Fatalf("LoadNetwork(%q) error: %v", path, err)
	}

	if net.Network.ID != "wt-level1" {
		t.Errorf("network.id = %q, want %q", net.Network.ID, "wt-level1")
	}

	if net.SchemaVersion != "0.1" {
		t.Errorf("schema_version = %q, want %q", net.SchemaVersion, "0.1")
	}
}

// TestLoadNetwork_AllSixAtoms validates success criterion 10:
// all 6 network atom YAML files load and parse without error.
func TestLoadNetwork_AllSixAtoms(t *testing.T) {
	networkIDs := []string{
		"wt-level1",
		"wt-level2",
		"wt-level3",
		"mfg-flat",
		"mfg-serial-bus",
		"cross-plant",
	}

	for _, id := range networkIDs {
		id := id
		t.Run(id, func(t *testing.T) {
			path := filepath.Join(designRoot(t), "networks", id+".yaml")
			net, err := config.LoadNetwork(path)
			if err != nil {
				t.Fatalf("LoadNetwork(%q) error: %v", path, err)
			}
			if net.Network.ID != id {
				t.Errorf("network.id = %q, want %q", net.Network.ID, id)
			}
		})
	}
}

// TestDefaultEnvironment_Validates checks that DefaultEnvironment produces
// a valid ResolvedEnvironment without loading any files.
func TestDefaultEnvironment_Validates(t *testing.T) {
	resolved := config.DefaultEnvironment()
	if err := resolved.Validate(); err != nil {
		t.Fatalf("DefaultEnvironment().Validate() error: %v", err)
	}
}

// TestValidate_EmptyName_Fails checks that a missing environment name is rejected.
func TestValidate_EmptyName_Fails(t *testing.T) {
	resolved := config.DefaultEnvironment()
	resolved.Env.Environment.Name = ""
	if err := resolved.Validate(); err == nil {
		t.Error("expected error for empty environment name, got nil")
	}
}

// TestValidate_UnknownDevice_Fails checks that a placement referencing an
// unknown device ID fails validation.
func TestValidate_UnknownDevice_Fails(t *testing.T) {
	resolved := config.DefaultEnvironment()
	resolved.Env.Placements[0].Device = "nonexistent-plc"
	if err := resolved.Validate(); err == nil {
		t.Error("expected error for unknown device ref, got nil")
	}
}

// TestValidate_UnknownNetwork_Fails checks that a placement referencing an
// unknown network ID fails validation.
func TestValidate_UnknownNetwork_Fails(t *testing.T) {
	resolved := config.DefaultEnvironment()
	resolved.Env.Placements[0].Network = "nonexistent-network"
	if err := resolved.Validate(); err == nil {
		t.Error("expected error for unknown network ref, got nil")
	}
}

// TestValidate_DuplicatePlacementID_Fails checks that two placements sharing
// the same ID fail validation.
func TestValidate_DuplicatePlacementID_Fails(t *testing.T) {
	resolved := config.DefaultEnvironment()
	dup := resolved.Env.Placements[0]
	resolved.Env.Placements = append(resolved.Env.Placements, dup)
	if err := resolved.Validate(); err == nil {
		t.Error("expected error for duplicate placement id, got nil")
	}
}

// TestValidate_InvalidPort_Fails checks that a Modbus port outside the
// non-privileged range (1024-65535) fails validation.
func TestValidate_InvalidPort_Fails(t *testing.T) {
	resolved := config.DefaultEnvironment()
	resolved.Env.Placements[0].ModbusPort = 502 // privileged port
	if err := resolved.Validate(); err == nil {
		t.Error("expected error for privileged modbus_port, got nil")
	}
}

// TestLoadDevice_MoxaNPort5150 verifies the completed Moxa NPort 5150 device atom:
// loads without error, has the correct ID and schema version, and contains the
// serial-gateway variant with exactly 9 holding registers and 0 coils.
func TestLoadDevice_MoxaNPort5150(t *testing.T) {
	path := filepath.Join(designRoot(t), "devices", "moxa-nport-5150.yaml")

	dev, err := config.LoadDevice(path)
	if err != nil {
		t.Fatalf("LoadDevice(%q) error: %v", path, err)
	}

	if dev.Device.ID != "moxa-nport-5150" {
		t.Errorf("device.id = %q, want %q", dev.Device.ID, "moxa-nport-5150")
	}

	if dev.SchemaVersion != "0.1" {
		t.Errorf("schema_version = %q, want %q", dev.SchemaVersion, "0.1")
	}

	variant, ok := dev.RegisterMapVariants["serial-gateway"]
	if !ok {
		t.Fatal("expected register_map_variants to include serial-gateway")
	}

	if got := len(variant.Holding); got != 9 {
		t.Errorf("serial-gateway holding register count = %d, want 9", got)
	}

	if got := len(variant.Coils); got != 0 {
		t.Errorf("serial-gateway coil count = %d, want 0", got)
	}
}

// TestLoadDevice_SLC500 verifies the completed SLC-500-05 device atom:
// loads without error, has the correct ID, contains mfg-line-a variant with
// 7 holding registers, and has DH-485 in its connectivity ports.
func TestLoadDevice_SLC500(t *testing.T) {
	path := filepath.Join(designRoot(t), "devices", "slc-500-05.yaml")

	dev, err := config.LoadDevice(path)
	if err != nil {
		t.Fatalf("LoadDevice(%q) error: %v", path, err)
	}

	if dev.Device.ID != "slc-500-05" {
		t.Errorf("device.id = %q, want %q", dev.Device.ID, "slc-500-05")
	}

	variant, ok := dev.RegisterMapVariants["mfg-line-a"]
	if !ok {
		t.Fatal("expected register_map_variants to include mfg-line-a")
	}

	if got := len(variant.Holding); got != 7 {
		t.Errorf("mfg-line-a holding register count = %d, want 7", got)
	}

	hasDH485 := false
	for _, port := range dev.Connectivity.Ports {
		if port.Type == "dh485" {
			hasDH485 = true
			break
		}
	}
	if !hasDH485 {
		t.Error("expected connectivity.ports to include a dh485 port")
	}
}

// TestLoadDevice_Modicon984 verifies the completed Modicon 984 device atom:
// loads without error, has the correct ID, contains mfg-cooling variant with
// 7 holding registers, and has max_holding set to 999.
func TestLoadDevice_Modicon984(t *testing.T) {
	path := filepath.Join(designRoot(t), "devices", "modicon-984.yaml")

	dev, err := config.LoadDevice(path)
	if err != nil {
		t.Fatalf("LoadDevice(%q) error: %v", path, err)
	}

	if dev.Device.ID != "modicon-984" {
		t.Errorf("device.id = %q, want %q", dev.Device.ID, "modicon-984")
	}

	variant, ok := dev.RegisterMapVariants["mfg-cooling"]
	if !ok {
		t.Fatal("expected register_map_variants to include mfg-cooling")
	}

	if got := len(variant.Holding); got != 7 {
		t.Errorf("mfg-cooling holding register count = %d, want 7", got)
	}

	if dev.Registers.MaxHolding != 999 {
		t.Errorf("registers.max_holding = %d, want 999", dev.Registers.MaxHolding)
	}
}

// TestLoadDevice_CompactLogix_WaterTreatment verifies that the CompactLogix device atom
// now contains three register map variants: water-intake, water-distribution, and
// the new water-treatment variant.
func TestLoadDevice_CompactLogix_WaterTreatment(t *testing.T) {
	path := filepath.Join(designRoot(t), "devices", "compactlogix-l33er.yaml")

	dev, err := config.LoadDevice(path)
	if err != nil {
		t.Fatalf("LoadDevice(%q) error: %v", path, err)
	}

	wantVariants := []string{"water-intake", "water-distribution", "water-treatment"}
	for _, name := range wantVariants {
		if _, ok := dev.RegisterMapVariants[name]; !ok {
			t.Errorf("expected register_map_variants to include %q", name)
		}
	}

	if got := len(dev.RegisterMapVariants); got != 4 {
		t.Errorf("register_map_variants count = %d, want 4", got)
	}
}

// TestLoadDevice_CompactLogix_NoRS485 verifies that the CompactLogix device atom
// has no RS-485 port in its connectivity (Ethernet-native device).
func TestLoadDevice_CompactLogix_NoRS485(t *testing.T) {
	path := filepath.Join(designRoot(t), "devices", "compactlogix-l33er.yaml")

	dev, err := config.LoadDevice(path)
	if err != nil {
		t.Fatalf("LoadDevice(%q) error: %v", path, err)
	}

	for _, port := range dev.Connectivity.Ports {
		if port.Type == "rs485" {
			t.Errorf("found rs485 port in CompactLogix connectivity -- device is Ethernet-native and has no RS-485 port")
		}
	}
}

// TestValidate_UnknownRegisterMapVariant_Fails checks that a placement specifying
// a register_map_variant not defined in the device fails validation.
func TestValidate_UnknownRegisterMapVariant_Fails(t *testing.T) {
	resolved := config.DefaultEnvironment()
	resolved.Env.Placements[0].RegisterMapVariant = "nonexistent-variant"
	if err := resolved.Validate(); err == nil {
		t.Error("expected error for unknown register_map_variant, got nil")
	}
}
