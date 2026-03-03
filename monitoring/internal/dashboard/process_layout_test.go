package dashboard_test

// process_layout_test.go - Unit tests for the SVG process view layout engine.
// Tests verify deterministic coordinate computation for the horizontal layout algorithm.
// All tests use table-driven or subtests where appropriate for clarity.

import (
	"strings"
	"testing"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/dashboard"
)

// ---- Test helpers ----

// makeStage creates a StageDef with N equipment items of a given type.
func makeStage(id string, equipTypes ...string) dashboard.StageDef {
	equip := make([]dashboard.EquipmentDef, len(equipTypes))
	for i, t := range equipTypes {
		equip[i] = dashboard.EquipmentDef{
			ID:    id + "-equip-" + t,
			Type:  t,
			Label: t,
		}
	}
	return dashboard.StageDef{ID: id, Name: id, Equipment: equip}
}

// makeSchematic creates a minimal ProcessSchematic with given stages.
func makeSchematic(stages ...dashboard.StageDef) *dashboard.ProcessSchematic {
	return &dashboard.ProcessSchematic{
		Process: dashboard.ProcessMeta{ID: "test", Name: "Test Process"},
		Stages:  stages,
	}
}

// ---- Phase 5 tests: computeStagePositions ----

// TestComputeProcessLayout_NilSchematic verifies nil input returns nil.
func TestComputeProcessLayout_NilSchematic(t *testing.T) {
	pv := dashboard.ComputeProcessLayout(nil, "env1", "Env 1")
	if pv != nil {
		t.Error("ComputeProcessLayout(nil) should return nil")
	}
}

// TestComputeProcessLayout_SingleStage verifies basic layout with one stage.
func TestComputeProcessLayout_SingleStage(t *testing.T) {
	sc := makeSchematic(makeStage("intake", "pump", "valve"))
	pv := dashboard.ComputeProcessLayout(sc, "test-env", "Test Env")

	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil for valid schematic")
	}
	if pv.EnvID != "test-env" {
		t.Errorf("EnvID: got %q, want %q", pv.EnvID, "test-env")
	}
	if len(pv.Stages) != 1 {
		t.Fatalf("Stages: got %d, want 1", len(pv.Stages))
	}
	if len(pv.Stages[0].Equipment) != 2 {
		t.Errorf("Equipment: got %d, want 2", len(pv.Stages[0].Equipment))
	}
}

// TestComputeStagePositions_Spacing verifies stage X positions are spaced 280px apart.
func TestComputeStagePositions_Spacing(t *testing.T) {
	sc := makeSchematic(
		makeStage("stage-a", "pump"),
		makeStage("stage-b", "valve"),
		makeStage("stage-c", "tank"),
	)
	pv := dashboard.ComputeProcessLayout(sc, "env", "Env")
	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}
	if len(pv.Stages) != 3 {
		t.Fatalf("Stages: got %d, want 3", len(pv.Stages))
	}

	// Stage X positions should increase by 280px each.
	const stageSpacing = 280.0
	for i := 1; i < len(pv.Stages); i++ {
		diff := pv.Stages[i].X - pv.Stages[i-1].X
		if diff < stageSpacing-1 || diff > stageSpacing+1 {
			t.Errorf("Stages[%d].X - Stages[%d].X: got %.1f, want ~%.1f",
				i, i-1, diff, stageSpacing)
		}
	}
}

// TestComputeEquipmentPositions_VerticalSpacing verifies equipment within a stage
// are spaced 120px apart vertically.
func TestComputeEquipmentPositions_VerticalSpacing(t *testing.T) {
	sc := makeSchematic(makeStage("s1", "pump", "valve", "meter"))
	pv := dashboard.ComputeProcessLayout(sc, "env", "Env")
	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}
	equip := pv.Stages[0].Equipment
	if len(equip) != 3 {
		t.Fatalf("Equipment: got %d, want 3", len(equip))
	}

	const equipSpacing = 120.0
	for i := 1; i < len(equip); i++ {
		diff := equip[i].Y - equip[i-1].Y
		if diff < equipSpacing-1 || diff > equipSpacing+1 {
			t.Errorf("Equipment[%d].Y - Equipment[%d].Y: got %.1f, want ~%.1f",
				i, i-1, diff, equipSpacing)
		}
	}
}

// TestComputeEquipmentPositions_CenterX verifies equipment X is the stage center.
func TestComputeEquipmentPositions_CenterX(t *testing.T) {
	sc := makeSchematic(makeStage("s1", "pump", "tank", "valve"))
	pv := dashboard.ComputeProcessLayout(sc, "env", "Env")
	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}
	equip := pv.Stages[0].Equipment

	// All equipment in a stage should share the same X center.
	for i := 1; i < len(equip); i++ {
		if equip[i].X != equip[0].X {
			t.Errorf("Equipment[%d].X = %.1f, Equipment[0].X = %.1f: all should share stage center X",
				i, equip[i].X, equip[0].X)
		}
	}
}

// TestComputeInstrumentPositions_CalloutOffset verifies instrument X is offset right from equipment.
func TestComputeInstrumentPositions_CalloutOffset(t *testing.T) {
	era := 2015
	sc := &dashboard.ProcessSchematic{
		Process: dashboard.ProcessMeta{ID: "test"},
		Stages: []dashboard.StageDef{
			{
				ID:   "s1",
				Name: "Stage 1",
				Equipment: []dashboard.EquipmentDef{
					{
						ID:   "pump-01",
						Type: "pump",
						Era:  &era,
						Instruments: []dashboard.InstrumentDef{
							{Tag: "FT-101", Name: "Flow", ISAType: "FT",
								RegisterType: "holding", RegisterAddr: 0,
								Unit: "GPM", RangeMin: 0, RangeMax: 500},
							{Tag: "PT-101", Name: "Pressure", ISAType: "PT",
								RegisterType: "holding", RegisterAddr: 1,
								Unit: "PSI", RangeMin: 0, RangeMax: 100},
						},
					},
				},
			},
		},
	}
	pv := dashboard.ComputeProcessLayout(sc, "env", "Env")
	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}
	equip := pv.Stages[0].Equipment[0]
	if len(equip.Instruments) != 2 {
		t.Fatalf("Instruments: got %d, want 2", len(equip.Instruments))
	}

	const instrumentOffsetX = 150.0
	for _, inst := range equip.Instruments {
		expectedX := equip.X + instrumentOffsetX
		if inst.X < expectedX-1 || inst.X > expectedX+1 {
			t.Errorf("Instrument %q X: got %.1f, want ~%.1f",
				inst.Tag, inst.X, expectedX)
		}
	}
}

// TestComputeInstrumentPositions_StackedY verifies instruments stack vertically at 18px.
func TestComputeInstrumentPositions_StackedY(t *testing.T) {
	sc := &dashboard.ProcessSchematic{
		Process: dashboard.ProcessMeta{ID: "test"},
		Stages: []dashboard.StageDef{
			{
				ID:   "s1",
				Name: "Stage 1",
				Equipment: []dashboard.EquipmentDef{
					{
						ID:   "tank-01",
						Type: "tank",
						Instruments: []dashboard.InstrumentDef{
							{Tag: "LT-101", ISAType: "LT", RegisterType: "holding", RegisterAddr: 0, Unit: "%"},
							{Tag: "AT-101", ISAType: "AT", RegisterType: "holding", RegisterAddr: 1, Unit: "pH"},
							{Tag: "TT-101", ISAType: "TT", RegisterType: "holding", RegisterAddr: 2, Unit: "C"},
						},
					},
				},
			},
		},
	}
	pv := dashboard.ComputeProcessLayout(sc, "env", "Env")
	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}
	instr := pv.Stages[0].Equipment[0].Instruments
	if len(instr) != 3 {
		t.Fatalf("Instruments: got %d, want 3", len(instr))
	}

	const lineSpacing = 18.0
	for i := 1; i < len(instr); i++ {
		diff := instr[i].Y - instr[i-1].Y
		if diff < lineSpacing-1 || diff > lineSpacing+1 {
			t.Errorf("Instrument[%d].Y - Instrument[%d].Y: got %.1f, want ~%.1f",
				i, i-1, diff, lineSpacing)
		}
	}
}

// TestComputeConnectionPaths_PathData verifies connection paths start with valid SVG M command.
func TestComputeConnectionPaths_PathData(t *testing.T) {
	sc := &dashboard.ProcessSchematic{
		Process: dashboard.ProcessMeta{ID: "test"},
		Stages: []dashboard.StageDef{
			{ID: "s1", Name: "Stage 1", Equipment: []dashboard.EquipmentDef{
				{ID: "pump-01", Type: "pump", Label: "Intake Pump"},
			}},
			{ID: "s2", Name: "Stage 2", Equipment: []dashboard.EquipmentDef{
				{ID: "tank-01", Type: "tank", Label: "Storage Tank"},
			}},
		},
		Connections: []dashboard.ConnectionDef{
			{From: "pump-01", To: "tank-01", Type: "pipe", Label: "Raw water"},
		},
	}
	pv := dashboard.ComputeProcessLayout(sc, "env", "Env")
	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}
	if len(pv.Connections) != 1 {
		t.Fatalf("Connections: got %d, want 1", len(pv.Connections))
	}
	conn := pv.Connections[0]
	if !strings.HasPrefix(conn.PathData, "M ") {
		t.Errorf("PathData should start with 'M ': got %q", conn.PathData)
	}
	if !strings.Contains(conn.PathData, "L ") {
		t.Errorf("PathData should contain 'L ': got %q", conn.PathData)
	}
	if !conn.ArrowEnd {
		t.Error("pipe connection should have ArrowEnd=true")
	}
}

// TestComputeConnectionPaths_MissingEndpoint verifies missing equipment is skipped.
func TestComputeConnectionPaths_MissingEndpoint(t *testing.T) {
	sc := &dashboard.ProcessSchematic{
		Process: dashboard.ProcessMeta{ID: "test"},
		Stages: []dashboard.StageDef{
			{ID: "s1", Name: "Stage 1", Equipment: []dashboard.EquipmentDef{
				{ID: "pump-01", Type: "pump"},
			}},
		},
		Connections: []dashboard.ConnectionDef{
			{From: "pump-01", To: "nonexistent-01", Type: "pipe"},
		},
	}
	pv := dashboard.ComputeProcessLayout(sc, "env", "Env")
	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}
	// Connection to missing equipment should be silently dropped.
	if len(pv.Connections) != 0 {
		t.Errorf("Connections with missing endpoint: got %d, want 0", len(pv.Connections))
	}
}

// TestComputeViewBox_SingleStage verifies viewBox covers a single stage with margin.
func TestComputeViewBox_SingleStage(t *testing.T) {
	sc := makeSchematic(makeStage("s1", "pump"))
	pv := dashboard.ComputeProcessLayout(sc, "env", "Env")
	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}

	// ViewBox should have positive width and height.
	if pv.ViewBox.Width <= 0 {
		t.Errorf("ViewBox.Width: got %.1f, want > 0", pv.ViewBox.Width)
	}
	if pv.ViewBox.Height <= 0 {
		t.Errorf("ViewBox.Height: got %.1f, want > 0", pv.ViewBox.Height)
	}
	// ViewBox should include margin (at least 40px).
	if pv.ViewBox.X > -10 {
		t.Errorf("ViewBox.X: got %.1f, expected negative (margin from content edge)", pv.ViewBox.X)
	}
}

// TestComputeViewBox_ThreeStages verifies viewBox grows with more stages.
func TestComputeViewBox_ThreeStages(t *testing.T) {
	sc1 := makeSchematic(makeStage("s1", "pump"))
	sc3 := makeSchematic(makeStage("s1", "pump"), makeStage("s2", "tank"), makeStage("s3", "valve"))

	pv1 := dashboard.ComputeProcessLayout(sc1, "env", "Env")
	pv3 := dashboard.ComputeProcessLayout(sc3, "env", "Env")
	if pv1 == nil || pv3 == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}

	if pv3.ViewBox.Width <= pv1.ViewBox.Width {
		t.Errorf("3-stage ViewBox.Width (%.1f) should be > 1-stage (%.1f)",
			pv3.ViewBox.Width, pv1.ViewBox.Width)
	}
}

// TestComputeViewBox_Deterministic verifies same input always produces same viewBox.
func TestComputeViewBox_Deterministic(t *testing.T) {
	sc := makeSchematic(makeStage("s1", "pump", "tank"), makeStage("s2", "valve"))

	pv1 := dashboard.ComputeProcessLayout(sc, "env", "Env")
	pv2 := dashboard.ComputeProcessLayout(sc, "env", "Env")
	if pv1 == nil || pv2 == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}

	if pv1.ViewBox != pv2.ViewBox {
		t.Errorf("Layout is not deterministic: run1=%v, run2=%v", pv1.ViewBox, pv2.ViewBox)
	}
	if len(pv1.Stages) != len(pv2.Stages) {
		t.Error("Stage count differs between runs")
	}
}

// TestComputeNetworkContextBox_Positioning verifies context box is below stage content.
func TestComputeNetworkContextBox_Positioning(t *testing.T) {
	sc := &dashboard.ProcessSchematic{
		Process: dashboard.ProcessMeta{ID: "test"},
		Stages:  []dashboard.StageDef{makeStage("s1", "pump")},
		NetworkContext: []dashboard.NetworkContextDef{
			{ID: "wan-01", Type: "wan_link", Label: "Internet Gateway", Warning: "Exposed to WAN"},
		},
	}
	pv := dashboard.ComputeProcessLayout(sc, "env", "Env")
	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}
	if pv.NetworkContext == nil {
		t.Fatal("NetworkContext should not be nil when schematic has network_context items")
	}
	if len(pv.NetworkContext.Items) != 1 {
		t.Errorf("NetworkContext.Items: got %d, want 1", len(pv.NetworkContext.Items))
	}
	// Context box Y should be below all stages.
	maxStageBottom := 0.0
	for _, s := range pv.Stages {
		bottom := s.Y + s.Height
		if bottom > maxStageBottom {
			maxStageBottom = bottom
		}
	}
	if pv.NetworkContext.Y <= maxStageBottom {
		t.Errorf("NetworkContext.Y (%.1f) should be below stage bottom (%.1f)",
			pv.NetworkContext.Y, maxStageBottom)
	}
}

// TestComputeNetworkContextBox_EmptyItems verifies nil is returned when no items.
func TestComputeNetworkContextBox_EmptyItems(t *testing.T) {
	sc := makeSchematic(makeStage("s1", "pump"))
	pv := dashboard.ComputeProcessLayout(sc, "env", "Env")
	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}
	if pv.NetworkContext != nil {
		t.Error("NetworkContext should be nil when schematic has no network_context items")
	}
}

// ---- Threshold color tests ----

// TestThresholdColor_Normal verifies green is returned when value is in normal range.
func TestThresholdColor_Normal(t *testing.T) {
	warning := 80.0
	alarm := 90.0
	td := &dashboard.ThresholdDef{Warning: &warning, Alarm: &alarm}

	got := dashboard.ThresholdColor(50.0, td)
	if got != "#28a745" {
		t.Errorf("ThresholdColor(50.0, normal range): got %q, want %q", got, "#28a745")
	}
}

// TestThresholdColor_Warning verifies yellow for value at or above warning threshold.
func TestThresholdColor_Warning(t *testing.T) {
	warning := 80.0
	alarm := 90.0
	td := &dashboard.ThresholdDef{Warning: &warning, Alarm: &alarm}

	got := dashboard.ThresholdColor(80.0, td)
	if got != "#ffc107" {
		t.Errorf("ThresholdColor(80.0, warning=80): got %q, want %q", got, "#ffc107")
	}
}

// TestThresholdColor_Alarm verifies red for value at or above alarm threshold.
func TestThresholdColor_Alarm(t *testing.T) {
	warning := 80.0
	alarm := 90.0
	td := &dashboard.ThresholdDef{Warning: &warning, Alarm: &alarm}

	got := dashboard.ThresholdColor(95.0, td)
	if got != "#dc3545" {
		t.Errorf("ThresholdColor(95.0, alarm=90): got %q, want %q", got, "#dc3545")
	}
}

// TestThresholdColor_WarningLow verifies yellow for value at or below warning_low threshold.
func TestThresholdColor_WarningLow(t *testing.T) {
	warningLow := 20.0
	alarmLow := 10.0
	td := &dashboard.ThresholdDef{WarningLow: &warningLow, AlarmLow: &alarmLow}

	got := dashboard.ThresholdColor(20.0, td)
	if got != "#ffc107" {
		t.Errorf("ThresholdColor(20.0, warning_low=20): got %q, want %q", got, "#ffc107")
	}
}

// TestThresholdColor_AlarmLow verifies red for value at or below alarm_low threshold.
func TestThresholdColor_AlarmLow(t *testing.T) {
	warningLow := 20.0
	alarmLow := 10.0
	td := &dashboard.ThresholdDef{WarningLow: &warningLow, AlarmLow: &alarmLow}

	got := dashboard.ThresholdColor(5.0, td)
	if got != "#dc3545" {
		t.Errorf("ThresholdColor(5.0, alarm_low=10): got %q, want %q", got, "#dc3545")
	}
}

// TestThresholdColor_NilThresholds verifies green is returned when no thresholds defined.
func TestThresholdColor_NilThresholds(t *testing.T) {
	got := dashboard.ThresholdColor(50.0, nil)
	if got != "#28a745" {
		t.Errorf("ThresholdColor(50.0, nil): got %q, want %q", got, "#28a745")
	}
}

// TestThresholdColor_AlarmBeatsWarning verifies alarm takes priority over warning.
func TestThresholdColor_AlarmBeatsWarning(t *testing.T) {
	warning := 80.0
	alarm := 80.0 // same threshold -- alarm wins
	td := &dashboard.ThresholdDef{Warning: &warning, Alarm: &alarm}

	got := dashboard.ThresholdColor(80.0, td)
	if got != "#dc3545" {
		t.Errorf("ThresholdColor alarm at same level as warning: got %q, want red", got)
	}
}

// ---- Value formatting tests ----

// TestFormatInstrumentValue_ZeroRaw verifies zero raw maps to rangeMin.
func TestFormatInstrumentValue_ZeroRaw(t *testing.T) {
	got := dashboard.FormatInstrumentValue(0, 0.0, 100.0, "%")
	// 0 raw -> 0.0%
	if got != "0.0 %" {
		t.Errorf("FormatInstrumentValue(0, 0, 100, %%): got %q, want %q", got, "0.0 %")
	}
}

// TestFormatInstrumentValue_MaxRaw verifies max raw maps to rangeMax.
func TestFormatInstrumentValue_MaxRaw(t *testing.T) {
	got := dashboard.FormatInstrumentValue(65535, 0.0, 100.0, "%")
	// 65535 raw -> 100.0%
	if got != "100.0 %" {
		t.Errorf("FormatInstrumentValue(65535, 0, 100, %%): got %q, want %q", got, "100.0 %")
	}
}

// TestFormatInstrumentValue_NoUnit verifies format without unit.
func TestFormatInstrumentValue_NoUnit(t *testing.T) {
	got := dashboard.FormatInstrumentValue(0, 0.0, 7.0, "")
	if got != "0.0" {
		t.Errorf("FormatInstrumentValue(0, 0, 7, \"\"): got %q, want %q", got, "0.0")
	}
}

// TestFormatInstrumentValue_PHRange verifies scaling over a pH range.
func TestFormatInstrumentValue_PHRange(t *testing.T) {
	// pH range 0-14, midpoint should be ~7.0
	got := dashboard.FormatInstrumentValue(32767, 0.0, 14.0, "pH")
	// 32767/65535 * 14 ~ 7.0
	if !strings.Contains(got, "7.") {
		t.Errorf("FormatInstrumentValue midpoint pH: got %q, expected value around 7.x pH", got)
	}
}

// ---- PipeLength and NeedsMidArrow tests ----

// TestPipeLength_Horizontal verifies length calculation for horizontal pipes.
func TestPipeLength_Horizontal(t *testing.T) {
	got := dashboard.PipeLength(0, 0, 300, 0)
	if got < 299 || got > 301 {
		t.Errorf("PipeLength(0,0,300,0): got %.1f, want 300.0", got)
	}
}

// TestNeedsMidArrow_LongPipe verifies mid-arrow is needed for pipes > 300px.
func TestNeedsMidArrow_LongPipe(t *testing.T) {
	if !dashboard.NeedsMidArrow(0, 0, 400, 0) {
		t.Error("NeedsMidArrow for 400px pipe: should be true")
	}
}

// TestNeedsMidArrow_ShortPipe verifies mid-arrow is not needed for pipes <= 300px.
func TestNeedsMidArrow_ShortPipe(t *testing.T) {
	if dashboard.NeedsMidArrow(0, 0, 200, 0) {
		t.Error("NeedsMidArrow for 200px pipe: should be false")
	}
}

// TestMidPoint_Horizontal verifies midpoint of a horizontal segment.
func TestMidPoint_Horizontal(t *testing.T) {
	mx, my := dashboard.MidPoint(0, 0, 200, 0)
	if mx < 99 || mx > 101 {
		t.Errorf("MidPoint X: got %.1f, want 100.0", mx)
	}
	if my != 0 {
		t.Errorf("MidPoint Y: got %.1f, want 0.0", my)
	}
}

// ---- CoilStatusColor tests ----

// TestCoilStatusColor_True verifies green for active coil.
func TestCoilStatusColor_True(t *testing.T) {
	got := dashboard.CoilStatusColor(true)
	if got != "#28a745" {
		t.Errorf("CoilStatusColor(true): got %q, want green #28a745", got)
	}
}

// TestCoilStatusColor_False verifies grey for inactive coil.
func TestCoilStatusColor_False(t *testing.T) {
	got := dashboard.CoilStatusColor(false)
	if got != "#6c757d" {
		t.Errorf("CoilStatusColor(false): got %q, want grey #6c757d", got)
	}
}

// TestStatusColor_KnownValues verifies all documented status strings return correct colors.
func TestStatusColor_KnownValues(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"running", "#28a745"},
		{"stopped", "#6c757d"},
		{"open", "#28a745"},
		{"closed", "#6c757d"},
		{"alarm", "#dc3545"},
		{"trip", "#dc3545"},
		{"unknown-status", "#6c757d"}, // default grey
	}
	for _, tc := range tests {
		got := dashboard.StatusColor(tc.status)
		if got != tc.want {
			t.Errorf("StatusColor(%q): got %q, want %q", tc.status, got, tc.want)
		}
	}
}

// ---- Full pipeline test ----

// TestComputeProcessLayout_FullPipeline verifies all fields are populated correctly
// in a realistic 3-stage process schematic.
func TestComputeProcessLayout_FullPipeline(t *testing.T) {
	era2010 := 2010
	era1998 := 1998
	sc := &dashboard.ProcessSchematic{
		Process: dashboard.ProcessMeta{
			ID:            "water-treatment",
			Name:          "Water Treatment",
			FlowDirection: "horizontal",
		},
		Stages: []dashboard.StageDef{
			{
				ID: "intake", Name: "Intake", Controller: "wt-plc-01",
				Equipment: []dashboard.EquipmentDef{
					{ID: "screen-01", Type: "screen", Label: "Bar Screen", Era: &era1998,
						Instruments: []dashboard.InstrumentDef{
							{Tag: "LS-101", ISAType: "LS", RegisterType: "coil", RegisterAddr: 0},
						},
					},
					{ID: "pump-01", Type: "pump", Label: "Intake Pump",
						Instruments: []dashboard.InstrumentDef{
							{Tag: "FT-101", ISAType: "FT", RegisterType: "holding", RegisterAddr: 0,
								Unit: "GPM", RangeMin: 0, RangeMax: 500},
						},
					},
				},
			},
			{
				ID: "treatment", Name: "Treatment", Controller: "wt-plc-02",
				Equipment: []dashboard.EquipmentDef{
					{ID: "basin-01", Type: "basin", Label: "Aeration Basin", Era: &era2010,
						Instruments: []dashboard.InstrumentDef{
							{Tag: "AT-201", ISAType: "AT", RegisterType: "holding", RegisterAddr: 2,
								Unit: "mg/L", RangeMin: 0, RangeMax: 10},
							{Tag: "LT-201", ISAType: "LT", RegisterType: "holding", RegisterAddr: 3,
								Unit: "%", RangeMin: 0, RangeMax: 100},
						},
					},
				},
			},
			{
				ID: "disinfection", Name: "Disinfection",
				Equipment: []dashboard.EquipmentDef{
					{ID: "uv-01", Type: "uv_system", Label: "UV System"},
				},
			},
		},
		Connections: []dashboard.ConnectionDef{
			{From: "pump-01", To: "basin-01", Type: "pipe", Label: "Raw water"},
			{From: "basin-01", To: "uv-01", Type: "pipe", Label: "Treated"},
		},
		NetworkContext: []dashboard.NetworkContextDef{
			{ID: "scada-link", Type: "wan_link", Label: "SCADA WAN Link", Warning: "Internet-facing"},
		},
	}

	pv := dashboard.ComputeProcessLayout(sc, "water-treatment", "Water Treatment Plant")
	if pv == nil {
		t.Fatal("ComputeProcessLayout returned nil")
	}

	// Verify env metadata.
	if pv.EnvID != "water-treatment" {
		t.Errorf("EnvID: got %q", pv.EnvID)
	}
	if pv.FlowDirection != "horizontal" {
		t.Errorf("FlowDirection: got %q", pv.FlowDirection)
	}

	// Verify stage count.
	if len(pv.Stages) != 3 {
		t.Fatalf("Stages: got %d, want 3", len(pv.Stages))
	}

	// Verify controller label on first stage.
	if pv.Stages[0].Controller != "wt-plc-01" {
		t.Errorf("Stage[0].Controller: got %q, want %q", pv.Stages[0].Controller, "wt-plc-01")
	}

	// Verify era class on screen-01.
	screen := pv.Stages[0].Equipment[0]
	if screen.EraClass != "era-1990s" {
		t.Errorf("screen-01 EraClass: got %q, want %q", screen.EraClass, "era-1990s")
	}

	// Verify era class on basin-01.
	basin := pv.Stages[1].Equipment[0]
	if basin.EraClass != "era-2010s" {
		t.Errorf("basin-01 EraClass: got %q, want %q", basin.EraClass, "era-2010s")
	}

	// Verify instrument count.
	if len(pv.Stages[1].Equipment[0].Instruments) != 2 {
		t.Errorf("basin-01 instruments: got %d, want 2",
			len(pv.Stages[1].Equipment[0].Instruments))
	}

	// Verify instrument tags preserved.
	firstInst := pv.Stages[1].Equipment[0].Instruments[0]
	if firstInst.Tag != "AT-201" {
		t.Errorf("basin-01 first instrument tag: got %q, want %q", firstInst.Tag, "AT-201")
	}

	// Verify connections.
	if len(pv.Connections) != 2 {
		t.Errorf("Connections: got %d, want 2", len(pv.Connections))
	}

	// Verify network context.
	if pv.NetworkContext == nil {
		t.Fatal("NetworkContext should not be nil")
	}
	if len(pv.NetworkContext.Items) != 1 {
		t.Errorf("NetworkContext.Items: got %d, want 1", len(pv.NetworkContext.Items))
	}
	if pv.NetworkContext.Items[0].Warning != "Internet-facing" {
		t.Errorf("NetworkContext warning: got %q", pv.NetworkContext.Items[0].Warning)
	}

	// Verify SVG defs are populated.
	if pv.Defs.ArrowMarkerID == "" {
		t.Error("Defs.ArrowMarkerID should not be empty")
	}

	// Verify viewBox is reasonable.
	if pv.ViewBox.Width < 200 {
		t.Errorf("ViewBox.Width: got %.1f, expected >= 200 for 3-stage layout", pv.ViewBox.Width)
	}
}
