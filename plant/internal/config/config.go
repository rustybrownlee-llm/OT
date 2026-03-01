// Package config loads and validates the plant topology YAML configuration.
// It defines typed structs for all configuration sections and provides
// DefaultConfig() for testing and Load() for production use.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level plant configuration structure. It maps directly
// to the plant.yaml file structure.
type Config struct {
	Plant          PlantMeta          `yaml:"plant"`
	Logging        LoggingConfig      `yaml:"logging"`
	WaterTreatment WaterTreatmentConfig `yaml:"water_treatment"`
	Manufacturing  ManufacturingConfig  `yaml:"manufacturing"`
	CrossPlant     CrossPlantConfig     `yaml:"cross_plant"`
}

// PlantMeta holds facility identification metadata.
type PlantMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// LoggingConfig controls structured log output.
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug | info | warn | error
	Format string `yaml:"format"` // json
}

// WaterTreatmentConfig defines the Purdue-model water treatment plant.
type WaterTreatmentConfig struct {
	Enabled  bool              `yaml:"enabled"`
	Networks WTNetworkConfig   `yaml:"networks"`
	HMI      ListenConfig      `yaml:"hmi"`
	Devices  []DeviceConfig    `yaml:"devices"`
}

// WTNetworkConfig defines the three Purdue model network levels.
type WTNetworkConfig struct {
	Level3 NetworkSegment `yaml:"level3"`
	Level2 NetworkSegment `yaml:"level2"`
	Level1 NetworkSegment `yaml:"level1"`
}

// NetworkSegment describes a single network segment.
type NetworkSegment struct {
	Name        string `yaml:"name"`
	Subnet      string `yaml:"subnet"`
	VLAN        int    `yaml:"vlan"`
	Description string `yaml:"description"`
}

// ManufacturingConfig defines the legacy flat-network manufacturing floor.
type ManufacturingConfig struct {
	Enabled bool              `yaml:"enabled"`
	Network NetworkSegment    `yaml:"network"`
	HMI     ListenConfig      `yaml:"hmi"`
	Devices []DeviceConfig    `yaml:"devices"`
}

// CrossPlantConfig defines the connection between the two facility networks.
type CrossPlantConfig struct {
	Enabled                  bool           `yaml:"enabled"`
	Network                  NetworkSegment `yaml:"network"`
	WaterTreatmentEndpoint   string         `yaml:"water_treatment_endpoint"`
	ManufacturingEndpoint    string         `yaml:"manufacturing_endpoint"`
}

// ListenConfig specifies a network listen address.
type ListenConfig struct {
	Addr string `yaml:"addr"`
	Port int    `yaml:"port"`
}

// DeviceConfig defines a single virtual device within the plant topology.
type DeviceConfig struct {
	ID            string `yaml:"id"`
	Name          string `yaml:"name"`
	Profile       string `yaml:"profile"`
	Network       string `yaml:"network,omitempty"`
	IP            string `yaml:"ip"`
	ModbusPort    int    `yaml:"modbus_port,omitempty"`
	SerialAddress int    `yaml:"serial_address,omitempty"`
	Gateway       string `yaml:"gateway,omitempty"`
	Description   string `yaml:"description"`
}

// Load reads and parses the YAML configuration file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config YAML: %w", err)
	}

	return &cfg, nil
}

// DefaultConfig returns a minimal valid configuration suitable for testing.
func DefaultConfig() *Config {
	return &Config{
		Plant: PlantMeta{
			Name:        "OT Simulator (default)",
			Description: "Default configuration for testing",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		WaterTreatment: WaterTreatmentConfig{
			Enabled: true,
			HMI:     ListenConfig{Addr: "0.0.0.0", Port: 8080},
		},
		Manufacturing: ManufacturingConfig{
			Enabled: true,
			HMI:     ListenConfig{Addr: "0.0.0.0", Port: 8081},
		},
	}
}

// Validate checks that required configuration fields are present and valid.
func (c *Config) Validate() error {
	if c.Plant.Name == "" {
		return fmt.Errorf("plant.name must not be empty")
	}
	if err := validateLogging(c.Logging); err != nil {
		return err
	}
	if err := validateHMIPorts(c); err != nil {
		return err
	}
	return nil
}

// validateLogging checks that the log level is one of the accepted values.
func validateLogging(l LoggingConfig) error {
	valid := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if l.Level != "" && !valid[l.Level] {
		return fmt.Errorf("logging.level %q is not valid (use debug|info|warn|error)", l.Level)
	}
	return nil
}

// validateHMIPorts checks that HMI port assignments are in the allowed range.
func validateHMIPorts(c *Config) error {
	if c.WaterTreatment.Enabled {
		if err := validatePort("water_treatment.hmi.port", c.WaterTreatment.HMI.Port); err != nil {
			return err
		}
	}
	if c.Manufacturing.Enabled {
		if err := validatePort("manufacturing.hmi.port", c.Manufacturing.HMI.Port); err != nil {
			return err
		}
	}
	return nil
}

// validatePort checks that port is within the non-privileged range.
func validatePort(field string, port int) error {
	if port == 0 {
		return nil // unset port is allowed (assigned by subsystem default)
	}
	if port < 1024 || port > 65535 {
		return fmt.Errorf("%s %d is outside the non-privileged port range (1024-65535)", field, port)
	}
	return nil
}
