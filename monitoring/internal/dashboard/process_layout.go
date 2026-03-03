package dashboard

// process_layout.go defines data types and display helpers for the SVG process view.
// Computation functions are in process_layout_engine.go.
//
// PROTOTYPE-DEBT: [td-041] Horizontal layout only. flow_direction: vertical is accepted
// by the schema but ignored; always uses horizontal layout.
// TODO-FUTURE: Add vertical layout path in Beta 0.5 or 0.6 follow-up.
//
// PROTOTYPE-DEBT: [td-042] Connection routing is straight lines only.
// TODO-FUTURE: Add orthogonal routing for complex multi-path flows.
//
// PROTOTYPE-DEBT: [td-043] Tank/basin fill level is cosmetic only.
// TODO-FUTURE: Add animated SVG fill via HTMX-triggered CSS transitions.
//
// PROTOTYPE-DEBT: [td-044] No SVG accessibility attributes.
// TODO-FUTURE: Add aria labels and SVG title/desc elements on all symbol templates.

import "fmt"

// Layout spacing constants for the horizontal process view.
const (
	stageSpacing      = 280.0 // px between stage left edges
	equipmentSpacing  = 120.0 // px between equipment centers (vertical, center-to-center)
	instrumentOffsetX = 150.0 // px right of equipment center for instrument callout
	stagePadding      = 20.0  // px internal stage padding
	stageHeaderHeight = 36.0  // px reserved for stage name header
	viewBoxMargin     = 40.0  // px margin on all sides of content bounding box
	minPipeArrowMid   = 300.0 // px threshold for adding a mid-pipe arrowhead
	stageColumnWidth  = 120.0 // px fixed equipment column width; all equipment centers share same X
)

// ProcessSchematic holds the full parsed content of a process.yaml document.
// Monitoring-local representation; mirrors the design layer without importing plant packages.
type ProcessSchematic struct {
	Process        ProcessMeta
	Stages         []StageDef
	Connections    []ConnectionDef
	NetworkContext []NetworkContextDef
}

// ProcessMeta holds identity fields from the process: section.
type ProcessMeta struct {
	ID            string
	Name          string
	Description   string
	FlowDirection string // "horizontal" (default) or "vertical" -- vertical is TD-041
}

// StageDef is one stage in the process flow.
type StageDef struct {
	ID         string
	Name       string
	Controller string // PLC placement ID label
	Equipment  []EquipmentDef
}

// EquipmentDef is one piece of equipment in a stage.
type EquipmentDef struct {
	ID          string
	Type        string // "tank", "pump", "valve", etc.
	Label       string
	Era         *int
	Instruments []InstrumentDef
}

// InstrumentDef is one sensor or actuator mapped to a Modbus register.
type InstrumentDef struct {
	Tag          string
	Name         string
	ISAType      string
	Placement    string
	RegisterType string // "holding" or "coil"
	RegisterAddr int
	Unit         string
	RangeMin     float64
	RangeMax     float64
	ScaleMin     float64
	ScaleMax     float64
	Thresholds   *ThresholdDef
}

// ThresholdDef holds optional alarm thresholds for color coding.
// Supports bidirectional thresholds per SOW-021.0.
type ThresholdDef struct {
	Warning    *float64
	Alarm      *float64
	WarningLow *float64
	AlarmLow   *float64
}

// ConnectionDef is a physical process connection between two equipment items.
type ConnectionDef struct {
	From  string
	To    string
	Type  string // "pipe" or "duct"
	Label string
}

// NetworkContextDef is a security-relevant network element for the callout box.
type NetworkContextDef struct {
	ID        string
	Type      string
	Label     string
	Era       *int
	Placement string
	Warning   string
	Notes     string
}

// ProcessViewData is the top-level data passed to SVG templates.
type ProcessViewData struct {
	EnvID          string
	EnvName        string
	FlowDirection  string
	ViewBox        ViewBox
	Stages         []StageLayout
	Connections    []ConnectionPath
	NetworkContext *NetworkContextLayout
	Defs           SVGDefs
}

// ViewBox defines the SVG coordinate space.
type ViewBox struct {
	X, Y, Width, Height float64
}

// String returns the SVG viewBox attribute value string.
func (v ViewBox) String() string {
	return fmt.Sprintf("%.0f %.0f %.0f %.0f", v.X, v.Y, v.Width, v.Height)
}

// StageLayout is a positioned stage group.
type StageLayout struct {
	ID         string
	Name       string
	Controller string
	X, Y       float64
	Width      float64
	Height     float64
	Equipment  []EquipmentLayout
}

// EquipmentLayout is a positioned equipment symbol.
type EquipmentLayout struct {
	ID          string
	Type        string
	Label       string
	X, Y        float64 // center position
	Width       float64
	Height      float64
	Era         *int
	EraClass    string
	Instruments []InstrumentLayout
}

// InstrumentLayout is a positioned instrument value display.
type InstrumentLayout struct {
	Tag          string
	Name         string
	ISAType      string
	Placement    string
	RegisterType string
	RegisterAddr int
	Unit         string
	Range        [2]float64
	Scale        [2]float64
	X, Y         float64 // SVG position for value text
	// Live data -- populated during render, not layout.
	Value       string
	RawValue    uint16
	CoilState   *bool
	StatusColor string
}

// ConnectionPath is an SVG path between two equipment items.
type ConnectionPath struct {
	FromID   string
	ToID     string
	Type     string // "pipe" or "duct"
	Label    string
	PathData string // SVG path d="" attribute
	ArrowEnd bool
}

// NetworkContextLayout is the positioned callout box for network context elements.
type NetworkContextLayout struct {
	X, Y   float64
	Width  float64
	Height float64
	Items  []NetworkContextItem
}

// NetworkContextItem is one security-relevant element in the callout box.
type NetworkContextItem struct {
	ID       string
	Label    string
	Type     string
	Era      *int
	EraClass string
	Warning  string
	Notes    string
}

// SVGDefs holds reusable SVG definitions included in the <defs> section.
type SVGDefs struct {
	ArrowMarkerID    string // ID of the endpoint arrowhead marker
	MidArrowMarkerID string // ID of the mid-pipe arrowhead marker
}

// equipmentSize returns the canonical width and height for a given equipment type.
// Sizes match the SVG symbol templates in process_symbols.html.
func equipmentSize(equipType string) (width, height float64) {
	switch equipType {
	case "tank":
		return 80, 100
	case "basin":
		return 120, 80
	case "pump":
		return 50, 50
	case "valve":
		return 40, 30
	case "blower":
		return 50, 50
	case "compressor":
		return 80, 60
	case "clarifier":
		return 80, 80
	case "screen":
		return 40, 80
	case "uv_system":
		return 60, 40
	case "meter":
		return 50, 50
	case "chromatograph":
		return 60, 40
	case "analyzer_station":
		return 60, 60
	case "heat_exchanger":
		return 60, 50
	default:
		return 60, 60
	}
}

// ThresholdColor returns the SVG fill color for a value given its threshold configuration.
// Priority order: alarm (high) > alarm (low) > warning (high) > warning (low) > normal.
// SIMULATOR CONVENTION: Threshold-driven text colors are used for IT engineer clarity.
// Real DCS/SCADA systems implement alarm color in the operator station software.
func ThresholdColor(value float64, t *ThresholdDef) string {
	if t == nil {
		return "#28a745" // green: no thresholds defined
	}
	if t.Alarm != nil && value >= *t.Alarm {
		return "#dc3545" // red: high alarm
	}
	if t.AlarmLow != nil && value <= *t.AlarmLow {
		return "#dc3545" // red: low alarm
	}
	if t.Warning != nil && value >= *t.Warning {
		return "#ffc107" // yellow: high warning
	}
	if t.WarningLow != nil && value <= *t.WarningLow {
		return "#ffc107" // yellow: low warning
	}
	return "#28a745" // green: normal
}

// statusColors maps device status strings to Bootstrap-compatible SVG fill colors.
// SIMULATOR CONVENTION: These fill colors are used on pump, valve, blower, and
// compressor symbols for IT engineer clarity. Real P&IDs use separate status indicator
// glyphs or position indicators per ISO 1219, not fill color changes on the symbol.
var statusColors = map[string]string{
	"running": "#28a745", // Bootstrap success green
	"stopped": "#6c757d", // Bootstrap secondary grey
	"open":    "#28a745",
	"closed":  "#6c757d",
	"alarm":   "#dc3545", // Bootstrap danger red
	"trip":    "#dc3545",
}

// StatusColor returns the SVG fill color for a device status string.
// Returns grey for unknown status values.
func StatusColor(status string) string {
	if c, ok := statusColors[status]; ok {
		return c
	}
	return "#6c757d"
}

// CoilStatusColor returns the fill color for a coil register state.
// true = running/open (green), false = stopped/closed (grey).
func CoilStatusColor(state bool) string {
	if state {
		return statusColors["running"]
	}
	return statusColors["stopped"]
}

// FormatInstrumentValue formats a raw uint16 register value as a display string.
// Linear scaling: raw 0 -> rangeMin, raw 65535 -> rangeMax.
func FormatInstrumentValue(raw uint16, rangeMin, rangeMax float64, unit string) string {
	scaled := rangeMin + (float64(raw)/65535.0)*(rangeMax-rangeMin)
	if unit != "" {
		return fmt.Sprintf("%.1f %s", scaled, unit)
	}
	return fmt.Sprintf("%.1f", scaled)
}
