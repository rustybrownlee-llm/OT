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
)

// RawDocument holds the top-level YAML for type detection.
// All doc sub-fields are pointers so absence is distinguishable from zero value.
type RawDocument struct {
	SchemaVersion string          `yaml:"schema_version"`
	Device        *DeviceDoc      `yaml:"device"`
	Connectivity  *ConnectivityDoc `yaml:"connectivity"`
	Registers     *RegistersDoc   `yaml:"registers"`
	Variants      map[string]VariantDoc `yaml:"register_map_variants"`
	Network       *NetworkDoc     `yaml:"network"`
	Properties    *NetworkPropertiesDoc `yaml:"properties"`
	Environment   *EnvironmentDoc `yaml:"environment"`
	Networks      []NetworkRefDoc `yaml:"networks"`
	Placements    []PlacementDoc  `yaml:"placements"`
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
