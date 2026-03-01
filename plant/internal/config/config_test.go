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
