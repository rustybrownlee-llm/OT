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
	Devices      map[string]*DeviceAtom      // keyed by device ID
	Networks     map[string]*NetworkAtom     // keyed by network ID
	Environments map[string]*EnvironmentDef  // keyed by environment ID
	Schematics   map[string]*ProcessSchematic // keyed by environment ID; optional per env
	RawYAML      map[string]string           // keyed by "<type>/<id>", raw content for display
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
	Boundaries    []BoundaryDef    `yaml:"boundaries"` // SOW-016.0 boundary states
}

// EnvironmentMeta holds the identity fields from an environment.
type EnvironmentMeta struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Archetype   string `yaml:"archetype"`  // "modern-segmented", "legacy-flat", "hybrid"
	EraSpan     string `yaml:"era_span"`   // e.g. "1997-2022" or "2015"
}

// NetworkRef is a reference to a network atom in an environment.
type NetworkRef struct {
	Ref string `yaml:"ref"`
}

// Placement wires a device atom onto a network with specific addressing.
type Placement struct {
	ID                 string              `yaml:"id"`
	Device             string              `yaml:"device"`
	Network            string              `yaml:"network"`
	IP                 string              `yaml:"ip"`
	ModbusPort         int                 `yaml:"modbus_port"`
	SerialAddress      int                 `yaml:"serial_address"`
	Gateway            string              `yaml:"gateway"`
	Role               string              `yaml:"role"`
	RegisterMapVariant string              `yaml:"register_map_variant"`
	Installed          *int                `yaml:"installed"`           // SOW-016.0 era marker
	AdditionalNetworks []AdditionalNetwork `yaml:"additional_networks"` // secondary network interfaces
	Bridges            []BridgeDef         `yaml:"bridges"`             // gateway bridge definitions
}

// AdditionalNetwork records a secondary network interface on a placement.
type AdditionalNetwork struct {
	Network string `yaml:"network"`
	IP      string `yaml:"ip"`
}

// BridgeDef records one bridge link on a gateway placement.
type BridgeDef struct {
	FromNetwork string `yaml:"from_network"`
	ToNetwork   string `yaml:"to_network"`
}

// BoundaryDef describes the segmentation state between two network tiers.
type BoundaryDef struct {
	Between        []string `yaml:"between"`
	State          string   `yaml:"state"`          // "enforced", "intended", "absent"
	Infrastructure string   `yaml:"infrastructure"` // e.g. "managed-switch"
	Installed      *int     `yaml:"installed"`
	Notes          string   `yaml:"notes"`
}

// emptyLibrary returns a DesignLibrary with empty but non-nil maps.
func emptyLibrary() *DesignLibrary {
	return &DesignLibrary{
		Devices:      make(map[string]*DeviceAtom),
		Networks:     make(map[string]*NetworkAtom),
		Environments: make(map[string]*EnvironmentDef),
		Schematics:   make(map[string]*ProcessSchematic),
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
	loadProcessSchematics(lib, filepath.Join(designDir, "environments"))

	slog.Info("design library loaded",
		"devices", len(lib.Devices),
		"networks", len(lib.Networks),
		"environments", len(lib.Environments),
		"schematics", len(lib.Schematics))

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

// inferPurdueLevel derives a Purdue level label for a network using an archetype-aware
// heuristic based on network ID naming conventions and archetype context.
//
// PROTOTYPE-DEBT: [td-045] Level assigned from network naming convention, not a schema
// field. TD-035 remains open for a purdue_level field on network atoms.
//
// Inference rules are evaluated in order:
//  1. Flat pre-filter: legacy-flat or hybrid archetypes with flat/unmanaged ethernet
//     networks yield "Flat (Unclassified)" -- no Purdue level.
//  2. Explicit level keywords: level1, level2, level3 in network ID.
//  3. Serial backbone: "serial" in network ID.
//  4. WAN/External: "wan" in network ID.
//  5. Point-to-point: short subnet or cross-plant link.
//  6. Default: "Unclassified".
//
// Note: This heuristic is a simulator convention for controlled naming, not a standard.
// Real environments require architectural analysis to assign Purdue levels.
func inferPurdueLevel(netID, archetype string, net *NetworkAtom) networkLevelInfo {
	isLegacyContext := archetype == "legacy-flat" || archetype == "hybrid"

	// Rule 1: Flat pre-filter for legacy and hybrid environments.
	// Flat networks in these archetypes have no Purdue level -- they coexist on one wire.
	if isLegacyContext && net != nil {
		if strings.Contains(netID, "flat") ||
			(net.Properties.ManagedSwitch == false && net.Properties.VLAN == 0 && net.Network.Type == "ethernet") {
			return networkLevelInfo{Label: "Flat (Unclassified)", PLevel: -1, IsFlat: true}
		}
	}

	// Rule 2-4: Explicit level keywords take precedence over default.
	switch {
	case strings.Contains(netID, "level1"):
		return networkLevelInfo{Label: "Level 1 - Basic Control", PLevel: 1}
	case strings.Contains(netID, "level2"):
		return networkLevelInfo{Label: "Level 2 - Supervisory Control", PLevel: 2}
	case strings.Contains(netID, "level3"):
		return networkLevelInfo{Label: "Level 3 - Site Operations", PLevel: 3}
	case strings.Contains(netID, "serial"):
		// Serial buses are physical layer, not a Purdue level. Attach to gateway device's level.
		return networkLevelInfo{Label: "Serial Backbone", PLevel: -2, IsSerial: true}
	case strings.Contains(netID, "wan"):
		// WAN is external connectivity, not a DMZ (Level 3.5). Render at top.
		return networkLevelInfo{Label: "External/WAN", PLevel: 99, IsWAN: true}
	}

	// Rule 5: Point-to-point cross-plant links.
	if strings.Contains(netID, "cross") {
		return networkLevelInfo{Label: "Link", PLevel: -3, IsLink: true}
	}

	// Rule 6: Default unclassified.
	return networkLevelInfo{Label: "Unclassified", PLevel: 0, IsUnclassified: true}
}

// networkLevelInfo holds the result of Purdue level inference for one network.
type networkLevelInfo struct {
	Label          string
	PLevel         int  // -3=link, -2=serial, -1=flat, 0=unclassified, 1-3=Purdue level, 99=WAN
	IsFlat         bool
	IsSerial       bool
	IsWAN          bool
	IsLink         bool
	IsUnclassified bool
}

// purdueLevel is retained for the design network detail page, which shows the level
// for a single network without archetype context. It uses the inference heuristic
// with an empty archetype (no flat pre-filter applied).
func purdueLevel(netID string) string {
	info := inferPurdueLevel(netID, "", nil)
	if info.IsUnclassified {
		return ""
	}
	return info.Label
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
