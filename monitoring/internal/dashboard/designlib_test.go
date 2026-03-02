package dashboard_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/dashboard"
)

const testDataDir = "testdata"

func TestLoadDesignLibrary_ValidFixtures(t *testing.T) {
	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: unexpected error: %v", err)
	}
	// 1 device: test-plc
	if len(lib.Devices) != 1 {
		t.Errorf("Devices: got %d, want 1", len(lib.Devices))
	}
	// 3 networks: test-net, test-level3, test-flat-net (added by SOW-019.0 for topology tests)
	if len(lib.Networks) != 3 {
		t.Errorf("Networks: got %d, want 3", len(lib.Networks))
	}
	// 3 environments: test-env, test-hybrid, test-flat (added by SOW-019.0 for topology tests)
	if len(lib.Environments) != 3 {
		t.Errorf("Environments: got %d, want 3", len(lib.Environments))
	}
}

func TestLoadDesignLibrary_MissingDir(t *testing.T) {
	lib, err := dashboard.LoadDesignLibrary("/nonexistent/path/xyz")
	if err != nil {
		t.Fatalf("LoadDesignLibrary missing dir: unexpected error: %v", err)
	}
	if !lib.IsEmpty() {
		t.Error("expected empty library for missing dir")
	}
}

func TestLoadDesignLibrary_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	devDir := filepath.Join(dir, "devices")
	netDir := filepath.Join(dir, "networks")
	envDir := filepath.Join(dir, "environments")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(netDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a malformed YAML device file.
	badPath := filepath.Join(devDir, "bad.yaml")
	if err := os.WriteFile(badPath, []byte("key:\n\t- invalid tab indentation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a valid device file.
	goodPath := filepath.Join(devDir, "good.yaml")
	goodContent := `
schema_version: "0.1"
device:
  id: "good-device"
  vendor: "GoodCorp"
  model: "Good 100"
  category: "plc"
registers:
  max_holding: 10
  max_coils: 5
  addressing: "zero-based"
`
	if err := os.WriteFile(goodPath, []byte(goodContent), 0o644); err != nil {
		t.Fatal(err)
	}

	lib, err := dashboard.LoadDesignLibrary(dir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary with bad YAML: unexpected error: %v", err)
	}
	// The malformed file should be skipped; the good file should be loaded.
	if len(lib.Devices) != 1 {
		t.Errorf("Devices: got %d, want 1 (bad file should be skipped)", len(lib.Devices))
	}
	if _, ok := lib.Devices["good-device"]; !ok {
		t.Error("good-device should be in library")
	}
}

func TestDeviceAtomParsing(t *testing.T) {
	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}
	atom, ok := lib.Devices["test-plc"]
	if !ok {
		t.Fatal("test-plc not found in library")
	}
	if atom.Device.Vendor != "TestCorp" {
		t.Errorf("Vendor: got %q, want %q", atom.Device.Vendor, "TestCorp")
	}
	if atom.Device.Category != "plc" {
		t.Errorf("Category: got %q, want %q", atom.Device.Category, "plc")
	}
	if atom.Registers.MaxHolding != 100 {
		t.Errorf("MaxHolding: got %d, want 100", atom.Registers.MaxHolding)
	}
	if atom.Registers.Addressing != "zero-based" {
		t.Errorf("Addressing: got %q, want %q", atom.Registers.Addressing, "zero-based")
	}
	variant, ok := atom.Variants["default"]
	if !ok {
		t.Fatal("default variant not found")
	}
	if len(variant.Holding) != 2 {
		t.Errorf("Holding registers: got %d, want 2", len(variant.Holding))
	}
	if variant.Holding[1].Writable != true {
		t.Error("pump_speed register should be writable")
	}
}

func TestNetworkAtomParsing(t *testing.T) {
	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}
	atom, ok := lib.Networks["test-net"]
	if !ok {
		t.Fatal("test-net not found in library")
	}
	if atom.Network.Type != "ethernet" {
		t.Errorf("Type: got %q, want %q", atom.Network.Type, "ethernet")
	}
	if atom.Properties.Subnet != "192.168.99.0/24" {
		t.Errorf("Subnet: got %q, want %q", atom.Properties.Subnet, "192.168.99.0/24")
	}
	if !atom.Properties.ManagedSwitch {
		t.Error("ManagedSwitch should be true")
	}
}

func TestEnvironmentParsing(t *testing.T) {
	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}
	env, ok := lib.Environments["test-env"]
	if !ok {
		t.Fatal("test-env not found in library")
	}
	if env.Env.Name != "Test Environment" {
		t.Errorf("Name: got %q, want %q", env.Env.Name, "Test Environment")
	}
	if len(env.Placements) != 1 {
		t.Fatalf("Placements: got %d, want 1", len(env.Placements))
	}
	p := env.Placements[0]
	if p.ID != "test-plc-01" {
		t.Errorf("Placement ID: got %q, want %q", p.ID, "test-plc-01")
	}
	if p.Device != "test-plc" {
		t.Errorf("Placement Device: got %q, want %q", p.Device, "test-plc")
	}
	if p.RegisterMapVariant != "default" {
		t.Errorf("RegisterMapVariant: got %q, want %q", p.RegisterMapVariant, "default")
	}
}

func TestResolveDeviceAtomID_Found(t *testing.T) {
	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}
	// config_description format: "test-plc-01 (some role)"
	got := lib.ResolveDeviceAtomID("test-plc-01 (Test PLC 1)")
	if got != "test-plc" {
		t.Errorf("ResolveDeviceAtomID: got %q, want %q", got, "test-plc")
	}
}

func TestResolveDeviceAtomID_NotFound(t *testing.T) {
	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}
	got := lib.ResolveDeviceAtomID("unknown-plc-99 (some role)")
	if got != "" {
		t.Errorf("ResolveDeviceAtomID for unknown: got %q, want empty string", got)
	}
}

func TestRawYAMLStored(t *testing.T) {
	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}
	if lib.RawYAML["devices/test-plc"] == "" {
		t.Error("RawYAML for devices/test-plc should not be empty")
	}
	if lib.RawYAML["networks/test-net"] == "" {
		t.Error("RawYAML for networks/test-net should not be empty")
	}
	if lib.RawYAML["environments/test-env"] == "" {
		t.Error("RawYAML for environments/test-env should not be empty")
	}
}

// TestEnvironmentParsing_SOW016Fields verifies that archetype, era_span, installed,
// and boundaries fields are parsed correctly from environment YAML (SOW-016.0 additions).
func TestEnvironmentParsing_SOW016Fields(t *testing.T) {
	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}
	env, ok := lib.Environments["test-hybrid"]
	if !ok {
		t.Fatal("test-hybrid environment not found in library")
	}
	if env.Env.Archetype != "hybrid" {
		t.Errorf("Archetype: got %q, want %q", env.Env.Archetype, "hybrid")
	}
	if env.Env.EraSpan != "1997-2022" {
		t.Errorf("EraSpan: got %q, want %q", env.Env.EraSpan, "1997-2022")
	}
	if len(env.Boundaries) != 1 {
		t.Fatalf("Boundaries: got %d, want 1", len(env.Boundaries))
	}
	b := env.Boundaries[0]
	if b.State != "enforced" {
		t.Errorf("Boundary state: got %q, want %q", b.State, "enforced")
	}
	if b.Infrastructure != "managed-switch" {
		t.Errorf("Boundary infrastructure: got %q, want %q", b.Infrastructure, "managed-switch")
	}
	if b.Installed == nil || *b.Installed != 2018 {
		t.Errorf("Boundary installed: want 2018, got %v", b.Installed)
	}
	if len(b.Between) != 2 {
		t.Fatalf("Boundary.Between: got %d, want 2", len(b.Between))
	}
	if b.Between[0] != "test-level3" || b.Between[1] != "test-flat-net" {
		t.Errorf("Boundary.Between: got %v, want [test-level3, test-flat-net]", b.Between)
	}
}

// TestPlacementParsing_InstalledField verifies that installed year is parsed from
// placement YAML (SOW-016.0 addition).
func TestPlacementParsing_InstalledField(t *testing.T) {
	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}
	env, ok := lib.Environments["test-hybrid"]
	if !ok {
		t.Fatal("test-hybrid not found")
	}
	var found *dashboard.Placement
	for i := range env.Placements {
		if env.Placements[i].ID == "hybrid-plc-02" {
			found = &env.Placements[i]
			break
		}
	}
	if found == nil {
		t.Fatal("hybrid-plc-02 placement not found")
	}
	if found.Installed == nil {
		t.Fatal("hybrid-plc-02: Installed should not be nil")
	}
	if *found.Installed != 1997 {
		t.Errorf("hybrid-plc-02 Installed: got %d, want 1997", *found.Installed)
	}
}

func TestScaleValue(t *testing.T) {
	// 0 -> scaleMin
	if got := dashboard.ScaleValue(0, 0, 100); got != 0.0 {
		t.Errorf("ScaleValue(0, 0, 100): got %f, want 0.0", got)
	}
	// 65535 -> scaleMax
	got := dashboard.ScaleValue(65535, 0, 100)
	if got < 99.99 || got > 100.01 {
		t.Errorf("ScaleValue(65535, 0, 100): got %f, want ~100.0", got)
	}
	// Midpoint approximately 50
	mid := dashboard.ScaleValue(32767, 0, 100)
	if mid < 49.0 || mid > 51.0 {
		t.Errorf("ScaleValue(32767, 0, 100): got %f, want ~50.0", mid)
	}
}
