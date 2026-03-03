package validate

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FileType identifies the kind of design layer YAML document.
type FileType int

const (
	// FileTypeUnknown means the YAML did not match any expected top-level key.
	FileTypeUnknown FileType = iota
	// FileTypeDevice means the YAML contains a top-level "device:" key.
	FileTypeDevice
	// FileTypeNetwork means the YAML contains a top-level "network:" key.
	FileTypeNetwork
	// FileTypeEnvironment means the YAML contains a top-level "environment:" key.
	FileTypeEnvironment
	// FileTypeProcess means the YAML contains a top-level "process:" key.
	FileTypeProcess
)

// RawDocument holds the top-level YAML for type detection.
// All doc sub-fields are pointers so absence is distinguishable from zero value.
type RawDocument struct {
	SchemaVersion string                `yaml:"schema_version"`
	Device        *DeviceDoc            `yaml:"device"`
	Connectivity  *ConnectivityDoc      `yaml:"connectivity"`
	Registers     *RegistersDoc         `yaml:"registers"`
	Variants      map[string]VariantDoc `yaml:"register_map_variants"`
	Network       *NetworkDoc           `yaml:"network"`
	Properties    *NetworkPropertiesDoc `yaml:"properties"`
	Environment   *EnvironmentDoc       `yaml:"environment"`
	Networks      []NetworkRefDoc       `yaml:"networks"`
	Placements    []PlacementDoc        `yaml:"placements"`
	Boundaries    []BoundaryDoc         `yaml:"boundaries"` // Optional: network boundary state declarations (ADR-010 D4)
	// Process schematic fields (ADR-009, Beta 0.5 milestone).
	Process        *ProcessDoc         `yaml:"process"`
	Stages         []StageDoc          `yaml:"stages"`
	Connections    []ConnectionDoc     `yaml:"connections"`
	NetworkContext []NetworkContextDoc `yaml:"network_context"`
}

// DeviceDoc represents the device: section of a device atom YAML.
type DeviceDoc struct {
	ID          string `yaml:"id"`
	Vendor      string `yaml:"vendor"`
	Model       string `yaml:"model"`
	Category    string `yaml:"category"`
	Vintage     int    `yaml:"vintage"`
	Description string `yaml:"description"`
}

// ConnectivityDoc represents the connectivity: section.
type ConnectivityDoc struct {
	Ports                []PortDoc `yaml:"ports"`
	ResponseDelayMs      *int      `yaml:"response_delay_ms"`
	ResponseJitterMs     *int      `yaml:"response_jitter_ms"`
	ConcurrentConnections *int     `yaml:"concurrent_connections"`
}

// PortDoc represents a single connectivity port entry.
type PortDoc struct {
	Type      string   `yaml:"type"`
	Protocols []string `yaml:"protocols"`
}

// RegistersDoc represents the registers: capabilities section.
type RegistersDoc struct {
	MaxHolding          *int    `yaml:"max_holding"`
	MaxCoils            *int    `yaml:"max_coils"`
	MaxInputRegisters   *int    `yaml:"max_input_registers"`
	MaxDiscreteInputs   *int    `yaml:"max_discrete_inputs"`
	Addressing          string  `yaml:"addressing"`
	FloatByteOrder      string  `yaml:"float_byte_order"`
	MaxRegistersPerRead *int    `yaml:"max_registers_per_read"`
}

// HoldingRegisterDoc represents a single holding register entry in a variant.
type HoldingRegisterDoc struct {
	Address     *int     `yaml:"address"`
	Name        string   `yaml:"name"`
	Unit        string   `yaml:"unit"`
	ScaleMin    *float64 `yaml:"scale_min"`
	ScaleMax    *float64 `yaml:"scale_max"`
	Writable    *bool    `yaml:"writable"`
	Description string   `yaml:"description"`
}

// CoilDoc represents a single coil entry in a variant.
type CoilDoc struct {
	Address     *int   `yaml:"address"`
	Name        string `yaml:"name"`
	Writable    *bool  `yaml:"writable"`
	Description string `yaml:"description"`
}

// VariantDoc represents a register map variant (holding registers + coils).
type VariantDoc struct {
	Holding []HoldingRegisterDoc `yaml:"holding"`
	Coils   []CoilDoc            `yaml:"coils"`
}

// NetworkDoc represents the network: section of a network atom YAML.
type NetworkDoc struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

// NetworkPropertiesDoc represents the properties: section of a network atom YAML.
type NetworkPropertiesDoc struct {
	Subnet        string `yaml:"subnet"`
	VLAN          *int   `yaml:"vlan"`
	ManagedSwitch *bool  `yaml:"managed_switch"`
	SpanCapable   *bool  `yaml:"span_capable"`
}

// EnvironmentDoc represents the environment: section of an environment YAML.
type EnvironmentDoc struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Archetype   string `yaml:"archetype"`  // Optional: "modern-segmented", "legacy-flat", or "hybrid" (ADR-010 D1)
	EraSpan     string `yaml:"era_span"`   // Optional: "YYYY" or "YYYY-YYYY" installation era (ADR-010 D3)
}

// NetworkRefDoc represents a network reference in an environment's networks: list.
type NetworkRefDoc struct {
	Ref string `yaml:"ref"`
}

// PlacementDoc represents a device placement in an environment.
type PlacementDoc struct {
	ID                 string             `yaml:"id"`
	Device             string             `yaml:"device"`
	Network            string             `yaml:"network"`
	IP                 string             `yaml:"ip"`
	ModbusPort         *int               `yaml:"modbus_port"`
	SerialAddress      *int               `yaml:"serial_address"`
	Gateway            string             `yaml:"gateway"`
	Role               string             `yaml:"role"`
	RegisterMapVariant string             `yaml:"register_map_variant"`
	Bridges            []BridgeDoc        `yaml:"bridges"`
	AdditionalNetworks []AdditionalNetDoc `yaml:"additional_networks"`
	Installed          *int               `yaml:"installed"` // Optional: year this device was first commissioned in this facility (first run in production)
}

// BoundaryDoc represents a single network boundary entry in the boundaries: list.
type BoundaryDoc struct {
	Between        []string `yaml:"between"`        // Exactly 2 network refs
	State          string   `yaml:"state"`           // enforced | intended | absent
	Infrastructure string   `yaml:"infrastructure"`  // Optional enum: managed-switch | firewall | ids-sensor | vlan-only | other
	Installed      *int     `yaml:"installed"`        // Optional: year this boundary was commissioned
	Notes          string   `yaml:"notes"`            // Optional: human context for training scenarios
}

// BridgeDoc represents a gateway bridge entry.
type BridgeDoc struct {
	FromNetwork string `yaml:"from_network"`
	ToNetwork   string `yaml:"to_network"`
}

// AdditionalNetDoc represents an additional network attachment for a placement.
type AdditionalNetDoc struct {
	Network string `yaml:"network"`
	IP      string `yaml:"ip"`
}

// ProcessDoc represents the process: section of a process schematic YAML.
type ProcessDoc struct {
	ID            string `yaml:"id"`
	Name          string `yaml:"name"`
	Description   string `yaml:"description"`
	FlowDirection string `yaml:"flow_direction"` // "horizontal" (default) or "vertical"
}

// StageDoc represents a stage in the process flow.
type StageDoc struct {
	ID         string         `yaml:"id"`
	Name       string         `yaml:"name"`
	Controller *ControllerDoc `yaml:"controller"`
	Equipment  []EquipmentDoc `yaml:"equipment"`
}

// ControllerDoc optionally identifies the primary PLC for a stage.
type ControllerDoc struct {
	Placement string `yaml:"placement"`
	Device    string `yaml:"device"`
}

// EquipmentDoc represents a physical piece of equipment.
// Valid types: tank, basin, pump, valve, blower, analyzer_station, chromatograph,
// uv_system, clarifier, screen, compressor, meter, heat_exchanger.
// Note: "gateway" is NOT valid here -- gateways belong in NetworkContext per RD-1.
// "basin" is distinct from "tank" (aeration basins, equalization basins).
type EquipmentDoc struct {
	ID          string          `yaml:"id"`
	Type        string          `yaml:"type"`
	Label       string          `yaml:"label"`
	Era         *int            `yaml:"era"`
	Instruments []InstrumentDoc `yaml:"instruments"`
}

// InstrumentDoc represents a sensor or actuator mapped to a Modbus register.
type InstrumentDoc struct {
	Tag        string        `yaml:"tag"`
	Name       string        `yaml:"name"`
	ISAType    string        `yaml:"isa_type"`
	Placement  string        `yaml:"placement"`
	Register   RegisterRef   `yaml:"register"`
	Unit       string        `yaml:"unit"`
	Range      []float64     `yaml:"range"`
	Scale      []float64     `yaml:"scale"`
	Thresholds *ThresholdDoc `yaml:"thresholds"`
}

// RegisterRef identifies a specific Modbus register on a device.
type RegisterRef struct {
	Type    string `yaml:"type"`    // "holding" or "coil"
	Address int    `yaml:"address"`
}

// ThresholdDoc defines optional alarm thresholds for color coding.
// Supports bidirectional thresholds: high (warning, alarm) and low (warning_low, alarm_low).
// Level instruments (tanks, basins, clarifiers) need low thresholds for pump cavitation
// and overflow protection. All fields are optional.
type ThresholdDoc struct {
	Warning    *float64 `yaml:"warning"`     // High warning threshold
	Alarm      *float64 `yaml:"alarm"`       // High alarm threshold
	WarningLow *float64 `yaml:"warning_low"` // Low warning threshold
	AlarmLow   *float64 `yaml:"alarm_low"`   // Low alarm threshold
}

// ConnectionDoc represents a physical process connection between equipment.
// Only physical process connections belong here: "pipe" (fluid/gas) and "duct" (air delivery).
// Serial bus and wireless/WAN links belong on the topology page (RD-1) or in NetworkContext (RD-4).
type ConnectionDoc struct {
	From  string `yaml:"from"`
	To    string `yaml:"to"`
	Type  string `yaml:"type"`  // "pipe" or "duct"
	Label string `yaml:"label"`
}

// NetworkContextDoc represents a security-relevant network element overlay (RD-4).
type NetworkContextDoc struct {
	ID        string `yaml:"id"`
	Type      string `yaml:"type"`      // "wan_link", "internet_gateway", "wireless_bridge"
	Label     string `yaml:"label"`
	Era       *int   `yaml:"era"`
	Placement string `yaml:"placement"`
	Warning   string `yaml:"warning"`
	Notes     string `yaml:"notes"`
}

// LoadFile reads a YAML file from disk and unmarshals it into a RawDocument.
// Returns an error if the file cannot be read or is not valid YAML.
func LoadFile(path string) (*RawDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read file %q: %w", path, err)
	}
	var doc RawDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("cannot parse YAML in %q: %w", path, err)
	}
	return &doc, nil
}

// DetectFileType returns the type of design layer document based on top-level keys.
func DetectFileType(doc *RawDocument) FileType {
	switch {
	case doc.Device != nil:
		return FileTypeDevice
	case doc.Network != nil:
		return FileTypeNetwork
	case doc.Environment != nil:
		return FileTypeEnvironment
	case doc.Process != nil:
		return FileTypeProcess
	default:
		return FileTypeUnknown
	}
}

// FindDesignRoot walks up the directory tree from startPath looking for a directory
// that contains devices/, networks/, and environments/ subdirectories.
// Returns the design root path or an error if not found.
func FindDesignRoot(startPath string) (string, error) {
	abs, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path %q: %w", startPath, err)
	}

	// Start from the directory (not the file itself).
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("cannot stat %q: %w", abs, err)
	}
	dir := abs
	if !info.IsDir() {
		dir = filepath.Dir(abs)
	}

	for {
		if isDesignRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding design root.
			return "", fmt.Errorf(
				"design root not found: no directory containing devices/, networks/, "+
					"and environments/ found above %q", startPath,
			)
		}
		dir = parent
	}
}

// isDesignRoot returns true if dir contains devices/, networks/, and environments/.
func isDesignRoot(dir string) bool {
	for _, sub := range []string{"devices", "networks", "environments"} {
		info, err := os.Stat(filepath.Join(dir, sub))
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}
