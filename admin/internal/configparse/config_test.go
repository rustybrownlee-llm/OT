package configparse_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rustybrownlee/ot-simulator/admin/internal/configparse"
)

// writeTemp writes content to a temporary file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "monitor.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}

func TestParse_ValidConfig(t *testing.T) {
	yaml := `
environments:
  - name: test-env
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
        description: "test PLC"
`
	path := writeTemp(t, yaml)
	cfg, err := configparse.Parse(path)
	if err != nil {
		t.Fatalf("Parse returned error for valid config: %v", err)
	}
	if cfg.PollIntervalSeconds != configparse.DefaultPollIntervalSeconds {
		t.Errorf("poll_interval_seconds default: got %d, want %d",
			cfg.PollIntervalSeconds, configparse.DefaultPollIntervalSeconds)
	}
	if cfg.EventDBPath != configparse.DefaultEventDBPath {
		t.Errorf("event_db_path default: got %q, want %q",
			cfg.EventDBPath, configparse.DefaultEventDBPath)
	}
	if cfg.EventRetentionDays != configparse.DefaultEventRetentionDays {
		t.Errorf("event_retention_days default: got %d, want %d",
			cfg.EventRetentionDays, configparse.DefaultEventRetentionDays)
	}
}

func TestParse_NoEnvironments(t *testing.T) {
	yaml := `log_level: info`
	path := writeTemp(t, yaml)
	_, err := configparse.Parse(path)
	if err == nil {
		t.Fatal("Parse should fail for config with no environments")
	}
	if !strings.Contains(err.Error(), "environment") {
		t.Errorf("error should mention 'environment', got: %v", err)
	}
}

func TestParse_InvalidPollInterval(t *testing.T) {
	yaml := `
environments:
  - name: test-env
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
poll_interval_seconds: 0
`
	path := writeTemp(t, yaml)
	_, err := configparse.Parse(path)
	if err == nil {
		t.Fatal("Parse should fail for poll_interval_seconds=0")
	}
	if !strings.Contains(err.Error(), "poll_interval_seconds") {
		t.Errorf("error should mention 'poll_interval_seconds', got: %v", err)
	}
}

func TestParse_InvalidRetentionDays(t *testing.T) {
	yaml := `
environments:
  - name: test-env
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
event_retention_days: 0
`
	path := writeTemp(t, yaml)
	_, err := configparse.Parse(path)
	if err == nil {
		t.Fatal("Parse should fail for event_retention_days=0")
	}
	if !strings.Contains(err.Error(), "event_retention_days") {
		t.Errorf("error should mention 'event_retention_days', got: %v", err)
	}
}

func TestParse_SyslogEnabledNoTarget(t *testing.T) {
	yaml := `
environments:
  - name: test-env
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
syslog:
  enabled: true
`
	path := writeTemp(t, yaml)
	_, err := configparse.Parse(path)
	if err == nil {
		t.Fatal("Parse should fail for syslog enabled without target")
	}
	if !strings.Contains(err.Error(), "target") {
		t.Errorf("error should mention 'target', got: %v", err)
	}
}

func TestParse_SyslogURLTarget(t *testing.T) {
	yaml := `
environments:
  - name: test-env
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
syslog:
  enabled: true
  target: "udp://localhost:514"
`
	path := writeTemp(t, yaml)
	_, err := configparse.Parse(path)
	if err == nil {
		t.Fatal("Parse should fail for URL-style syslog target")
	}
	if !strings.Contains(err.Error(), "host:port") {
		t.Errorf("error should mention 'host:port', got: %v", err)
	}
}

func TestParse_SyslogBadProtocol(t *testing.T) {
	yaml := `
environments:
  - name: test-env
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
syslog:
  enabled: true
  target: "localhost:514"
  protocol: "tls"
`
	path := writeTemp(t, yaml)
	_, err := configparse.Parse(path)
	if err == nil {
		t.Fatal("Parse should fail for unknown syslog protocol")
	}
	if !strings.Contains(err.Error(), "protocol") {
		t.Errorf("error should mention 'protocol', got: %v", err)
	}
}

func TestParse_SyslogBadFacility(t *testing.T) {
	yaml := `
environments:
  - name: test-env
    address: "127.0.0.1"
    endpoints:
      - port: 5020
        unit_id: 1
syslog:
  enabled: true
  target: "localhost:514"
  facility: "not-a-facility"
`
	path := writeTemp(t, yaml)
	_, err := configparse.Parse(path)
	if err == nil {
		t.Fatal("Parse should fail for unrecognized syslog facility")
	}
	if !strings.Contains(err.Error(), "facility") {
		t.Errorf("error should mention 'facility', got: %v", err)
	}
}

func TestParse_GatewayMissingScanUnitIDs(t *testing.T) {
	yaml := `
environments:
  - name: test-env
    address: "127.0.0.1"
    endpoints:
      - port: 5030
        gateway: true
`
	path := writeTemp(t, yaml)
	_, err := configparse.Parse(path)
	if err == nil {
		t.Fatal("Parse should fail for gateway endpoint without scan_unit_ids")
	}
	if !strings.Contains(err.Error(), "scan_unit_ids") {
		t.Errorf("error should mention 'scan_unit_ids', got: %v", err)
	}
}

func TestParse_NonGatewayMissingUnitID(t *testing.T) {
	yaml := `
environments:
  - name: test-env
    address: "127.0.0.1"
    endpoints:
      - port: 5020
`
	path := writeTemp(t, yaml)
	_, err := configparse.Parse(path)
	if err == nil {
		t.Fatal("Parse should fail for non-gateway endpoint without unit_id")
	}
	if !strings.Contains(err.Error(), "unit_id") {
		t.Errorf("error should mention 'unit_id', got: %v", err)
	}
}

func TestParse_MissingFile(t *testing.T) {
	_, err := configparse.Parse("/nonexistent/path/monitor.yaml")
	if err == nil {
		t.Fatal("Parse should fail for missing file")
	}
}
