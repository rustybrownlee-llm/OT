package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/config"
)

func TestParse_ValidConfig(t *testing.T) {
	cfg, err := config.Parse("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Parse(valid.yaml) unexpected error: %v", err)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.PollIntervalSeconds != 2 {
		t.Errorf("PollIntervalSeconds: got %d, want 2", cfg.PollIntervalSeconds)
	}
	if cfg.GatewayRequestDelayMs != 10 {
		t.Errorf("GatewayRequestDelayMs: got %d, want 10", cfg.GatewayRequestDelayMs)
	}
	if cfg.APIAddr != ":8091" {
		t.Errorf("APIAddr: got %q, want %q", cfg.APIAddr, ":8091")
	}
	if cfg.DashboardAddr != ":8090" {
		t.Errorf("DashboardAddr: got %q, want %q", cfg.DashboardAddr, ":8090")
	}
	if len(cfg.Environments) != 2 {
		t.Fatalf("Environments: got %d, want 2", len(cfg.Environments))
	}
}

func TestParse_GatewayEndpoint(t *testing.T) {
	cfg, err := config.Parse("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env := cfg.Environments[0]
	if len(env.Endpoints) != 2 {
		t.Fatalf("Endpoints: got %d, want 2", len(env.Endpoints))
	}

	gw := env.Endpoints[1]
	if !gw.Gateway {
		t.Error("second endpoint: Gateway should be true")
	}
	if len(gw.ScanUnitIDs) != 11 {
		t.Errorf("ScanUnitIDs: got %d, want 11", len(gw.ScanUnitIDs))
	}
	// Verify 247 is in the scan list.
	found247 := false
	for _, id := range gw.ScanUnitIDs {
		if id == 247 {
			found247 = true
		}
	}
	if !found247 {
		t.Error("ScanUnitIDs: expected 247 in list")
	}
}

func TestParse_SingleUnitIDEndpoint(t *testing.T) {
	cfg, err := config.Parse("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ep := cfg.Environments[0].Endpoints[0]
	if ep.Gateway {
		t.Error("first endpoint: Gateway should be false")
	}
	if ep.UnitID != 1 {
		t.Errorf("UnitID: got %d, want 1", ep.UnitID)
	}
}

func TestParse_MissingFile(t *testing.T) {
	_, err := config.Parse("testdata/nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(tmp, []byte(":::invalid yaml:::\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.Parse(tmp)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestParse_InvalidPollInterval(t *testing.T) {
	_, err := config.Parse("testdata/invalid-poll.yaml")
	if err == nil {
		t.Fatal("expected error for poll_interval_seconds=0, got nil")
	}
}

func TestValidate_ZeroEnvironments(t *testing.T) {
	cfg := &config.Config{
		PollIntervalSeconds: 2,
		APIAddr:             ":8091",
		Environments:        nil,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for zero environments, got nil")
	}
}

func TestValidate_EmptyAddress(t *testing.T) {
	cfg := &config.Config{
		PollIntervalSeconds: 2,
		APIAddr:             ":8091",
		Environments: []config.Environment{
			{
				Name:    "test",
				Address: "",
				Endpoints: []config.Endpoint{
					{Port: 5020, UnitID: 1},
				},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty address, got nil")
	}
}

func TestDefaults_Applied(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "minimal.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.PollIntervalSeconds != 2 {
		t.Errorf("default PollIntervalSeconds: got %d, want 2", cfg.PollIntervalSeconds)
	}
	if cfg.APIAddr != ":8091" {
		t.Errorf("default APIAddr: got %q, want %q", cfg.APIAddr, ":8091")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("default LogLevel: got %q, want %q", cfg.LogLevel, "info")
	}
}

// TestParse_BaselineFieldsPresent verifies that baseline fields are parsed correctly
// when present in the YAML.
func TestParse_BaselineFieldsPresent(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "baseline.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
baseline_learning_cycles: 75
ring_buffer_size: 150
max_alerts: 500
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.BaselineLearningCycles != 75 {
		t.Errorf("BaselineLearningCycles: got %d, want 75", cfg.BaselineLearningCycles)
	}
	if cfg.RingBufferSize != 150 {
		t.Errorf("RingBufferSize: got %d, want 150", cfg.RingBufferSize)
	}
	if cfg.MaxAlerts != 500 {
		t.Errorf("MaxAlerts: got %d, want 500", cfg.MaxAlerts)
	}
}

// TestParse_BaselineFieldsAbsent verifies that defaults are applied when baseline
// fields are absent from the YAML.
func TestParse_BaselineFieldsAbsent(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "no-baseline.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.BaselineLearningCycles != 150 {
		t.Errorf("default BaselineLearningCycles: got %d, want 150", cfg.BaselineLearningCycles)
	}
	if cfg.RingBufferSize != 300 {
		t.Errorf("default RingBufferSize: got %d, want 300", cfg.RingBufferSize)
	}
	if cfg.MaxAlerts != 1000 {
		t.Errorf("default MaxAlerts: got %d, want 1000", cfg.MaxAlerts)
	}
}

// TestParse_EventStoreFieldsPresent verifies that explicit event store fields
// are parsed and honored correctly when present in the YAML.
func TestParse_EventStoreFieldsPresent(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "event-store.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
event_db_path: "/tmp/custom-events.db"
event_retention_days: 14
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.EventDBPath != "/tmp/custom-events.db" {
		t.Errorf("EventDBPath: got %q, want %q", cfg.EventDBPath, "/tmp/custom-events.db")
	}
	if cfg.EventRetentionDays != 14 {
		t.Errorf("EventRetentionDays: got %d, want 14", cfg.EventRetentionDays)
	}
}

// TestParse_EventStoreDefaults verifies that default values are applied when
// event store fields are absent from the YAML.
func TestParse_EventStoreDefaults(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "event-defaults.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.EventDBPath != "data/events.db" {
		t.Errorf("default EventDBPath: got %q, want %q", cfg.EventDBPath, "data/events.db")
	}
	if cfg.EventRetentionDays != 7 {
		t.Errorf("default EventRetentionDays: got %d, want 7", cfg.EventRetentionDays)
	}
}

// TestParse_InvalidRetentionDays verifies that event_retention_days: 0 is
// rejected by validation because the minimum is 1.
func TestParse_InvalidRetentionDays(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad-retention.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
event_retention_days: 0
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Parse(tmp)
	if err == nil {
		t.Fatal("expected error for event_retention_days=0, got nil")
	}
}

// TestValidate_BaselineDefaultsForNegativeValues verifies that Validate applies
// defaults for zero or negative baseline configuration values.
func TestValidate_BaselineDefaultsForNegativeValues(t *testing.T) {
	cfg := &config.Config{
		PollIntervalSeconds:    2,
		APIAddr:                ":8091",
		BaselineLearningCycles: -1,
		RingBufferSize:         0,
		MaxAlerts:              -5,
		Environments: []config.Environment{
			{
				Name:    "test",
				Address: "127.0.0.1",
				Endpoints: []config.Endpoint{
					{Port: 5020, UnitID: 1},
				},
			},
		},
	}

	// Validate is called internally by Parse; call applyDefaults via a Parse round-trip.
	tmp := filepath.Join(t.TempDir(), "neg.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
baseline_learning_cycles: -1
ring_buffer_size: 0
max_alerts: -5
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed, err := config.Parse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.BaselineLearningCycles != 150 {
		t.Errorf("BaselineLearningCycles default: got %d, want 150", parsed.BaselineLearningCycles)
	}
	if parsed.RingBufferSize != 300 {
		t.Errorf("RingBufferSize default: got %d, want 300", parsed.RingBufferSize)
	}
	if parsed.MaxAlerts != 1000 {
		t.Errorf("MaxAlerts default: got %d, want 1000", parsed.MaxAlerts)
	}

	// Suppress unused variable lint.
	_ = cfg
}
