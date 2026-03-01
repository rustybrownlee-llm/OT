package process

import (
	"testing"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// makeIntakeStore creates a RegisterStore for the water-intake variant.
// Pump speed InitValue is 0 so ramp tests start from a known zero-flow state.
func makeIntakeStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "wt-plc-01",
		DeviceID:    "compactlogix-l33er",
		Addressing:  "zero-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 0, Name: "intake_flow_rate", Unit: "L/s", ScaleMin: 0, ScaleMax: 100, Writable: false, InitValue: 0},
			{Address: 1, Name: "intake_pump_speed", Unit: "%", ScaleMin: 0, ScaleMax: 100, Writable: true, InitValue: 0},
			{Address: 2, Name: "raw_water_ph", Unit: "pH", ScaleMin: 0, ScaleMax: 14, Writable: false, InitValue: 16383},
			{Address: 3, Name: "raw_water_turbidity", Unit: "NTU", ScaleMin: 0, ScaleMax: 100, Writable: false, InitValue: 16383},
			{Address: 4, Name: "intake_water_temp", Unit: "degC", ScaleMin: 0, ScaleMax: 40, Writable: false, InitValue: 16383},
		},
		Coils: []device.CoilEntry{
			{Address: 0, Name: "intake_pump_01_run", Writable: true},
			{Address: 1, Name: "intake_pump_02_run", Writable: true},
			{Address: 2, Name: "screen_wash_active", Writable: false},
			{Address: 3, Name: "low_well_level_alarm", Writable: false},
		},
	})
}

// makeTreatmentStore creates a RegisterStore for the water-treatment variant.
func makeTreatmentStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "wt-plc-02",
		DeviceID:    "compactlogix-l33er",
		Addressing:  "zero-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 0, Name: "filter_inlet_pressure", Unit: "kPa", ScaleMin: 0, ScaleMax: 500, Writable: false, InitValue: 16383},
			{Address: 1, Name: "filter_outlet_pressure", Unit: "kPa", ScaleMin: 0, ScaleMax: 500, Writable: false, InitValue: 16383},
			{Address: 2, Name: "filter_differential_pressure", Unit: "kPa", ScaleMin: 0, ScaleMax: 50, Writable: false, InitValue: 0},
			{Address: 3, Name: "uv_intensity", Unit: "mW/cm2", ScaleMin: 0, ScaleMax: 100, Writable: false, InitValue: 16383},
			{Address: 4, Name: "chemical_feed_rate", Unit: "mL/min", ScaleMin: 0, ScaleMax: 500, Writable: true, InitValue: 16383},
			{Address: 5, Name: "chlorine_residual", Unit: "mg/L", ScaleMin: 0, ScaleMax: 5, Writable: false, InitValue: 16383},
			{Address: 6, Name: "turbidity_post_filter", Unit: "NTU", ScaleMin: 0, ScaleMax: 5, Writable: false, InitValue: 0},
		},
		Coils: []device.CoilEntry{
			{Address: 0, Name: "filter_backwash_command", Writable: true},
			{Address: 1, Name: "chemical_feed_pump_run", Writable: true},
			{Address: 2, Name: "uv_system_active", Writable: false},
			{Address: 3, Name: "high_dp_alarm", Writable: false},
		},
	})
}

// makeDistributionStore creates a RegisterStore for the water-distribution variant.
func makeDistributionStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "wt-plc-03",
		DeviceID:    "compactlogix-l33er",
		Addressing:  "zero-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 0, Name: "clear_well_level", Unit: "%", ScaleMin: 0, ScaleMax: 100, Writable: false, InitValue: 16383},
			{Address: 1, Name: "distribution_flow_rate", Unit: "L/s", ScaleMin: 0, ScaleMax: 150, Writable: false, InitValue: 0},
			{Address: 2, Name: "distribution_pressure", Unit: "kPa", ScaleMin: 0, ScaleMax: 700, Writable: false, InitValue: 16383},
			{Address: 3, Name: "residual_chlorine", Unit: "mg/L", ScaleMin: 0, ScaleMax: 5, Writable: false, InitValue: 16383},
			{Address: 4, Name: "distribution_water_temp", Unit: "degC", ScaleMin: 0, ScaleMax: 40, Writable: false, InitValue: 16383},
		},
		Coils: []device.CoilEntry{
			{Address: 0, Name: "distribution_pump_01_run", Writable: true},
			{Address: 1, Name: "distribution_pump_02_run", Writable: true},
			{Address: 2, Name: "booster_pump_run", Writable: true},
		},
	})
}

// readFlowRate reads the intake flow_rate register in engineering units (L/s).
func readFlowRate(t *testing.T, store *mbstore.RegisterStore) float64 {
	t.Helper()
	vals, err := store.ReadHolding(intakeRegFlowRate, 1)
	if err != nil {
		t.Fatalf("ReadHolding flow_rate: %v", err)
	}
	return toEng(vals[0], 0, 100)
}

// readDistFlowRate reads the distribution flow_rate register in engineering units.
func readDistFlowRate(t *testing.T, store *mbstore.RegisterStore) float64 {
	t.Helper()
	vals, err := store.ReadHolding(distRegFlowRate, 1)
	if err != nil {
		t.Fatalf("ReadHolding dist flow_rate: %v", err)
	}
	return toEng(vals[0], 0, 150)
}

// TestIntakeModel_PumpOn_FlowIncreasesOverFiveTicks verifies the pump ramp: when pump is
// switched ON, flow increases gradually and reaches a steady value within 5 ticks.
func TestIntakeModel_PumpOn_FlowIncreasesOverFiveTicks(t *testing.T) {
	store := makeIntakeStore()
	model := NewIntakeModel(store, nil)

	// Set pump speed to 100%.
	_ = store.WriteHolding(intakeRegPumpSpeed, []uint16{32767})

	// Enable pump 1.
	_ = store.WriteCoils(intakeCoilPump01, []bool{true})

	// Tick 0: initial flow should be near zero (pump just started).
	initialFlow := readFlowRate(t, store)

	var prevFlow float64
	steadyTick := -1
	for tick := 1; tick <= 5; tick++ {
		model.Tick()
		flow := readFlowRate(t, store)
		if flow <= prevFlow-1.0 && tick > 1 {
			t.Errorf("tick %d: flow decreased unexpectedly from %v to %v", tick, prevFlow, flow)
		}
		if flow >= 45.0 && steadyTick == -1 {
			steadyTick = tick
		}
		prevFlow = flow
	}

	finalFlow := readFlowRate(t, store)
	if finalFlow <= initialFlow {
		t.Errorf("flow did not increase: initial=%v final=%v", initialFlow, finalFlow)
	}
	if finalFlow < 40.0 {
		t.Errorf("flow too low after 5 ticks: got %v, want >= 40 L/s", finalFlow)
	}
}

// TestIntakeModel_PumpOff_FlowDecreasesOverFiveTicks verifies ramp-down: flow returns to 0.
func TestIntakeModel_PumpOff_FlowDecreasesOverFiveTicks(t *testing.T) {
	store := makeIntakeStore()
	model := NewIntakeModel(store, nil)

	// Start from fully ramped up state.
	_ = store.WriteHolding(intakeRegPumpSpeed, []uint16{32767})
	_ = store.WriteCoils(intakeCoilPump01, []bool{true})
	for i := 0; i < 10; i++ {
		model.Tick()
	}
	highFlow := readFlowRate(t, store)
	if highFlow < 40.0 {
		t.Fatalf("setup: expected high flow before ramp-down, got %v", highFlow)
	}

	// Turn pump off.
	_ = store.WriteCoils(intakeCoilPump01, []bool{false})

	// Tick through ramp-down.
	var prevFlow float64
	prevFlow = highFlow
	for tick := 1; tick <= 7; tick++ {
		model.Tick()
		flow := readFlowRate(t, store)
		if flow > prevFlow+1.0 {
			t.Errorf("tick %d: flow increased during ramp-down from %v to %v", tick, prevFlow, flow)
		}
		prevFlow = flow
	}

	finalFlow := readFlowRate(t, store)
	if finalFlow > 5.0 {
		t.Errorf("flow did not decrease after 7 ticks: got %v, want < 5 L/s", finalFlow)
	}
}

// TestTreatmentModel_ChlorineDecaysWhenFeedPumpOff verifies chlorine decay without dosing.
// The model initializes chlorine at 1.0 mg/L and decays at 0.001/tick.
// After 500 ticks the value should be ~0.5 mg/L (meaningful, measurable decay).
func TestTreatmentModel_ChlorineDecaysWhenFeedPumpOff(t *testing.T) {
	store := makeTreatmentStore()
	model := NewTreatmentModel(store, nil)

	// Feed pump is off (default false). Chlorine decays at 0.001/tick.
	initialChl := model.chlorineLevel

	for i := 0; i < 500; i++ {
		model.Tick()
	}

	finalRaw, _ := store.ReadHolding(treatRegChlorineResidual, 1)
	final := toEng(finalRaw[0], 0, 5)

	if final >= initialChl {
		t.Errorf("chlorine did not decay: initial=%v final=%v", initialChl, final)
	}
	// After 500 ticks at 0.001/tick, should have decayed ~0.5 mg/L from initial 1.0.
	if final > initialChl-0.3 {
		t.Errorf("chlorine decay too slow: initial=%v final=%v after 500 ticks", initialChl, final)
	}
}

// TestTreatmentModel_HighDP_AlarmWithHysteresis verifies alarm triggers at 25 kPa
// and clears at 15 kPa (hysteresis).
func TestTreatmentModel_HighDP_AlarmWithHysteresis(t *testing.T) {
	store := makeTreatmentStore()
	model := NewTreatmentModel(store, nil)

	// Drive fouling high to trigger alarm.
	model.filterFouling = 0.5 // produces ~25 kPa DP

	// Tick until alarm fires.
	fired := false
	for i := 0; i < 10; i++ {
		model.Tick()
		coils, _ := store.ReadCoils(treatCoilHighDP, 1)
		if coils[0] {
			fired = true
			break
		}
	}
	if !fired {
		t.Error("high_dp_alarm did not fire with fouling=0.5")
	}

	// Reduce fouling below hysteresis threshold.
	model.filterFouling = 0.0
	model.highDPLatched = true // ensure it was set

	// Tick until alarm clears.
	cleared := false
	for i := 0; i < 10; i++ {
		model.Tick()
		coils, _ := store.ReadCoils(treatCoilHighDP, 1)
		if !coils[0] {
			cleared = true
			break
		}
	}
	if !cleared {
		t.Error("high_dp_alarm did not clear after fouling reset to 0")
	}
}

// TestTreatmentModel_Backwash_ResetsFouling verifies filter fouling decreases during backwash.
func TestTreatmentModel_Backwash_ResetsFouling(t *testing.T) {
	store := makeTreatmentStore()
	model := NewTreatmentModel(store, nil)

	// Set significant fouling.
	model.filterFouling = 0.6

	// Trigger backwash.
	_ = store.WriteCoils(treatCoilBackwash, []bool{true})

	foulingBefore := model.filterFouling
	for i := 0; i < 20; i++ {
		model.Tick()
	}

	if model.filterFouling >= foulingBefore {
		t.Errorf("backwash did not reduce fouling: before=%v after=%v", foulingBefore, model.filterFouling)
	}
	if model.filterFouling > 0.1 {
		t.Errorf("fouling still high after 20-tick backwash: %v", model.filterFouling)
	}
}

// TestDistributionModel_PumpOn_FlowIncreasesOverFiveTicks verifies distribution pump ramp.
func TestDistributionModel_PumpOn_FlowIncreasesOverFiveTicks(t *testing.T) {
	store := makeDistributionStore()
	model := NewDistributionModel(store, nil)

	_ = store.WriteCoils(distCoilPump01, []bool{true})

	initial := readDistFlowRate(t, store)

	for tick := 1; tick <= 5; tick++ {
		model.Tick()
	}

	final := readDistFlowRate(t, store)
	if final <= initial {
		t.Errorf("distribution flow did not increase: initial=%v final=%v", initial, final)
	}
	if final < 50.0 {
		t.Errorf("distribution flow too low after 5 ticks: got %v, want >= 50 L/s", final)
	}
}

// TestDistributionModel_PressureDecreaseWithFlow verifies the pump curve: higher flow means
// lower pressure (centrifugal pump characteristic).
func TestDistributionModel_PressureDecreaseWithFlow(t *testing.T) {
	store := makeDistributionStore()
	model := NewDistributionModel(store, nil)

	// No pumps: pressure at deadhead (near 500 kPa).
	for i := 0; i < 3; i++ {
		model.Tick()
	}
	lowFlowPressRaw, _ := store.ReadHolding(distRegPressure, 1)
	lowFlowPress := toEng(lowFlowPressRaw[0], 0, 700)

	// Turn on both pumps.
	_ = store.WriteCoils(distCoilPump01, []bool{true})
	_ = store.WriteCoils(distCoilPump02, []bool{true})
	for i := 0; i < 7; i++ {
		model.Tick()
	}

	highFlowPressRaw, _ := store.ReadHolding(distRegPressure, 1)
	highFlowPress := toEng(highFlowPressRaw[0], 0, 700)

	if highFlowPress >= lowFlowPress {
		t.Errorf("pressure should decrease with flow: no-pump press=%v, two-pump press=%v",
			lowFlowPress, highFlowPress)
	}
}
