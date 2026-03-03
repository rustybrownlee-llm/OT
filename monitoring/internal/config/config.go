// Package config parses and validates the monitoring module configuration.
// The config tells the monitor WHERE to look (IP addresses and ports) but
// NOT WHAT to expect (register maps, device types, normal values). Device
// characteristics are discovered by network probing, not from config.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the monitoring module configuration.
type Config struct {
	LogLevel              string        `yaml:"log_level"`
	PollIntervalSeconds   int           `yaml:"poll_interval_seconds"`
	GatewayRequestDelayMs int           `yaml:"gateway_request_delay_ms"` // [OT-REVIEW] RS-485 inter-frame gap
	APIAddr               string        `yaml:"api_addr"`
	DashboardAddr         string        `yaml:"dashboard_addr"`
	Environments          []Environment `yaml:"environments"`

	// Baseline and alert configuration (SOW-013.0).
	BaselineLearningCycles int `yaml:"baseline_learning_cycles"` // default: 150
	RingBufferSize         int `yaml:"ring_buffer_size"`         // default: 300
	MaxAlerts              int `yaml:"max_alerts"`               // default: 1000

	// Event store configuration (SOW-027.0).
	EventDBPath        string `yaml:"event_db_path"`        // default: "data/events.db"
	EventRetentionDays int    `yaml:"event_retention_days"` // default: 7, minimum: 1
}

// Environment groups a set of Modbus endpoints behind a single IP address.
// The address is the Docker network IP of the plant container serving all
// ports in this environment.
type Environment struct {
	Name      string     `yaml:"name"`
	Address   string     `yaml:"address"`
	Endpoints []Endpoint `yaml:"endpoints"`
}

// Endpoint describes a single Modbus TCP port and how to poll it.
// Non-gateway endpoints have a single UnitID. Gateway endpoints have
// Gateway set to true and a ScanUnitIDs list for downstream serial devices.
type Endpoint struct {
	Port        int     `yaml:"port"`
	UnitID      uint8   `yaml:"unit_id"`
	Gateway     bool    `yaml:"gateway"`
	ScanUnitIDs []uint8 `yaml:"scan_unit_ids"`
	Description string  `yaml:"description"`
}

// rawConfig is an intermediate type used during parsing to capture the raw
// poll_interval_seconds value before defaults are applied, enabling validation
// to distinguish "not set" (0 -> use default) from "explicitly set to 0" (error).
type rawConfig struct {
	LogLevel               string        `yaml:"log_level"`
	PollIntervalSeconds    *int          `yaml:"poll_interval_seconds"`
	GatewayRequestDelayMs  int           `yaml:"gateway_request_delay_ms"`
	APIAddr                string        `yaml:"api_addr"`
	DashboardAddr          string        `yaml:"dashboard_addr"`
	Environments           []Environment `yaml:"environments"`
	BaselineLearningCycles int           `yaml:"baseline_learning_cycles"`
	RingBufferSize         int           `yaml:"ring_buffer_size"`
	MaxAlerts              int           `yaml:"max_alerts"`
	EventDBPath            string        `yaml:"event_db_path"`
	EventRetentionDays     *int          `yaml:"event_retention_days"`
}

// defaults applied when config fields are zero-valued.
const (
	defaultPollIntervalSeconds   = 2
	defaultGatewayRequestDelayMs = 10
	defaultAPIAddr               = ":8091"
	defaultDashboardAddr         = ":8090"
	defaultLogLevel              = "info"
	minPollIntervalSeconds       = 1

	// Baseline defaults (SOW-013.0).
	defaultBaselineLearningCycles = 150
	defaultRingBufferSize         = 300
	defaultMaxAlerts              = 1000

	// Event store defaults (SOW-027.0).
	defaultEventDBPath        = "data/events.db"
	defaultEventRetentionDays = 7
	minEventRetentionDays     = 1
)

// PROTOTYPE-DEBT: [td-config-028] monitor.yaml duplicates endpoint knowledge that also
// exists in the design layer YAML. The monitor cannot read design YAML (ADR-005 D4),
// so this duplication is architecturally intentional but creates a maintenance burden.
// Future: CLI tool that generates monitor.yaml from environment definitions.

// Parse reads the YAML config file at path and returns a validated Config.
// Parse errors and validation errors both produce a descriptive error.
func Parse(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	cfg := fromRaw(raw)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %w", path, err)
	}

	return cfg, nil
}

// fromRaw converts a rawConfig to a Config, applying defaults and validating
// that explicitly-provided values are within acceptable ranges.
func fromRaw(raw rawConfig) *Config {
	cfg := &Config{
		LogLevel:               raw.LogLevel,
		GatewayRequestDelayMs:  raw.GatewayRequestDelayMs,
		APIAddr:                raw.APIAddr,
		DashboardAddr:          raw.DashboardAddr,
		Environments:           raw.Environments,
		BaselineLearningCycles: raw.BaselineLearningCycles,
		RingBufferSize:         raw.RingBufferSize,
		MaxAlerts:              raw.MaxAlerts,
		EventDBPath:            raw.EventDBPath,
	}

	// Preserve explicitly-set values; do NOT apply defaults yet.
	// Validation checks these values; defaults are applied after validation
	// only for fields that were absent from the YAML (nil pointer).
	if raw.PollIntervalSeconds != nil {
		cfg.PollIntervalSeconds = *raw.PollIntervalSeconds
	}
	if raw.EventRetentionDays != nil {
		cfg.EventRetentionDays = *raw.EventRetentionDays
	}

	applyDefaults(cfg, raw)
	return cfg
}

// applyDefaults fills in zero-valued fields with their default values.
// Only applies defaults to fields that were absent from the source YAML.
func applyDefaults(cfg *Config, raw rawConfig) {
	if raw.PollIntervalSeconds == nil {
		cfg.PollIntervalSeconds = defaultPollIntervalSeconds
	}
	if cfg.GatewayRequestDelayMs == 0 {
		cfg.GatewayRequestDelayMs = defaultGatewayRequestDelayMs
	}
	if cfg.APIAddr == "" {
		cfg.APIAddr = defaultAPIAddr
	}
	if cfg.DashboardAddr == "" {
		cfg.DashboardAddr = defaultDashboardAddr
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = defaultLogLevel
	}
	if cfg.BaselineLearningCycles <= 0 {
		cfg.BaselineLearningCycles = defaultBaselineLearningCycles
	}
	if cfg.RingBufferSize <= 0 {
		cfg.RingBufferSize = defaultRingBufferSize
	}
	if cfg.MaxAlerts <= 0 {
		cfg.MaxAlerts = defaultMaxAlerts
	}
	if cfg.EventDBPath == "" {
		cfg.EventDBPath = defaultEventDBPath
	}
	if raw.EventRetentionDays == nil {
		cfg.EventRetentionDays = defaultEventRetentionDays
	}
}

// Validate checks that required fields are present and values are in range.
// Returns a descriptive error if any constraint is violated.
func (cfg *Config) Validate() error {
	if len(cfg.Environments) == 0 {
		return fmt.Errorf("at least one environment must be defined")
	}

	if cfg.PollIntervalSeconds < minPollIntervalSeconds {
		return fmt.Errorf("poll_interval_seconds must be >= %d, got %d",
			minPollIntervalSeconds, cfg.PollIntervalSeconds)
	}

	if cfg.EventRetentionDays < minEventRetentionDays {
		return fmt.Errorf("event_retention_days must be >= %d, got %d",
			minEventRetentionDays, cfg.EventRetentionDays)
	}

	for i, env := range cfg.Environments {
		if env.Name == "" {
			return fmt.Errorf("environment[%d]: name is required", i)
		}
		if env.Address == "" {
			return fmt.Errorf("environment %q: address is required", env.Name)
		}
		if err := validateEndpoints(env); err != nil {
			return fmt.Errorf("environment %q: %w", env.Name, err)
		}
	}

	return nil
}

// validateEndpoints checks that each endpoint in an environment is well-formed.
func validateEndpoints(env Environment) error {
	for i, ep := range env.Endpoints {
		if ep.Port < 1 || ep.Port > 65535 {
			return fmt.Errorf("endpoint[%d]: port %d out of range [1, 65535]", i, ep.Port)
		}
		if ep.Gateway && len(ep.ScanUnitIDs) == 0 {
			return fmt.Errorf("endpoint[%d] (port %d): gateway=true requires scan_unit_ids",
				i, ep.Port)
		}
		if !ep.Gateway && ep.UnitID == 0 {
			return fmt.Errorf("endpoint[%d] (port %d): non-gateway endpoint requires unit_id",
				i, ep.Port)
		}
	}
	return nil
}
