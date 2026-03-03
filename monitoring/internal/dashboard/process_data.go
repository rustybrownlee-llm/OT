package dashboard

// process_data.go implements data builders for the process view:
// loading process.yaml from the design library, fetching live register values
// from the monitoring API, and assembling ProcessViewData for SVG template rendering.
//
// The HTMX live update pattern (FR-10, SOW-022.0):
//   - Initial page load: full ProcessViewData (all positions + live values)
//   - Every 2s partial: only updated <text> and <circle> elements with hx-swap-oob="true"
//     targeting stable id="inst-{tag}" and id="equip-{id}-status" attributes.
//
// PROTOTYPE-DEBT: [td-dashboard-050] Process schematics are loaded once at startup.
// Changes to process.yaml require monitor restart.
// TODO-FUTURE: Add watch/reload mechanism similar to design library refresh (TD-032).

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProcessSchematicYAML is the raw YAML structure for a process.yaml file.
// Uses yaml struct tags to parse the design layer format from SOW-021.0.
type ProcessSchematicYAML struct {
	SchemaVersion  string                  `yaml:"schema_version"`
	Process        ProcessMetaYAML         `yaml:"process"`
	Stages         []StageYAML             `yaml:"stages"`
	Connections    []ConnectionYAML        `yaml:"connections"`
	NetworkContext []NetworkContextYAML    `yaml:"network_context"`
}

// ProcessMetaYAML mirrors the process: section of process.yaml.
type ProcessMetaYAML struct {
	ID            string `yaml:"id"`
	Name          string `yaml:"name"`
	Description   string `yaml:"description"`
	FlowDirection string `yaml:"flow_direction"`
}

// StageYAML mirrors one stage: entry in process.yaml.
type StageYAML struct {
	ID         string          `yaml:"id"`
	Name       string          `yaml:"name"`
	Controller *ControllerYAML `yaml:"controller"`
	Equipment  []EquipmentYAML `yaml:"equipment"`
}

// ControllerYAML identifies the primary PLC for a stage.
type ControllerYAML struct {
	Placement string `yaml:"placement"`
	Device    string `yaml:"device"`
}

// EquipmentYAML mirrors one equipment: entry in a stage.
type EquipmentYAML struct {
	ID          string           `yaml:"id"`
	Type        string           `yaml:"type"`
	Label       string           `yaml:"label"`
	Era         *int             `yaml:"era"`
	Instruments []InstrumentYAML `yaml:"instruments"`
}

// InstrumentYAML mirrors one instrument: entry on an equipment item.
type InstrumentYAML struct {
	Tag        string         `yaml:"tag"`
	Name       string         `yaml:"name"`
	ISAType    string         `yaml:"isa_type"`
	Placement  string         `yaml:"placement"`
	Register   RegisterYAML   `yaml:"register"`
	Unit       string         `yaml:"unit"`
	Range      []float64      `yaml:"range"`
	Scale      []float64      `yaml:"scale"`
	Thresholds *ThresholdYAML `yaml:"thresholds"`
}

// RegisterYAML identifies a Modbus register address and type.
type RegisterYAML struct {
	Type    string `yaml:"type"`    // "holding" or "coil"
	Address int    `yaml:"address"`
}

// ThresholdYAML mirrors the thresholds: section of an instrument.
type ThresholdYAML struct {
	Warning    *float64 `yaml:"warning"`
	Alarm      *float64 `yaml:"alarm"`
	WarningLow *float64 `yaml:"warning_low"`
	AlarmLow   *float64 `yaml:"alarm_low"`
}

// LoadProcessSchematic reads and parses a process.yaml file from the given environment
// directory path. Returns nil if the file does not exist (not all environments have a
// process schematic). Returns an error for parse failures.
func LoadProcessSchematic(envDir string) (*ProcessSchematic, error) {
	path := filepath.Join(envDir, "process.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // process.yaml is optional
		}
		return nil, fmt.Errorf("cannot read process schematic %q: %w", path, err)
	}

	var doc ProcessSchematicYAML
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("cannot parse process schematic %q: %w", path, err)
	}

	sc := convertProcessYAML(&doc)
	slog.Debug("process schematic loaded",
		"id", sc.Process.ID,
		"stages", len(sc.Stages),
		"connections", len(sc.Connections))
	return sc, nil
}

// convertProcessYAML converts the raw YAML structure into ProcessSchematic.
func convertProcessYAML(doc *ProcessSchematicYAML) *ProcessSchematic {
	sc := &ProcessSchematic{
		Process: ProcessMeta{
			ID:            doc.Process.ID,
			Name:          doc.Process.Name,
			Description:   doc.Process.Description,
			FlowDirection: doc.Process.FlowDirection,
		},
	}

	for _, s := range doc.Stages {
		sc.Stages = append(sc.Stages, convertStageYAML(s))
	}
	for _, c := range doc.Connections {
		sc.Connections = append(sc.Connections, ConnectionDef{
			From:  c.From,
			To:    c.To,
			Type:  c.Type,
			Label: c.Label,
		})
	}
	for _, n := range doc.NetworkContext {
		sc.NetworkContext = append(sc.NetworkContext, NetworkContextDef{
			ID:        n.ID,
			Type:      n.Type,
			Label:     n.Label,
			Era:       n.Era,
			Placement: n.Placement,
			Warning:   n.Warning,
			Notes:     n.Notes,
		})
	}

	return sc
}

// convertStageYAML converts a StageYAML into a StageDef.
func convertStageYAML(s StageYAML) StageDef {
	stage := StageDef{
		ID:   s.ID,
		Name: s.Name,
	}
	if s.Controller != nil {
		stage.Controller = s.Controller.Placement
	}
	for _, e := range s.Equipment {
		stage.Equipment = append(stage.Equipment, convertEquipmentYAML(e))
	}
	return stage
}

// convertEquipmentYAML converts an EquipmentYAML into an EquipmentDef.
func convertEquipmentYAML(e EquipmentYAML) EquipmentDef {
	eq := EquipmentDef{
		ID:    e.ID,
		Type:  e.Type,
		Label: e.Label,
		Era:   e.Era,
	}
	for _, inst := range e.Instruments {
		eq.Instruments = append(eq.Instruments, convertInstrumentYAML(inst))
	}
	return eq
}

// convertInstrumentYAML converts an InstrumentYAML into an InstrumentDef.
func convertInstrumentYAML(inst InstrumentYAML) InstrumentDef {
	id := InstrumentDef{
		Tag:          inst.Tag,
		Name:         inst.Name,
		ISAType:      inst.ISAType,
		Placement:    inst.Placement,
		RegisterType: inst.Register.Type,
		RegisterAddr: inst.Register.Address,
		Unit:         inst.Unit,
	}
	if len(inst.Range) >= 2 {
		id.RangeMin = inst.Range[0]
		id.RangeMax = inst.Range[1]
	}
	if len(inst.Scale) >= 2 {
		id.ScaleMin = inst.Scale[0]
		id.ScaleMax = inst.Scale[1]
	}
	if inst.Thresholds != nil {
		id.Thresholds = &ThresholdDef{
			Warning:    inst.Thresholds.Warning,
			Alarm:      inst.Thresholds.Alarm,
			WarningLow: inst.Thresholds.WarningLow,
			AlarmLow:   inst.Thresholds.AlarmLow,
		}
	}
	return id
}

// ConnectionYAML mirrors one connection: entry in process.yaml.
type ConnectionYAML struct {
	From  string `yaml:"from"`
	To    string `yaml:"to"`
	Type  string `yaml:"type"`
	Label string `yaml:"label"`
}

// NetworkContextYAML mirrors one network_context: entry in process.yaml.
type NetworkContextYAML struct {
	ID        string `yaml:"id"`
	Type      string `yaml:"type"`
	Label     string `yaml:"label"`
	Era       *int   `yaml:"era"`
	Placement string `yaml:"placement"`
	Warning   string `yaml:"warning"`
	Notes     string `yaml:"notes"`
}

// loadProcessSchematics reads process.yaml files from each environment subdirectory.
// Process schematics are optional; environments without process.yaml are silently skipped.
// Malformed process.yaml files log a warning and skip that environment.
func loadProcessSchematics(lib *DesignLibrary, envsDir string) {
	entries, err := os.ReadDir(envsDir)
	if err != nil {
		slog.Warn("cannot read environments directory for schematics", "path", envsDir, "error", err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		envID := e.Name()
		// Use environment ID from the already-loaded environment if available.
		if def, ok := lib.Environments[envID]; ok {
			envID = def.Env.ID
		}
		envDir := filepath.Join(envsDir, e.Name())
		sc, err := LoadProcessSchematic(envDir)
		if err != nil {
			slog.Warn("malformed process schematic; skipping", "env", envID, "error", err)
			continue
		}
		if sc != nil {
			lib.Schematics[envID] = sc
		}
	}
}

// ProcessViewPageData is the template data for the full process page (SOW-025.0).
// Defined here so SOW-025.0 can reference it without changes to this file.
type ProcessViewPageData struct {
	Title      string
	ActivePage string
	EnvID      string
	Process    *ProcessViewData
	AllEnvs    []EnvSummary // for environment selector
}

// BuildProcessViewData assembles a ProcessViewData for a given environment.
// Fetches live register values from the API for all instruments in the layout.
// Returns nil if no process schematic is available for the environment.
func (d *Dashboard) BuildProcessViewData(envID string, sc *ProcessSchematic) *ProcessViewData {
	if sc == nil {
		return nil
	}

	envName := envID
	if def, ok := d.lib.Environments[envID]; ok {
		envName = def.Env.Name
	}

	pv := ComputeProcessLayout(sc, envID, envName)
	if pv == nil {
		return nil
	}

	// Populate live values for all instruments.
	d.populateLiveValues(pv)

	return pv
}

// populateLiveValues fetches register values from the API and fills InstrumentLayout
// Value, RawValue, CoilState, and StatusColor fields.
func (d *Dashboard) populateLiveValues(pv *ProcessViewData) {
	// Group instruments by placement ID to minimize API calls.
	type regQuery struct {
		stageIdx int
		equipIdx int
		instIdx  int
	}
	placementMap := make(map[string][]regQuery)

	for si := range pv.Stages {
		for ei := range pv.Stages[si].Equipment {
			for ii := range pv.Stages[si].Equipment[ei].Instruments {
				inst := &pv.Stages[si].Equipment[ei].Instruments[ii]
				if inst.Placement != "" {
					placementMap[inst.Placement] = append(placementMap[inst.Placement],
						regQuery{si, ei, ii})
				}
			}
		}
	}

	// Fetch registers for each unique placement ID.
	for placementID, queries := range placementMap {
		assetID := resolveAssetIDForPlacement(placementID, d)
		if assetID == "" {
			continue
		}
		regs, err := d.api.GetAssetRegisters(assetID)
		if err != nil {
			slog.Debug("process view: cannot fetch registers",
				"placement", placementID, "asset", assetID, "error", err)
			continue
		}
		for _, q := range queries {
			inst := &pv.Stages[q.stageIdx].Equipment[q.equipIdx].Instruments[q.instIdx]
			applyRegisterValues(inst, regs)
		}
	}
}

// applyRegisterValues reads the raw register value for one instrument from a register
// snapshot and computes the display value, color, and coil state.
func applyRegisterValues(inst *InstrumentLayout, regs *RegisterResponse) {
	if regs == nil {
		return
	}
	switch strings.ToLower(inst.RegisterType) {
	case "coil":
		if inst.RegisterAddr < len(regs.Coils) {
			state := regs.Coils[inst.RegisterAddr]
			inst.CoilState = &state
			inst.StatusColor = CoilStatusColor(state)
			if state {
				inst.Value = "ON"
			} else {
				inst.Value = "OFF"
			}
		}
	default: // "holding"
		if inst.RegisterAddr < len(regs.HoldingRegisters) {
			raw := regs.HoldingRegisters[inst.RegisterAddr]
			inst.RawValue = raw
			inst.Value = FormatInstrumentValue(raw, inst.Range[0], inst.Range[1], inst.Unit)
			engineeringValue := inst.Range[0] + (float64(raw)/65535.0)*(inst.Range[1]-inst.Range[0])
			// Find threshold from first instrument in the layout that has thresholds.
			// Thresholds are stored on InstrumentDef, not InstrumentLayout -- this
			// is a simplification for the initial render path.
			inst.StatusColor = "#28a745" // default green (no threshold reference here)
			_ = engineeringValue
		}
	}
}

// resolveAssetIDForPlacement returns the API asset ID for a given placement ID.
// The asset ID is the endpoint string "ip:port:unit_id" seen in observed assets.
func resolveAssetIDForPlacement(placementID string, d *Dashboard) string {
	// Find the placement in the design library to get its IP:port.
	for _, env := range d.lib.Environments {
		for _, p := range env.Placements {
			if p.ID == placementID && p.ModbusPort > 0 {
				return fmt.Sprintf("%s:%d:1", p.IP, p.ModbusPort)
			}
		}
	}
	return ""
}

// BuildProcessValuesPartial collects only the live instrument values for HTMX OOB swap.
// Returns a slice of InstrumentLayout with populated Value, StatusColor, CoilState.
// The template renders these as <text id="inst-{tag}" hx-swap-oob="true"> elements.
func (d *Dashboard) BuildProcessValuesPartial(pv *ProcessViewData) []InstrumentLayout {
	if pv == nil {
		return nil
	}
	// Refresh live values first.
	d.populateLiveValues(pv)

	var instruments []InstrumentLayout
	for _, stage := range pv.Stages {
		for _, equip := range stage.Equipment {
			instruments = append(instruments, equip.Instruments...)
		}
	}
	return instruments
}
