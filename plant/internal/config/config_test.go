package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rustybrownlee/ot-simulator/plant/internal/config"
)

func TestLoad_PlantYAML(t *testing.T) {
	// Criterion 11: plant.yaml loads and validates without error.
	// Path is relative to this file's location in the source tree.
	// During testing, working directory is the package directory.
	// Walk up to find the config file.
	cfgPath := findFile(t, "plant.yaml")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%q) error: %v", cfgPath, err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if cfg.Plant.Name == "" {
		t.Error("expected Plant.Name to be non-empty")
	}
}

func TestLoad_DeviceProfileYAML(t *testing.T) {
	// Criterion 12: device profile YAML loads without error (YAML parse check).
	profilePath := findFile(t, "compactlogix-l33er.yaml")

	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error: %v", profilePath, err)
	}
	if len(data) == 0 {
		t.Fatal("device profile file is empty")
	}
}

func TestDefaultConfig_Validate(t *testing.T) {
	cfg := config.DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() error: %v", err)
	}
}

func TestValidate_EmptyName_Fails(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Plant.Name = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty Plant.Name, got nil")
	}
}

func TestValidate_InvalidLogLevel_Fails(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Logging.Level = "verbose"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid log level, got nil")
	}
}

func TestValidate_InvalidPort_Fails(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WaterTreatment.HMI.Port = 80 // privileged port
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for privileged HMI port, got nil")
	}
}

// findFile walks up the directory tree from the test binary working directory
// to find a file by name within the config/ or config/devices/ directories.
func findFile(t *testing.T, name string) string {
	t.Helper()

	// Test runs from the package directory; config is at ../../config/
	candidates := []string{
		filepath.Join("..", "..", "config", name),
		filepath.Join("..", "..", "config", "devices", name),
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	t.Fatalf("could not locate config file %q in search paths", name)
	return ""
}
