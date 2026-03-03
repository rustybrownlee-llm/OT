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

// TestSyslogDefaults verifies that syslog fields default to expected values
// when absent from the YAML.
func TestSyslogDefaults(t *testing.T) {
	tmp := writeMinimalConfig(t)

	cfg, err := config.Parse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Syslog.Enabled {
		t.Error("Syslog.Enabled: want false by default, got true")
	}
	if cfg.Syslog.Protocol != "udp" {
		t.Errorf("Syslog.Protocol: got %q, want %q", cfg.Syslog.Protocol, "udp")
	}
	if cfg.Syslog.Facility != "local0" {
		t.Errorf("Syslog.Facility: got %q, want %q", cfg.Syslog.Facility, "local0")
	}
	if cfg.Syslog.Format != "cef" {
		t.Errorf("Syslog.Format: got %q, want %q", cfg.Syslog.Format, "cef")
	}
}

// TestSyslogParsed verifies that explicitly-set syslog fields are parsed correctly.
func TestSyslogParsed(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "syslog.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
syslog:
  enabled: true
  target: "192.168.1.100:514"
  protocol: "tcp"
  facility: "local2"
  format: "cef"
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Syslog.Enabled {
		t.Error("Syslog.Enabled: want true")
	}
	if cfg.Syslog.Target != "192.168.1.100:514" {
		t.Errorf("Syslog.Target: got %q, want %q", cfg.Syslog.Target, "192.168.1.100:514")
	}
	if cfg.Syslog.Protocol != "tcp" {
		t.Errorf("Syslog.Protocol: got %q, want %q", cfg.Syslog.Protocol, "tcp")
	}
	if cfg.Syslog.Facility != "local2" {
		t.Errorf("Syslog.Facility: got %q, want %q", cfg.Syslog.Facility, "local2")
	}
	if cfg.Syslog.Format != "cef" {
		t.Errorf("Syslog.Format: got %q, want %q", cfg.Syslog.Format, "cef")
	}
}

// TestSyslogValidation_TargetRequired verifies that an empty target is rejected
// when syslog is enabled.
func TestSyslogValidation_TargetRequired(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "syslog-notarget.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
syslog:
  enabled: true
  protocol: "udp"
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Parse(tmp)
	if err == nil {
		t.Fatal("expected error for missing target when enabled, got nil")
	}
}

// TestSyslogValidation_RejectsURL verifies that a URL-style target ("://") is
// rejected with a descriptive error pointing to the correct host:port format.
func TestSyslogValidation_RejectsURL(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "syslog-url.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
syslog:
  enabled: true
  target: "udp://localhost:514"
  protocol: "udp"
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Parse(tmp)
	if err == nil {
		t.Fatal("expected error for URL-style target, got nil")
	}
}

// TestSyslogValidation_InvalidProtocol verifies that unsupported protocol values
// are rejected.
func TestSyslogValidation_InvalidProtocol(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "syslog-proto.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
syslog:
  enabled: true
  target: "localhost:514"
  protocol: "tls"
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Parse(tmp)
	if err == nil {
		t.Fatal("expected error for unsupported protocol, got nil")
	}
}

// TestSyslogValidation_InvalidFacility verifies that unrecognized facility names
// are rejected when syslog is enabled.
func TestSyslogValidation_InvalidFacility(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "syslog-facility.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
syslog:
  enabled: true
  target: "localhost:514"
  protocol: "udp"
  facility: "invalid_facility"
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Parse(tmp)
	if err == nil {
		t.Fatal("expected error for invalid facility name, got nil")
	}
}

// TestSyslogValidation_InvalidFormat verifies that unsupported format values
// are rejected.
func TestSyslogValidation_InvalidFormat(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "syslog-format.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
syslog:
  enabled: true
  target: "localhost:514"
  protocol: "udp"
  format: "json"
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Parse(tmp)
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}
}

// TestSyslogDisabled_NoValidation verifies that a disabled syslog block with an
// invalid target does not trigger validation errors (validation is skipped when
// disabled).
func TestSyslogDisabled_NoValidation(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "syslog-disabled.yaml")
	content := `environments:
  - name: "test"
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
syslog:
  enabled: false
  target: ""
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse(tmp)
	if err != nil {
		t.Fatalf("unexpected error for disabled syslog: %v", err)
	}
	if cfg.Syslog.Enabled {
		t.Error("Syslog.Enabled: want false")
	}
}

// writeMinimalConfig writes a minimal valid config YAML to a temp file and
// returns the path. Used by multiple test functions to avoid repetition.
func writeMinimalConfig(t *testing.T) string {
	t.Helper()
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
	return tmp
}
