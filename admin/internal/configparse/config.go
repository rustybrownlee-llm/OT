// Package configparse parses and validates the monitoring module configuration
// for the admin CLI. The Config struct and validation rules mirror
// monitoring/internal/config/config.go exactly.
//
// PROTOTYPE-DEBT: [td-admin-090] Config struct and validation logic duplicated
// from monitoring/internal/config/. Changes to monitor.yaml schema must be
// applied in both places. Future: extract shared config types to a common
// library package, or generate both from a schema definition.
package configparse

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the monitoring module configuration.
// YAML tags must match monitoring/internal/config/config.go exactly.
type Config struct {
	LogLevel              string        `yaml:"log_level"`
	PollIntervalSeconds   int           `yaml:"poll_interval_seconds"`
	GatewayRequestDelayMs int           `yaml:"gateway_request_delay_ms"`
	APIAddr               string        `yaml:"api_addr"`
	DashboardAddr         string        `yaml:"dashboard_addr"`
	Environments          []Environment `yaml:"environments"`

	BaselineLearningCycles int `yaml:"baseline_learning_cycles"`
	RingBufferSize         int `yaml:"ring_buffer_size"`
	MaxAlerts              int `yaml:"max_alerts"`

	EventDBPath        string `yaml:"event_db_path"`
	EventRetentionDays int    `yaml:"event_retention_days"`

	Syslog SyslogConfig `yaml:"syslog"`
}

// SyslogConfig holds syslog forwarding configuration.
type SyslogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Target   string `yaml:"target"`
	Protocol string `yaml:"protocol"`
	Facility string `yaml:"facility"`
	Format   string `yaml:"format"`
}

// Environment groups Modbus endpoints behind a single IP address.
type Environment struct {
	Name      string     `yaml:"name"`
	Address   string     `yaml:"address"`
	Endpoints []Endpoint `yaml:"endpoints"`
}

// Endpoint describes one Modbus TCP port and how to poll it.
type Endpoint struct {
	Port        int     `yaml:"port"`
	UnitID      uint8   `yaml:"unit_id"`
	Gateway     bool    `yaml:"gateway"`
	ScanUnitIDs []uint8 `yaml:"scan_unit_ids"`
	Description string  `yaml:"description"`
}

// rawConfig captures raw YAML values before defaults are applied, allowing
// validation to distinguish "not set" (nil) from "explicitly zero".
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
	Syslog                 SyslogConfig  `yaml:"syslog"`
}

// Defaults applied when config fields are zero-valued or absent from YAML.
const (
	DefaultPollIntervalSeconds   = 2
	DefaultGatewayRequestDelayMs = 10
	DefaultAPIAddr               = ":8091"
	DefaultDashboardAddr         = ":8090"
	DefaultLogLevel              = "info"
	MinPollIntervalSeconds       = 1

	DefaultBaselineLearningCycles = 150
	DefaultRingBufferSize         = 300
	DefaultMaxAlerts              = 1000

	DefaultEventDBPath        = "data/events.db"
	DefaultEventRetentionDays = 7
	MinEventRetentionDays     = 1

	DefaultSyslogProtocol = "udp"
	DefaultSyslogFacility = "local0"
	DefaultSyslogFormat   = "cef"
)

// ParseContent validates the given YAML string as a monitoring configuration.
// The string is not written to disk; this is used by the web API editor endpoint.
// Returns a descriptive error on parse or validation failure, nil on success.
func ParseContent(content string) error {
	var raw rawConfig
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	cfg := fromRaw(raw)
	return cfg.Validate()
}

// Parse reads the YAML config file at path and returns a validated Config
// with defaults applied. Returns a descriptive error on parse or validation failure.
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

// ParseLenient reads and applies defaults without running full validation.
// Used by "config view" to display configs that may have errors.
func ParseLenient(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	return fromRaw(raw), nil
}

// fromRaw converts a rawConfig to a Config, applying defaults.
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
		Syslog:                 raw.Syslog,
	}

	if raw.PollIntervalSeconds != nil {
		cfg.PollIntervalSeconds = *raw.PollIntervalSeconds
	}
	if raw.EventRetentionDays != nil {
		cfg.EventRetentionDays = *raw.EventRetentionDays
	}

	applyDefaults(cfg, raw)
	return cfg
}

// applyDefaults fills zero-valued fields with their default values.
func applyDefaults(cfg *Config, raw rawConfig) {
	if raw.PollIntervalSeconds == nil {
		cfg.PollIntervalSeconds = DefaultPollIntervalSeconds
	}
	if cfg.GatewayRequestDelayMs == 0 {
		cfg.GatewayRequestDelayMs = DefaultGatewayRequestDelayMs
	}
	if cfg.APIAddr == "" {
		cfg.APIAddr = DefaultAPIAddr
	}
	if cfg.DashboardAddr == "" {
		cfg.DashboardAddr = DefaultDashboardAddr
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = DefaultLogLevel
	}
	if cfg.BaselineLearningCycles <= 0 {
		cfg.BaselineLearningCycles = DefaultBaselineLearningCycles
	}
	if cfg.RingBufferSize <= 0 {
		cfg.RingBufferSize = DefaultRingBufferSize
	}
	if cfg.MaxAlerts <= 0 {
		cfg.MaxAlerts = DefaultMaxAlerts
	}
	if cfg.EventDBPath == "" {
		cfg.EventDBPath = DefaultEventDBPath
	}
	if raw.EventRetentionDays == nil {
		cfg.EventRetentionDays = DefaultEventRetentionDays
	}
	if cfg.Syslog.Protocol == "" {
		cfg.Syslog.Protocol = DefaultSyslogProtocol
	}
	if cfg.Syslog.Facility == "" {
		cfg.Syslog.Facility = DefaultSyslogFacility
	}
	if cfg.Syslog.Format == "" {
		cfg.Syslog.Format = DefaultSyslogFormat
	}
}

// Validate checks required fields and value ranges.
// Returns a descriptive error including field name and expected value.
func (cfg *Config) Validate() error {
	if len(cfg.Environments) == 0 {
		return fmt.Errorf("at least one environment must be defined")
	}

	if cfg.PollIntervalSeconds < MinPollIntervalSeconds {
		return fmt.Errorf("poll_interval_seconds must be >= %d, got %d",
			MinPollIntervalSeconds, cfg.PollIntervalSeconds)
	}

	if cfg.EventRetentionDays < MinEventRetentionDays {
		return fmt.Errorf("event_retention_days must be >= %d, got %d",
			MinEventRetentionDays, cfg.EventRetentionDays)
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

	if err := validateSyslog(cfg.Syslog); err != nil {
		return fmt.Errorf("syslog: %w", err)
	}

	return nil
}

// validateEndpoints checks each endpoint in an environment is well-formed.
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

// validateSyslog checks syslog configuration fields.
func validateSyslog(s SyslogConfig) error {
	if !s.Enabled {
		if s.Format != "" && s.Format != DefaultSyslogFormat {
			return fmt.Errorf("format %q is not supported; only %q is accepted in Beta 0.6",
				s.Format, DefaultSyslogFormat)
		}
		return nil
	}

	if s.Target == "" {
		return fmt.Errorf("target is required when enabled=true")
	}

	if containsURLScheme(s.Target) {
		return fmt.Errorf("target must be host:port format, not a URL (e.g., 'localhost:514' not 'udp://localhost:514')")
	}

	if s.Protocol != "udp" && s.Protocol != "tcp" {
		return fmt.Errorf("protocol must be %q or %q, got %q", "udp", "tcp", s.Protocol)
	}

	if s.Format != DefaultSyslogFormat {
		return fmt.Errorf("format %q is not supported; only %q is accepted in Beta 0.6",
			s.Format, DefaultSyslogFormat)
	}

	if !isKnownFacility(s.Facility) {
		return fmt.Errorf("facility %q is not a recognized syslog facility name", s.Facility)
	}

	return nil
}

// knownFacilities lists recognized syslog facility names.
var knownFacilities = map[string]struct{}{
	"kern": {}, "user": {}, "mail": {}, "daemon": {}, "auth": {},
	"syslog": {}, "lpr": {}, "news": {}, "uucp": {}, "cron": {},
	"local0": {}, "local1": {}, "local2": {}, "local3": {},
	"local4": {}, "local5": {}, "local6": {}, "local7": {},
}

// isKnownFacility reports whether name is a recognized syslog facility.
func isKnownFacility(name string) bool {
	_, ok := knownFacilities[name]
	return ok
}

// containsURLScheme reports whether s contains a URL scheme separator "://".
func containsURLScheme(s string) bool {
	for i := 0; i+2 < len(s); i++ {
		if s[i] == ':' && s[i+1] == '/' && s[i+2] == '/' {
			return true
		}
	}
	return false
}
