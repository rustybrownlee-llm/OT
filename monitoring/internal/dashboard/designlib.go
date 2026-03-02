package dashboard

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DesignLibrary holds parsed design-layer YAML for the reference pages.
// Loaded once at startup. Read-only after initialization.
// ADR-005 D4: this does NOT import plant packages. All types are defined locally.
type DesignLibrary struct {
	Devices      map[string]*DeviceAtom    // keyed by device ID
	Networks     map[string]*NetworkAtom   // keyed by network ID
	Environments map[string]*EnvironmentDef // keyed by environment ID
	RawYAML      map[string]string         // keyed by "<type>/<id>", raw content for display
}

// DeviceAtom is the monitoring-local representation of a device atom YAML.
// Parses only the fields needed for dashboard display.
type DeviceAtom struct {
	SchemaVersion string                       `yaml:"schema_version"`
	Device        DeviceIdentity               `yaml:"device"`
	Connectivity  DeviceConnectivity           `yaml:"connectivity"`
	Registers     RegisterCapability           `yaml:"registers"`
	Variants      map[string]RegisterMapVariant `yaml:"register_map_variants"`
	Diagnostics   DeviceDiagnostics            `yaml:"diagnostics"`
}

// DeviceIdentity holds the identity fields from a device atom.
type DeviceIdentity struct {
	ID          string `yaml:"id"`
	Vendor      string `yaml:"vendor"`
	Model       string `yaml:"model"`
	Category    string `yaml:"category"`
	Vintage     int    `yaml:"vintage"`
	Description string `yaml:"description"`
}

// DeviceConnectivity holds connectivity fields from a device atom.
type DeviceConnectivity struct {
	ResponseDelayMs      int `yaml:"response_delay_ms"`
	ResponseJitterMs     int `yaml:"response_jitter_ms"`
	ConcurrentConnections int `yaml:"concurrent_connections"`
}

// RegisterCapability holds the hardware register limits from a device atom.
type RegisterCapability struct {
	MaxHolding          int    `yaml:"max_holding"`
	MaxCoils            int    `yaml:"max_coils"`
	MaxInputRegisters   int    `yaml:"max_input_registers"`
	MaxDiscreteInputs   int    `yaml:"max_discrete_inputs"`
	Addressing          string `yaml:"addressing"`
	FloatByteOrder      string `yaml:"float_byte_order"`
	MaxRegistersPerRead int    `yaml:"max_registers_per_read"`
}

// RegisterMapVariant holds one named register map variant from a device atom.
type RegisterMapVariant struct {
	Holding []RegisterEntry `yaml:"holding"`
	Coils   []CoilEntry     `yaml:"coils"`
}

// RegisterEntry describes one holding register in a variant.
type RegisterEntry struct {
	Address     int     `yaml:"address"`
	Name        string  `yaml:"name"`
	Unit        string  `yaml:"unit"`
	ScaleMin    float64 `yaml:"scale_min"`
	ScaleMax    float64 `yaml:"scale_max"`
	Writable    bool    `yaml:"writable"`
	Description string  `yaml:"description"`
}

// CoilEntry describes one coil in a variant.
type CoilEntry struct {
	Address     int    `yaml:"address"`
	Name        string `yaml:"name"`
	Writable    bool   `yaml:"writable"`
	Description string `yaml:"description"`
}

// DeviceDiagnostics holds the diagnostics section of a device atom.
type DeviceDiagnostics struct {
	WebServer     bool `yaml:"web_server"`
	WebPort       int  `yaml:"web_port"`
	SNMP          bool `yaml:"snmp"`
	Syslog        bool `yaml:"syslog"`
	FaultLog      bool `yaml:"fault_log"`
	FaultLogDepth int  `yaml:"fault_log_depth"`
}

// NetworkAtom is the monitoring-local representation of a network atom YAML.
type NetworkAtom struct {
	SchemaVersion string            `yaml:"schema_version"`
	Network       NetworkIdentity   `yaml:"network"`
	Properties    NetworkProperties `yaml:"properties"`
}

// NetworkIdentity holds the identity fields from a network atom.
type NetworkIdentity struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

// NetworkProperties holds properties from a network atom.
type NetworkProperties struct {
	Subnet        string `yaml:"subnet"`
	VLAN          int    `yaml:"vlan"`
	ManagedSwitch bool   `yaml:"managed_switch"`
	SPANCapable   bool   `yaml:"span_capable"`
}

// EnvironmentDef is the monitoring-local representation of an environment YAML.
type EnvironmentDef struct {
	SchemaVersion string           `yaml:"schema_version"`
	Env           EnvironmentMeta  `yaml:"environment"`
	Networks      []NetworkRef     `yaml:"networks"`
	Placements    []Placement      `yaml:"placements"`
}

// EnvironmentMeta holds the identity fields from an environment.
type EnvironmentMeta struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// NetworkRef is a reference to a network atom in an environment.
type NetworkRef struct {
	Ref string `yaml:"ref"`
}

// Placement wires a device atom onto a network with specific addressing.
type Placement struct {
	ID                 string `yaml:"id"`
	Device             string `yaml:"device"`
	Network            string `yaml:"network"`
	IP                 string `yaml:"ip"`
	ModbusPort         int    `yaml:"modbus_port"`
	SerialAddress      int    `yaml:"serial_address"`
	Gateway            string `yaml:"gateway"`
	Role               string `yaml:"role"`
	RegisterMapVariant string `yaml:"register_map_variant"`
}

// emptyLibrary returns a DesignLibrary with empty but non-nil maps.
func emptyLibrary() *DesignLibrary {
	return &DesignLibrary{
		Devices:      make(map[string]*DeviceAtom),
		Networks:     make(map[string]*NetworkAtom),
		Environments: make(map[string]*EnvironmentDef),
		RawYAML:      make(map[string]string),
	}
}

// LoadDesignLibrary reads all YAML files from designDir and returns a populated
// DesignLibrary. If designDir does not exist, a warning is logged and an empty
// library is returned (not an error). Malformed YAML files are skipped with
// a warning; they do not prevent other files from loading.
func LoadDesignLibrary(designDir string) (*DesignLibrary, error) {
	lib := emptyLibrary()

	if _, err := os.Stat(designDir); errors.Is(err, os.ErrNotExist) {
		slog.Warn("design library directory not found; reference pages will be unavailable",
			"design_dir", designDir)
		return lib, nil
	}

	loadDevices(lib, filepath.Join(designDir, "devices"))
	loadNetworks(lib, filepath.Join(designDir, "networks"))
	loadEnvironments(lib, filepath.Join(designDir, "environments"))

	slog.Info("design library loaded",
		"devices", len(lib.Devices),
		"networks", len(lib.Networks),
		"environments", len(lib.Environments))

	return lib, nil
}

// loadDevices reads all *.yaml files in devicesDir into lib.Devices.
func loadDevices(lib *DesignLibrary, devicesDir string) {
	entries, err := os.ReadDir(devicesDir)
	if err != nil {
		slog.Warn("cannot read devices directory", "path", devicesDir, "error", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(devicesDir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("cannot read device atom file", "path", path, "error", err)
			continue
		}
		var atom DeviceAtom
		if err := yaml.Unmarshal(raw, &atom); err != nil {
			slog.Warn("malformed device atom YAML; skipping", "path", path, "error", err)
			continue
		}
		// A successfully-parsed but completely empty struct indicates invalid content.
		if atom.Device.Vendor == "" && atom.Device.Model == "" && atom.SchemaVersion == "" {
			slog.Warn("device atom YAML has no recognizable content; skipping", "path", path)
			continue
		}
		if atom.Device.ID == "" {
			atom.Device.ID = strings.TrimSuffix(e.Name(), ".yaml")
		}
		lib.Devices[atom.Device.ID] = &atom
		lib.RawYAML["devices/"+atom.Device.ID] = string(raw)
	}
}

// loadNetworks reads all *.yaml files in networksDir into lib.Networks.
func loadNetworks(lib *DesignLibrary, networksDir string) {
	entries, err := os.ReadDir(networksDir)
	if err != nil {
		slog.Warn("cannot read networks directory", "path", networksDir, "error", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(networksDir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("cannot read network atom file", "path", path, "error", err)
			continue
		}
		var atom NetworkAtom
		if err := yaml.Unmarshal(raw, &atom); err != nil {
			slog.Warn("malformed network atom YAML; skipping", "path", path, "error", err)
			continue
		}
		if atom.Network.ID == "" {
			atom.Network.ID = strings.TrimSuffix(e.Name(), ".yaml")
		}
		lib.Networks[atom.Network.ID] = &atom
		lib.RawYAML["networks/"+atom.Network.ID] = string(raw)
	}
}

// loadEnvironments reads environment.yaml files from subdirectories of envsDir.
func loadEnvironments(lib *DesignLibrary, envsDir string) {
	entries, err := os.ReadDir(envsDir)
	if err != nil {
		slog.Warn("cannot read environments directory", "path", envsDir, "error", err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(envsDir, e.Name(), "environment.yaml")
		raw, err := os.ReadFile(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				slog.Warn("cannot read environment file", "path", path, "error", err)
			}
			continue
		}
		var def EnvironmentDef
		if err := yaml.Unmarshal(raw, &def); err != nil {
			slog.Warn("malformed environment YAML; skipping", "path", path, "error", err)
			continue
		}
		if def.Env.ID == "" {
			def.Env.ID = e.Name()
		}
		lib.Environments[def.Env.ID] = &def
		lib.RawYAML["environments/"+def.Env.ID] = string(raw)
	}
}

// ResolveDeviceAtomID attempts to identify which device atom an observed asset
// corresponds to. It parses the config_description field for a known placement ID,
// then looks up that placement in the environment definitions to find the device atom ID.
// Returns "" if no match is found -- no fallback heuristics are used.
// [OT-REVIEW] Register count/addressing fallback removed: produces silent false matches.
func (lib *DesignLibrary) ResolveDeviceAtomID(configDesc string) string {
	placementID := extractPlacementID(configDesc)
	if placementID == "" {
		return ""
	}
	for _, env := range lib.Environments {
		for _, p := range env.Placements {
			if p.ID == placementID {
				return p.Device
			}
		}
	}
	return ""
}

// ResolveVariantForPlacement finds the register_map_variant for a given placement ID.
func (lib *DesignLibrary) ResolveVariantForPlacement(configDesc string) string {
	placementID := extractPlacementID(configDesc)
	if placementID == "" {
		return ""
	}
	for _, env := range lib.Environments {
		for _, p := range env.Placements {
			if p.ID == placementID {
				return p.RegisterMapVariant
			}
		}
	}
	return ""
}

// extractPlacementID parses a config_description like "wt-plc-01 (water intake)"
// and returns the leading placement ID token (e.g., "wt-plc-01").
func extractPlacementID(configDesc string) string {
	if configDesc == "" {
		return ""
	}
	// The placement ID is the first space-delimited token.
	idx := strings.IndexByte(configDesc, ' ')
	if idx < 0 {
		return configDesc
	}
	return configDesc[:idx]
}

// EnvsUsingNetwork returns the IDs and names of all environments that reference
// the given network atom ID in their networks list.
func (lib *DesignLibrary) EnvsUsingNetwork(networkID string) []struct{ ID, Name string } {
	var result []struct{ ID, Name string }
	for _, env := range lib.Environments {
		for _, ref := range env.Networks {
			if ref.Ref == networkID {
				result = append(result, struct{ ID, Name string }{env.Env.ID, env.Env.Name})
				break
			}
		}
	}
	return result
}

// IsEmpty reports whether the design library has no content.
func (lib *DesignLibrary) IsEmpty() bool {
	return len(lib.Devices) == 0 && len(lib.Networks) == 0 && len(lib.Environments) == 0
}

// purdueLevel derives a human-readable Purdue level label from a network ID.
// The label is derived from the network type and name convention in this project.
func purdueLevel(netID string) string {
	levels := map[string]string{
		"wt-level1": "Level 1 - Basic Control",
		"wt-level2": "Level 2 - Supervisory Control",
		"wt-level3": "Level 3 - Site Operations",
	}
	label, ok := levels[netID]
	if !ok {
		return ""
	}
	return label
}

// ScaleValue converts a raw uint16 Modbus value to an engineering unit value
// using the linear mapping convention from the device atoms.
// Convention: 0 maps to scaleMin, 65535 maps to scaleMax.
func ScaleValue(raw uint16, scaleMin, scaleMax float64) float64 {
	return scaleMin + (float64(raw)/65535.0)*(scaleMax-scaleMin)
}

// FormatScaled formats a scaled value with two decimal places and its unit.
func FormatScaled(raw uint16, scaleMin, scaleMax float64, unit string) string {
	v := ScaleValue(raw, scaleMin, scaleMax)
	if unit != "" {
		return fmt.Sprintf("%.2f %s", v, unit)
	}
	return fmt.Sprintf("%.2f", v)
}
