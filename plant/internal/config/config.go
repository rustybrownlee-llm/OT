// Package config loads and validates design layer YAML artifacts for the plant binary.
// It defines typed structs for environments, devices, and networks, and provides
// LoadEnvironment() for production use and DefaultEnvironment() for testing.
//
// Loading strategy: LoadEnvironment accepts a path to an environment directory
// (e.g., "../../design/environments/greenfield-water-mfg"). It reads environment.yaml
// from that directory, walks up to locate the design/ root, then resolves all device
// and network references from design/devices/ and design/networks/ respectively.
//
// PROTOTYPE-DEBT: [td-config-001] LoadEnvironment uses relative path walking to locate
// the design/ root. This works for local development and Docker but is fragile if the
// binary is invoked from an unexpected working directory.
// TODO-FUTURE: Accept an explicit --design-root flag or derive from binary location.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Environment is the top-level structure of an environment.yaml file.
// It references networks and placements by ID; resolution is done by LoadEnvironment.
type Environment struct {
	SchemaVersion string       `yaml:"schema_version"`
	Environment   EnvMeta      `yaml:"environment"`
	Networks      []NetworkRef `yaml:"networks"`
	Placements    []Placement  `yaml:"placements"`
}

// EnvMeta holds environment identification metadata.
type EnvMeta struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// NetworkRef is an entry in the environment's network list.
type NetworkRef struct {
	Ref string `yaml:"ref"`
}

// Placement assigns a device to a network with specific addressing.
type Placement struct {
	ID                 string             `yaml:"id"`
	Device             string             `yaml:"device"`
	Network            string             `yaml:"network"`
	IP                 string             `yaml:"ip,omitempty"`
	ModbusPort         int                `yaml:"modbus_port,omitempty"`
	SerialAddress      int                `yaml:"serial_address,omitempty"`
	Gateway            string             `yaml:"gateway,omitempty"`
	Role               string             `yaml:"role,omitempty"`
	RegisterMapVariant string             `yaml:"register_map_variant,omitempty"`
	AdditionalNetworks []AdditionalNet    `yaml:"additional_networks,omitempty"`
	Bridges            []BridgeSpec       `yaml:"bridges,omitempty"`
}

// AdditionalNet declares a secondary network attachment for a multi-homed device.
type AdditionalNet struct {
	Network string `yaml:"network"`
	IP      string `yaml:"ip,omitempty"`
}

// BridgeSpec declares a gateway bridge between two networks.
type BridgeSpec struct {
	FromNetwork string `yaml:"from_network"`
	ToNetwork   string `yaml:"to_network"`
}

// Device is the top-level structure of a device atom YAML file (design/devices/).
type Device struct {
	SchemaVersion       string                     `yaml:"schema_version"`
	Device              DeviceMeta                 `yaml:"device"`
	Connectivity        Connectivity               `yaml:"connectivity"`
	Registers           RegisterCapacity           `yaml:"registers"`
	RegisterMapVariants map[string]RegisterMap     `yaml:"register_map_variants"`
	Diagnostics         Diagnostics                `yaml:"diagnostics"`
}

// DeviceMeta holds device identification metadata.
type DeviceMeta struct {
	ID          string `yaml:"id"`
	Vendor      string `yaml:"vendor"`
	Model       string `yaml:"model"`
	Category    string `yaml:"category"`
	Vintage     int    `yaml:"vintage"`
	Description string `yaml:"description"`
}

// Connectivity describes how the device communicates on the network.
type Connectivity struct {
	Ports                []PortSpec `yaml:"ports"`
	ResponseDelayMS      int        `yaml:"response_delay_ms"`
	ResponseJitterMS     int        `yaml:"response_jitter_ms"`
	ConcurrentConns      int        `yaml:"concurrent_connections"`
}

// PortSpec describes one physical or logical port on a device.
type PortSpec struct {
	Type      string   `yaml:"type"`
	Protocols []string `yaml:"protocols"`
}

// RegisterCapacity defines the maximum register address space this device supports.
type RegisterCapacity struct {
	MaxHolding         int    `yaml:"max_holding"`
	MaxCoils           int    `yaml:"max_coils"`
	MaxInputRegisters  int    `yaml:"max_input_registers"`
	MaxDiscreteInputs  int    `yaml:"max_discrete_inputs"`
	Addressing         string `yaml:"addressing"`
	FloatByteOrder     string `yaml:"float_byte_order"`
	MaxRegistersPerRead int   `yaml:"max_registers_per_read"`
}

// RegisterMap is a named collection of holding register and coil definitions.
type RegisterMap struct {
	Holding []RegisterDef `yaml:"holding"`
	Coils   []CoilDef     `yaml:"coils"`
}

// RegisterDef describes a single holding register entry in a register map variant.
type RegisterDef struct {
	Address     int     `yaml:"address"`
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Unit        string  `yaml:"unit"`
	ScaleMin    float64 `yaml:"scale_min"`
	ScaleMax    float64 `yaml:"scale_max"`
	Writable    bool    `yaml:"writable"`
}

// CoilDef describes a single coil entry in a register map variant.
type CoilDef struct {
	Address     int    `yaml:"address"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Writable    bool   `yaml:"writable"`
}

// Diagnostics describes the device's built-in diagnostic and management capabilities.
type Diagnostics struct {
	WebServer      bool `yaml:"web_server"`
	WebPort        int  `yaml:"web_port"`
	SNMP           bool `yaml:"snmp"`
	Syslog         bool `yaml:"syslog"`
	FaultLog       bool `yaml:"fault_log"`
	FaultLogDepth  int  `yaml:"fault_log_depth"`
}

// Network is the top-level structure of a network atom YAML file (design/networks/).
type Network struct {
	SchemaVersion string      `yaml:"schema_version"`
	Network       NetworkMeta `yaml:"network"`
	Properties    NetworkProps `yaml:"properties"`
}

// NetworkMeta holds network identification metadata.
type NetworkMeta struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

// NetworkProps describes the communication medium's operational properties.
type NetworkProps struct {
	Subnet        string `yaml:"subnet,omitempty"`
	VLAN          int    `yaml:"vlan,omitempty"`
	ManagedSwitch bool   `yaml:"managed_switch"`
	SPANCapable   bool   `yaml:"span_capable"`
}

// ResolvedEnvironment is the fully loaded result of LoadEnvironment.
// All device and network references have been resolved from the design layer.
type ResolvedEnvironment struct {
	Env        Environment
	Devices    map[string]*Device  // keyed by device atom ID (e.g., "compactlogix-l33er")
	Networks   map[string]*Network // keyed by network atom ID (e.g., "wt-level1")
}

// LoadEnvironment reads an environment directory, resolves all device and network
// references from the design layer, and returns a validated ResolvedEnvironment.
// envDir must be the path to a directory containing environment.yaml.
func LoadEnvironment(envDir string) (*ResolvedEnvironment, error) {
	absEnvDir, err := filepath.Abs(envDir)
	if err != nil {
		return nil, fmt.Errorf("resolve environment path %q: %w", envDir, err)
	}

	env, err := loadEnvironmentFile(absEnvDir)
	if err != nil {
		return nil, err
	}

	designRoot, err := findDesignRoot(absEnvDir)
	if err != nil {
		return nil, err
	}

	resolved, err := resolveRefs(env, designRoot)
	if err != nil {
		return nil, err
	}

	if err := resolved.Validate(); err != nil {
		return nil, fmt.Errorf("environment validation: %w", err)
	}

	return resolved, nil
}

// loadEnvironmentFile reads and unmarshals the environment.yaml from envDir.
func loadEnvironmentFile(envDir string) (*Environment, error) {
	envFile := filepath.Join(envDir, "environment.yaml")

	data, err := os.ReadFile(envFile)
	if err != nil {
		return nil, fmt.Errorf("read environment file %q: %w", envFile, err)
	}

	var env Environment
	if err := yaml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse environment YAML %q: %w", envFile, err)
	}

	return &env, nil
}

// findDesignRoot walks up from envDir to find the design/ root directory.
// It expects the structure: design/environments/<env-id>/environment.yaml
// so the root is three levels above the environment.yaml file.
//
// PROTOTYPE-DEBT: [td-config-001] Path walking is fragile if invoked outside
// the expected directory structure. See package-level comment.
func findDesignRoot(envDir string) (string, error) {
	// environments/<env-id>/ is two levels below design/
	designRoot := filepath.Dir(filepath.Dir(envDir))
	designMark := filepath.Join(designRoot, "devices")

	if _, err := os.Stat(designMark); err != nil {
		return "", fmt.Errorf(
			"design root not found: expected design/devices/ at %q (walked up from %q)",
			designMark, envDir,
		)
	}

	return designRoot, nil
}

// resolveRefs loads all device and network atoms referenced by the environment.
func resolveRefs(env *Environment, designRoot string) (*ResolvedEnvironment, error) {
	resolved := &ResolvedEnvironment{
		Env:      *env,
		Devices:  make(map[string]*Device),
		Networks: make(map[string]*Network),
	}

	if err := resolveNetworks(env, designRoot, resolved); err != nil {
		return nil, err
	}

	if err := resolveDevices(env, designRoot, resolved); err != nil {
		return nil, err
	}

	return resolved, nil
}

// resolveNetworks loads each network atom referenced in the environment's network list.
func resolveNetworks(env *Environment, designRoot string, resolved *ResolvedEnvironment) error {
	for _, ref := range env.Networks {
		if ref.Ref == "" {
			continue
		}
		if _, ok := resolved.Networks[ref.Ref]; ok {
			continue // already loaded (duplicate ref in environment.yaml is OK)
		}

		net, err := LoadNetwork(filepath.Join(designRoot, "networks", ref.Ref+".yaml"))
		if err != nil {
			return fmt.Errorf("resolve network ref %q: %w", ref.Ref, err)
		}
		resolved.Networks[ref.Ref] = net
	}
	return nil
}

// resolveDevices loads each device atom referenced by a placement in the environment.
func resolveDevices(env *Environment, designRoot string, resolved *ResolvedEnvironment) error {
	for _, p := range env.Placements {
		if p.Device == "" {
			continue
		}
		if _, ok := resolved.Devices[p.Device]; ok {
			continue // already loaded (multiple placements of same device type)
		}

		dev, err := LoadDevice(filepath.Join(designRoot, "devices", p.Device+".yaml"))
		if err != nil {
			return fmt.Errorf("resolve device ref %q (placement %q): %w", p.Device, p.ID, err)
		}
		resolved.Devices[p.Device] = dev
	}
	return nil
}

// LoadDevice reads and parses a single device atom YAML file.
func LoadDevice(path string) (*Device, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read device file %q: %w", path, err)
	}

	var dev Device
	if err := yaml.Unmarshal(data, &dev); err != nil {
		return nil, fmt.Errorf("parse device YAML %q: %w", path, err)
	}

	return &dev, nil
}

// LoadNetwork reads and parses a single network atom YAML file.
func LoadNetwork(path string) (*Network, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read network file %q: %w", path, err)
	}

	var net Network
	if err := yaml.Unmarshal(data, &net); err != nil {
		return nil, fmt.Errorf("parse network YAML %q: %w", path, err)
	}

	return &net, nil
}

// DefaultEnvironment returns a minimal valid ResolvedEnvironment for testing.
// It does not load from disk -- all data is constructed in memory.
func DefaultEnvironment() *ResolvedEnvironment {
	env := Environment{
		SchemaVersion: "0.1",
		Environment: EnvMeta{
			ID:   "test-default",
			Name: "OT Simulator Default Test Environment",
		},
		Networks: []NetworkRef{
			{Ref: "test-net"},
		},
		Placements: []Placement{
			{
				ID:         "test-plc-01",
				Device:     "test-device",
				Network:    "test-net",
				IP:         "10.0.0.1",
				ModbusPort: 5020,
				Role:       "Test PLC",
			},
		},
	}

	devices := map[string]*Device{
		"test-device": {
			SchemaVersion: "0.1",
			Device: DeviceMeta{
				ID:       "test-device",
				Vendor:   "Test Vendor",
				Model:    "Test Model",
				Category: "plc",
			},
		},
	}

	networks := map[string]*Network{
		"test-net": {
			SchemaVersion: "0.1",
			Network: NetworkMeta{
				ID:   "test-net",
				Name: "Test Network",
				Type: "ethernet",
			},
		},
	}

	return &ResolvedEnvironment{
		Env:      env,
		Devices:  devices,
		Networks: networks,
	}
}
