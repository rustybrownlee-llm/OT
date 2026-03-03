package schema

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CrossRefError describes a single cross-reference validation failure.
type CrossRefError struct {
	File    string // Source file path
	Line    int    // YAML source line number (0 = unknown)
	Message string // Human-readable error description
}

// String formats the error as "file:line: message".
func (e *CrossRefError) String() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Message)
}

// CrossRefResult holds the outcome of cross-reference validation for an environment.
type CrossRefResult struct {
	Errors []*CrossRefError
}

// OK returns true when no cross-reference errors were found.
func (r *CrossRefResult) OK() bool {
	return len(r.Errors) == 0
}

// ValidateCrossRefs checks all cross-file and intra-file references in an
// environment definition. envDir must be a directory containing environment.yaml.
// designDir is the root design directory (parent of devices/, networks/, environments/).
func ValidateCrossRefs(envDir, designDir string) (*CrossRefResult, error) {
	envFilePath := filepath.Join(envDir, "environment.yaml")
	procFilePath := filepath.Join(envDir, "process.yaml")

	envData, err := os.ReadFile(envFilePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", envFilePath, err)
	}

	var envRoot yaml.Node
	if err := yaml.Unmarshal(envData, &envRoot); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", envFilePath, err)
	}

	var env environmentDef
	if err := yaml.Unmarshal(envData, &env); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", envFilePath, err)
	}

	result := &CrossRefResult{}
	ctx := &crossRefCtx{
		envFilePath: envFilePath,
		designDir:   designDir,
		envRoot:     &envRoot,
		result:      result,
		deviceCache: make(map[string]*deviceAtom),
	}

	// Build lookup sets from the environment definition.
	networkRefs := buildNetworkRefSet(env.Networks)
	placementIDs := buildPlacementIDSet(env.Placements)
	serialNetworks := ctx.loadSerialNetworkSet(env.Networks)

	ctx.checkNetworkFiles(env.Networks)
	ctx.checkDeviceFiles(env.Placements)
	ctx.checkPlacementNetworkRefs(env.Placements, networkRefs, serialNetworks)
	ctx.checkAdditionalNetworks(env.Placements, networkRefs)
	ctx.checkBridgeNetworks(env.Placements, networkRefs)
	ctx.checkGatewayRefs(env.Placements, placementIDs)
	ctx.checkRegisterMapVariants(env.Placements)
	ctx.checkBoundaryNetworks(env.Boundaries, networkRefs)
	ctx.checkRegisterAddressBounds(env.Placements)

	if fileExists(procFilePath) {
		procData, err := os.ReadFile(procFilePath)
		if err == nil {
			var proc processDef
			_ = yaml.Unmarshal(procData, &proc)
			ctx.checkProcessConsistency(proc, placementIDs, env.Placements, procFilePath)
		}
	}

	return result, nil
}

// crossRefCtx carries shared state for a single cross-reference validation run.
type crossRefCtx struct {
	envFilePath string
	designDir   string
	envRoot     *yaml.Node
	result      *CrossRefResult
	deviceCache map[string]*deviceAtom
}

// addError appends a cross-reference error, resolving the line number from the
// yaml.Node tree using a JSON Pointer path.
func (ctx *crossRefCtx) addError(jsonPointer, msg string) {
	line := ResolveLineNumber(ctx.envRoot, jsonPointer)
	ctx.result.Errors = append(ctx.result.Errors, &CrossRefError{
		File:    ctx.envFilePath,
		Line:    line,
		Message: msg,
	})
}

// addErrorForFile appends an error associated with an explicit file (e.g., process.yaml).
func (ctx *crossRefCtx) addErrorForFile(filePath string, line int, msg string) {
	ctx.result.Errors = append(ctx.result.Errors, &CrossRefError{
		File:    filePath,
		Line:    line,
		Message: msg,
	})
}

// loadSerialNetworkSet returns a set of network IDs that are serial-type networks.
func (ctx *crossRefCtx) loadSerialNetworkSet(networks []networkRef) map[string]bool {
	serial := make(map[string]bool)
	for _, n := range networks {
		netFile := filepath.Join(ctx.designDir, "networks", n.Ref+".yaml")
		data, err := os.ReadFile(netFile)
		if err != nil {
			continue
		}
		var atom networkAtomFile
		if err := yaml.Unmarshal(data, &atom); err != nil {
			continue
		}
		if atom.Network.Type == "serial-rs485" || atom.Network.Type == "serial-rs232" {
			serial[n.Ref] = true
		}
	}
	return serial
}

// checkNetworkFiles verifies that each networks[].ref resolves to a file in design/networks/.
func (ctx *crossRefCtx) checkNetworkFiles(networks []networkRef) {
	for i, n := range networks {
		path := filepath.Join(ctx.designDir, "networks", n.Ref+".yaml")
		if !fileExists(path) {
			ctx.addError(fmt.Sprintf("/networks/%d/ref", i),
				fmt.Sprintf("network ref %q: file design/networks/%s.yaml not found", n.Ref, n.Ref))
		}
	}
}

// checkDeviceFiles verifies that each placement.device resolves to a file in design/devices/.
func (ctx *crossRefCtx) checkDeviceFiles(placements []placement) {
	for i, p := range placements {
		path := filepath.Join(ctx.designDir, "devices", p.Device+".yaml")
		if !fileExists(path) {
			ctx.addError(fmt.Sprintf("/placements/%d/device", i),
				fmt.Sprintf("placement %q: device %q not found in design/devices/", p.ID, p.Device))
		}
	}
}

// checkPlacementNetworkRefs verifies placement.network is in the environment networks
// and enforces the serial-network IP absence rule.
func (ctx *crossRefCtx) checkPlacementNetworkRefs(
	placements []placement, networkRefs, serialNetworks map[string]bool,
) {
	for i, p := range placements {
		if p.Network == "" {
			continue
		}
		if !networkRefs[p.Network] {
			ctx.addError(fmt.Sprintf("/placements/%d/network", i),
				fmt.Sprintf("placement %q: network %q is not listed in environment networks", p.ID, p.Network))
		}
		if serialNetworks[p.Network] && p.IP != "" {
			ctx.addError(fmt.Sprintf("/placements/%d/ip", i),
				fmt.Sprintf("placement %q: ip field must not be set for serial-network placements (network %q is serial)", p.ID, p.Network))
		}
	}
}

// checkAdditionalNetworks verifies that additional_networks[].network references are valid.
func (ctx *crossRefCtx) checkAdditionalNetworks(placements []placement, networkRefs map[string]bool) {
	for i, p := range placements {
		for j, an := range p.AdditionalNetworks {
			if !networkRefs[an.Network] {
				ctx.addError(fmt.Sprintf("/placements/%d/additional_networks/%d/network", i, j),
					fmt.Sprintf("placement %q additional_networks[%d]: network %q is not listed in environment networks", p.ID, j, an.Network))
			}
		}
	}
}

// checkBridgeNetworks verifies that bridge from_network and to_network references are valid.
func (ctx *crossRefCtx) checkBridgeNetworks(placements []placement, networkRefs map[string]bool) {
	for i, p := range placements {
		for j, b := range p.Bridges {
			if !networkRefs[b.FromNetwork] {
				ctx.addError(fmt.Sprintf("/placements/%d/bridges/%d/from_network", i, j),
					fmt.Sprintf("placement %q bridges[%d]: from_network %q is not listed in environment networks", p.ID, j, b.FromNetwork))
			}
			if !networkRefs[b.ToNetwork] {
				ctx.addError(fmt.Sprintf("/placements/%d/bridges/%d/to_network", i, j),
					fmt.Sprintf("placement %q bridges[%d]: to_network %q is not listed in environment networks", p.ID, j, b.ToNetwork))
			}
		}
	}
}

// checkGatewayRefs verifies that gateway fields reference valid placement IDs.
func (ctx *crossRefCtx) checkGatewayRefs(placements []placement, placementIDs map[string]bool) {
	for i, p := range placements {
		if p.Gateway == "" {
			continue
		}
		if !placementIDs[p.Gateway] {
			ctx.addError(fmt.Sprintf("/placements/%d/gateway", i),
				fmt.Sprintf("placement %q: gateway %q is not a placement ID in this environment", p.ID, p.Gateway))
		}
	}
}

// checkRegisterMapVariants verifies that register_map_variant values exist in the device atom.
func (ctx *crossRefCtx) checkRegisterMapVariants(placements []placement) {
	for i, p := range placements {
		if p.RegisterMapVariant == "" {
			continue
		}
		atom, err := ctx.loadDeviceAtom(p.Device)
		if err != nil {
			continue // device file missing; already reported by checkDeviceFiles
		}
		if !variantExists(atom, p.RegisterMapVariant) {
			ctx.addError(fmt.Sprintf("/placements/%d/register_map_variant", i),
				fmt.Sprintf("placement %q: register_map_variant %q not found in device %q", p.ID, p.RegisterMapVariant, p.Device))
		}
	}
}

// checkBoundaryNetworks verifies that boundaries[].between network IDs exist in the environment.
func (ctx *crossRefCtx) checkBoundaryNetworks(boundaries []boundary, networkRefs map[string]bool) {
	for i, b := range boundaries {
		for j, net := range b.Between {
			if !networkRefs[net] {
				ctx.addError(fmt.Sprintf("/boundaries/%d/between/%d", i, j),
					fmt.Sprintf("boundary[%d]: network %q in between[%d] is not listed in environment networks", i, net, j))
			}
		}
	}
}

// checkRegisterAddressBounds verifies register addresses stay within device capacity.
func (ctx *crossRefCtx) checkRegisterAddressBounds(placements []placement) {
	for i, p := range placements {
		if p.RegisterMapVariant == "" {
			continue
		}
		atom, err := ctx.loadDeviceAtom(p.Device)
		if err != nil {
			continue
		}
		variant, ok := atom.RegisterMapVariants[p.RegisterMapVariant]
		if !ok {
			continue
		}
		for _, reg := range variant.Holding {
			if reg.Address >= atom.Registers.MaxHolding {
				ctx.addError(fmt.Sprintf("/placements/%d/register_map_variant", i),
					fmt.Sprintf("placement %q variant %q: holding register address %d exceeds device max_holding %d",
						p.ID, p.RegisterMapVariant, reg.Address, atom.Registers.MaxHolding))
			}
		}
		for _, coil := range variant.Coils {
			if coil.Address >= atom.Registers.MaxCoils {
				ctx.addError(fmt.Sprintf("/placements/%d/register_map_variant", i),
					fmt.Sprintf("placement %q variant %q: coil address %d exceeds device max_coils %d",
						p.ID, p.RegisterMapVariant, coil.Address, atom.Registers.MaxCoils))
			}
		}
	}
}

// checkProcessConsistency validates process.yaml references against the environment.
func (ctx *crossRefCtx) checkProcessConsistency(
	proc processDef, placementIDs map[string]bool, placements []placement, processFilePath string,
) {
	placementDevices := buildPlacementDeviceMap(placements)

	for _, stage := range proc.Stages {
		if stage.Controller.Placement != "" && !placementIDs[stage.Controller.Placement] {
			ctx.addErrorForFile(processFilePath, 0,
				fmt.Sprintf("stage %q controller.placement %q is not a placement ID in the environment", stage.ID, stage.Controller.Placement))
		}
		if stage.Controller.Placement != "" && stage.Controller.Device != "" {
			if actualDevice, ok := placementDevices[stage.Controller.Placement]; ok {
				if actualDevice != stage.Controller.Device {
					ctx.addErrorForFile(processFilePath, 0,
						fmt.Sprintf("stage %q controller.device %q does not match placement %q device %q",
							stage.ID, stage.Controller.Device, stage.Controller.Placement, actualDevice))
				}
			}
		}
		for _, equip := range stage.Equipment {
			for _, inst := range equip.Instruments {
				if inst.Placement != "" && !placementIDs[inst.Placement] {
					ctx.addErrorForFile(processFilePath, 0,
						fmt.Sprintf("stage %q equipment %q instrument %q placement %q is not a placement ID in the environment",
							stage.ID, equip.ID, inst.Tag, inst.Placement))
				}
			}
		}
	}

	for _, nc := range proc.NetworkContext {
		if nc.Placement != "" && !placementIDs[nc.Placement] {
			ctx.addErrorForFile(processFilePath, 0,
				fmt.Sprintf("network_context %q placement %q is not a placement ID in the environment", nc.ID, nc.Placement))
		}
	}
}

// loadDeviceAtom parses a device atom from design/devices/<deviceID>.yaml.
// Results are cached within the validation run.
func (ctx *crossRefCtx) loadDeviceAtom(deviceID string) (*deviceAtom, error) {
	if atom, ok := ctx.deviceCache[deviceID]; ok {
		return atom, nil
	}
	path := filepath.Join(ctx.designDir, "devices", deviceID+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading device atom %s: %w", deviceID, err)
	}
	var atom deviceAtom
	if err := yaml.Unmarshal(data, &atom); err != nil {
		return nil, fmt.Errorf("parsing device atom %s: %w", deviceID, err)
	}
	ctx.deviceCache[deviceID] = &atom
	return &atom, nil
}

// --- Minimal YAML struct types for cross-reference parsing ---

// environmentDef is a minimal struct for parsing environment.yaml fields needed
// for cross-reference validation. It intentionally excludes fields not used by the validator.
// PROTOTYPE-DEBT: [td-admin-100] This duplicates structure from any potential plant config types.
type environmentDef struct {
	Networks   []networkRef `yaml:"networks"`
	Placements []placement  `yaml:"placements"`
	Boundaries []boundary   `yaml:"boundaries"`
}

type networkRef struct {
	Ref string `yaml:"ref"`
}

type placement struct {
	ID                 string             `yaml:"id"`
	Device             string             `yaml:"device"`
	Network            string             `yaml:"network"`
	IP                 string             `yaml:"ip"`
	Gateway            string             `yaml:"gateway"`
	RegisterMapVariant string             `yaml:"register_map_variant"`
	AdditionalNetworks []additionalNet    `yaml:"additional_networks"`
	Bridges            []bridgeSpec       `yaml:"bridges"`
}

type additionalNet struct {
	Network string `yaml:"network"`
	IP      string `yaml:"ip"`
}

type bridgeSpec struct {
	FromNetwork string `yaml:"from_network"`
	ToNetwork   string `yaml:"to_network"`
}

type boundary struct {
	Between []string `yaml:"between"`
}

// processDef is a minimal struct for parsing process.yaml fields needed
// for cross-reference validation.
type processDef struct {
	Stages         []stage          `yaml:"stages"`
	NetworkContext []networkContext `yaml:"network_context"`
}

type stage struct {
	ID         string      `yaml:"id"`
	Controller controller  `yaml:"controller"`
	Equipment  []equipment `yaml:"equipment"`
}

type controller struct {
	Placement string `yaml:"placement"`
	Device    string `yaml:"device"`
}

type equipment struct {
	ID          string       `yaml:"id"`
	Instruments []instrument `yaml:"instruments"`
}

type instrument struct {
	Tag       string `yaml:"tag"`
	Placement string `yaml:"placement"`
}

type networkContext struct {
	ID        string `yaml:"id"`
	Placement string `yaml:"placement"`
}

type networkAtomFile struct {
	Network struct {
		Type string `yaml:"type"`
	} `yaml:"network"`
}

type deviceAtom struct {
	Registers struct {
		MaxHolding int `yaml:"max_holding"`
		MaxCoils   int `yaml:"max_coils"`
	} `yaml:"registers"`
	RegisterMapVariants map[string]registerMap `yaml:"register_map_variants"`
}

type registerMap struct {
	Holding []registerEntry `yaml:"holding"`
	Coils   []registerEntry `yaml:"coils"`
}

type registerEntry struct {
	Address int `yaml:"address"`
}

// --- Helper functions ---

func buildNetworkRefSet(networks []networkRef) map[string]bool {
	set := make(map[string]bool, len(networks))
	for _, n := range networks {
		set[n.Ref] = true
	}
	return set
}

func buildPlacementIDSet(placements []placement) map[string]bool {
	set := make(map[string]bool, len(placements))
	for _, p := range placements {
		set[p.ID] = true
	}
	return set
}

func buildPlacementDeviceMap(placements []placement) map[string]string {
	m := make(map[string]string, len(placements))
	for _, p := range placements {
		m[p.ID] = p.Device
	}
	return m
}

func variantExists(atom *deviceAtom, variant string) bool {
	_, ok := atom.RegisterMapVariants[variant]
	return ok
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
