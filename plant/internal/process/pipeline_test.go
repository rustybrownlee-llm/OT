package process

import (
	"math"
	"testing"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// ---- Store constructors ----

// makeCompressorStore creates a RegisterStore for the compressor-control variant.
// CompactLogix L33ER, zero-based addressing.
func makeCompressorStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "ps-plc-01",
		DeviceID:    "compactlogix-l33er",
		Addressing:  "zero-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 0, Name: "compressor_speed", Unit: "RPM", ScaleMin: 0, ScaleMax: 6000, Writable: false},
			{Address: 1, Name: "suction_pressure", Unit: "PSIG", ScaleMin: 0, ScaleMax: 1500, Writable: false},
			{Address: 2, Name: "discharge_pressure", Unit: "PSIG", ScaleMin: 0, ScaleMax: 1500, Writable: false},
			{Address: 3, Name: "bearing_temp_drive_end", Unit: "degF", ScaleMin: 0, ScaleMax: 350, Writable: false},
			{Address: 4, Name: "bearing_temp_non_drive_end", Unit: "degF", ScaleMin: 0, ScaleMax: 350, Writable: false},
			{Address: 5, Name: "vibration_overall", Unit: "mils", ScaleMin: 0, ScaleMax: 10, Writable: false},
			{Address: 6, Name: "lube_oil_pressure", Unit: "PSIG", ScaleMin: 0, ScaleMax: 100, Writable: false},
			{Address: 7, Name: "lube_oil_temperature", Unit: "degF", ScaleMin: 0, ScaleMax: 200, Writable: false},
			{Address: 8, Name: "compressor_status_word", Unit: "bitmask", ScaleMin: 0, ScaleMax: 65535, Writable: false},
		},
		Coils: []device.CoilEntry{
			{Address: 0, Name: "compressor_run_cmd", Writable: true},
			{Address: 1, Name: "compressor_e_stop", Writable: false},
			{Address: 2, Name: "surge_protection_active", Writable: false},
			{Address: 3, Name: "compressor_tripped", Writable: false},
		},
	})
}

// makeMeteringStore creates a RegisterStore for the pipeline-metering variant.
// Emerson ROC800, one-based addressing.
func makeMeteringStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "ps-rtu-01",
		DeviceID:    "emerson-roc800",
		Addressing:  "one-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 1, Name: "meter_run_1_flow_rate", Unit: "MSCFH", ScaleMin: 0, ScaleMax: 5000, Writable: false},
			{Address: 2, Name: "meter_run_1_volume_today", Unit: "MCF", ScaleMin: 0, ScaleMax: 100000, Writable: false},
			{Address: 3, Name: "meter_run_1_pressure", Unit: "PSIG", ScaleMin: 0, ScaleMax: 1500, Writable: false},
			{Address: 4, Name: "meter_run_1_temperature", Unit: "degF", ScaleMin: -20, ScaleMax: 150, Writable: false},
			{Address: 5, Name: "meter_run_1_differential_pressure", Unit: "inH2O", ScaleMin: 0, ScaleMax: 200, Writable: false},
			{Address: 6, Name: "station_total_flow", Unit: "MSCFH", ScaleMin: 0, ScaleMax: 20000, Writable: false},
			{Address: 7, Name: "station_total_volume_today", Unit: "MCF", ScaleMin: 0, ScaleMax: 100000, Writable: false},
		},
		Coils: []device.CoilEntry{
			// Coil InitValue is not applied per SOW-002.0 spec; tests must write explicitly.
			{Address: 1, Name: "meter_run_1_enabled", Writable: true},
			{Address: 2, Name: "meter_run_2_enabled", Writable: true},
			{Address: 3, Name: "meter_run_3_enabled", Writable: true},
			{Address: 4, Name: "meter_run_4_enabled", Writable: true},
		},
	})
}

// makeStationStore creates a RegisterStore for the station-monitoring variant.
// Emerson ROC800, one-based addressing.
func makeStationStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "ps-rtu-02",
		DeviceID:    "emerson-roc800",
		Addressing:  "one-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 1, Name: "station_inlet_pressure", Unit: "PSIG", ScaleMin: 0, ScaleMax: 1500, Writable: false},
			{Address: 2, Name: "station_outlet_pressure", Unit: "PSIG", ScaleMin: 0, ScaleMax: 1500, Writable: false},
			{Address: 3, Name: "station_inlet_temperature", Unit: "degF", ScaleMin: -20, ScaleMax: 150, Writable: false},
			{Address: 4, Name: "station_outlet_temperature", Unit: "degF", ScaleMin: -20, ScaleMax: 150, Writable: false},
			{Address: 5, Name: "inlet_block_valve_position", Unit: "%", ScaleMin: 0, ScaleMax: 100, Writable: false},
			{Address: 6, Name: "outlet_block_valve_position", Unit: "%", ScaleMin: 0, ScaleMax: 100, Writable: false},
			{Address: 7, Name: "esd_valve_position", Unit: "%", ScaleMin: 0, ScaleMax: 100, Writable: false},
			{Address: 8, Name: "station_status_word", Unit: "bitmask", ScaleMin: 0, ScaleMax: 65535, Writable: false},
		},
		Coils: []device.CoilEntry{
			{Address: 1, Name: "esd_activate", Writable: false},
			{Address: 2, Name: "inlet_block_valve_cmd", Writable: true, InitValue: true},
			{Address: 3, Name: "outlet_block_valve_cmd", Writable: true, InitValue: true},
			{Address: 4, Name: "esd_active_status", Writable: false},
		},
	})
}

// makeGasStore creates a RegisterStore for the gas-analysis variant.
// ABB TotalFlow G5, one-based addressing.
func makeGasStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "ps-fc-01",
		DeviceID:    "abb-totalflow-g5",
		Addressing:  "one-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 1, Name: "methane_pct", Unit: "%", ScaleMin: 0, ScaleMax: 100, Writable: false},
			{Address: 2, Name: "ethane_pct", Unit: "%", ScaleMin: 0, ScaleMax: 20, Writable: false},
			{Address: 3, Name: "propane_pct", Unit: "%", ScaleMin: 0, ScaleMax: 10, Writable: false},
			{Address: 4, Name: "co2_pct", Unit: "%", ScaleMin: 0, ScaleMax: 5, Writable: false},
			{Address: 5, Name: "nitrogen_pct", Unit: "%", ScaleMin: 0, ScaleMax: 10, Writable: false},
			{Address: 6, Name: "btu_heating_value", Unit: "BTU/SCF", ScaleMin: 900, ScaleMax: 1200, Writable: false},
			{Address: 7, Name: "specific_gravity", Unit: "SG", ScaleMin: 0.50, ScaleMax: 1.00, Writable: false},
			{Address: 8, Name: "moisture_content", Unit: "lb/MMSCF", ScaleMin: 0, ScaleMax: 20, Writable: false},
			{Address: 9, Name: "analysis_cycle_status", Unit: "enum", ScaleMin: 0, ScaleMax: 3, Writable: false},
		},
		Coils: []device.CoilEntry{
			{Address: 1, Name: "analysis_in_progress", Writable: false},
			{Address: 2, Name: "gc_alarm_active", Writable: false},
			{Address: 3, Name: "moisture_alarm_active", Writable: false},
		},
	})
}

// ---- Helper functions ----

// readHoldingEng reads a holding register at addr and converts to engineering units.
func readHoldingEng(t *testing.T, store *mbstore.RegisterStore, addr uint16, min, max float64) float64 {
	t.Helper()
	vals, err := store.ReadHolding(addr, 1)
	if err != nil {
		t.Fatalf("ReadHolding addr %d: %v", addr, err)
	}
	return toEng(vals[0], min, max)
}

// readHoldingRaw reads a raw holding register value.
func readHoldingRaw(t *testing.T, store *mbstore.RegisterStore, addr uint16) uint16 {
	t.Helper()
	vals, err := store.ReadHolding(addr, 1)
	if err != nil {
		t.Fatalf("ReadHolding addr %d: %v", addr, err)
	}
	return vals[0]
}

// readCoil reads a single coil at the given address.
func readCoil(t *testing.T, store *mbstore.RegisterStore, addr uint16) bool {
	t.Helper()
	coils, err := store.ReadCoils(addr, 1)
	if err != nil {
		t.Fatalf("ReadCoils addr %d: %v", addr, err)
	}
	return coils[0]
}

// writeCoil writes a single coil at the given address.
func writeCoil(t *testing.T, store *mbstore.RegisterStore, addr uint16, val bool) {
	t.Helper()
	if err := store.WriteCoils(addr, []bool{val}); err != nil {
		t.Fatalf("WriteCoils addr %d: %v", addr, err)
	}
}

// ---- CompressorModel tests ----

// TestCompressorModel_PreLubeBeforeSpeedRamp verifies the startup sequence:
// pre-lube (30 ticks) must complete before compressor speed begins ramping.
func TestCompressorModel_PreLubeBeforeSpeedRamp(t *testing.T) {
	store := makeCompressorStore()
	model := NewCompressorModel(store, nil)
	writeCoil(t, store, compCoilRunCmd, true)

	// During pre-lube (ticks 1-30), speed must remain 0.
	for tick := 1; tick <= compPreLubeTicks; tick++ {
		model.Tick()
		speed := readHoldingEng(t, store, compRegSpeed, 0, 6000)
		if speed > 0.1 {
			t.Errorf("tick %d: speed should be 0 during pre-lube, got %.1f RPM", tick, speed)
		}
	}
}

// TestCompressorModel_SpeedRampsAfterPreLube verifies speed ramps from 0 to 4500 RPM in 10 ticks.
// The pre-lube loop runs compPreLubeTicks+1 ticks: compPreLubeTicks to count down the timer
// and one additional tick for the state-transition tick (PreLube -> Running, speed stays 0).
func TestCompressorModel_SpeedRampsAfterPreLube(t *testing.T) {
	store := makeCompressorStore()
	model := NewCompressorModel(store, nil)
	writeCoil(t, store, compCoilRunCmd, true)

	// Complete pre-lube including the state-transition tick (PreLube -> Running).
	for i := 0; i <= compPreLubeTicks; i++ {
		model.Tick()
	}
	// Speed ramp: 10 ticks to full speed (450 RPM/tick * 10 = 4500 RPM).
	for tick := 1; tick <= 10; tick++ {
		model.Tick()
	}
	speed := readHoldingEng(t, store, compRegSpeed, 0, 6000)
	if math.Abs(speed-compMaxSpeed) > 50 {
		t.Errorf("speed after 10-tick ramp: got %.1f RPM, want ~%.1f RPM", speed, compMaxSpeed)
	}
}

// TestCompressorModel_StopRampsDownIn10Ticks verifies normal shutdown ramp-down.
// The state transition tick (Running -> RampDown) does not ramp. The following 10 ticks
// each decrement speed by 450 RPM, reaching 0 in 11 total ticks after the stop command.
// This mirrors the startup sequence where the PreLube -> Running transition tick is also non-ramping.
func TestCompressorModel_StopRampsDownIn10Ticks(t *testing.T) {
	store := makeCompressorStore()
	model := NewCompressorModel(store, nil)
	writeCoil(t, store, compCoilRunCmd, true)

	// Reach full speed: compPreLubeTicks+1 for pre-lube (including transition tick), then 10 ramp ticks.
	for i := 0; i <= compPreLubeTicks+10; i++ {
		model.Tick()
	}

	// Issue stop command. State-transition tick (Running -> RampDown) does not decrement speed;
	// 10 subsequent ticks each remove 450 RPM, reaching 0 in 11 total ticks.
	writeCoil(t, store, compCoilRunCmd, false)
	for tick := 1; tick <= 11; tick++ {
		model.Tick()
	}
	speed := readHoldingEng(t, store, compRegSpeed, 0, 6000)
	if speed > 5 {
		t.Errorf("speed did not ramp to 0 in 11 ticks (10 ramp + 1 transition): got %.1f RPM", speed)
	}
}

// TestCompressorModel_EStop_ImmediateSpeedZero verifies e-stop drops speed to 0 in 1 tick.
// [OT-REVIEW] E-stop = fuel cutoff + shaft brake. Not a ramp.
func TestCompressorModel_EStop_ImmediateSpeedZero(t *testing.T) {
	store := makeCompressorStore()
	model := NewCompressorModel(store, nil)
	writeCoil(t, store, compCoilRunCmd, true)

	// Reach full speed: compPreLubeTicks+1 for pre-lube (including transition tick), then 10 ramp ticks.
	for i := 0; i <= compPreLubeTicks+10; i++ {
		model.Tick()
	}
	speed := readHoldingEng(t, store, compRegSpeed, 0, 6000)
	if speed < 4000 {
		t.Fatalf("setup: expected speed near 4500, got %.1f", speed)
	}

	// Apply e-stop -- speed must drop to 0 in the current tick.
	_ = store.WriteCoilsInternal(compCoilEStop, []bool{true}) // simulate hardware e-stop activation
	model.Tick()
	speed = readHoldingEng(t, store, compRegSpeed, 0, 6000)
	if speed > 0.1 {
		t.Errorf("e-stop: speed did not drop to 0 in 1 tick: got %.1f RPM", speed)
	}
}

// TestCompressorModel_EStop_SetsTripped verifies e-stop latches the tripped coil.
func TestCompressorModel_EStop_SetsTripped(t *testing.T) {
	store := makeCompressorStore()
	model := NewCompressorModel(store, nil)
	writeCoil(t, store, compCoilRunCmd, true)
	for i := 0; i <= compPreLubeTicks+10; i++ {
		model.Tick()
	}

	_ = store.WriteCoilsInternal(compCoilEStop, []bool{true})
	model.Tick()

	if !readCoil(t, store, compCoilTripped) {
		t.Error("compressor_tripped coil should be true after e-stop")
	}
}

// TestCompressorModel_TripClears_After60Ticks verifies trip resets after 60 ticks.
func TestCompressorModel_TripClears_After60Ticks(t *testing.T) {
	store := makeCompressorStore()
	model := NewCompressorModel(store, nil)

	// Force a trip via direct struct manipulation (high vibration interlock).
	model.tripped = true
	model.tripTimer = compTripClearTicks
	model.compressorSpeed = 0

	for i := 0; i < compTripClearTicks; i++ {
		model.Tick()
	}
	if model.tripped {
		t.Error("compressor should not be tripped after 60 ticks")
	}
}

// TestCompressorModel_BearingTemps_IncreaseWhenRunning verifies bearing temps drift toward
// running targets (DE: 180 degF, NDE: 165 degF) when compressor is at speed.
// lubeOilPressure is set to a safe operating value to prevent low-lube-oil interlock trip.
func TestCompressorModel_BearingTemps_IncreaseWhenRunning(t *testing.T) {
	store := makeCompressorStore()
	model := NewCompressorModel(store, nil)
	// Force to running state directly with safe lube oil pressure to avoid interlock.
	model.compressorSpeed = compMaxSpeed
	model.startupState = compStateRunning
	model.lubeOilPressure = 35.0 // nominal running pressure, above compTripLubeOilPSI (10 PSIG)
	// Set temperatures below target.
	model.bearingDETemp = 80.0
	model.bearingNDETemp = 80.0
	writeCoil(t, store, compCoilRunCmd, true)

	initialDE := model.bearingDETemp
	initialNDE := model.bearingNDETemp
	for i := 0; i < 100; i++ {
		model.Tick()
	}
	if model.bearingDETemp <= initialDE {
		t.Errorf("DE bearing temp should increase when running: initial=%.1f final=%.1f", initialDE, model.bearingDETemp)
	}
	if model.bearingNDETemp <= initialNDE {
		t.Errorf("NDE bearing temp should increase when running: initial=%.1f final=%.1f", initialNDE, model.bearingNDETemp)
	}
}

// TestCompressorModel_BearingTemps_DecreaseWhenStopped verifies bearing temps decay toward 80 degF.
func TestCompressorModel_BearingTemps_DecreaseWhenStopped(t *testing.T) {
	store := makeCompressorStore()
	model := NewCompressorModel(store, nil)
	// Temperatures initialized high (175/160 degF, residual heat).

	for i := 0; i < 200; i++ {
		model.Tick()
	}
	if model.bearingDETemp >= 175.0 {
		t.Errorf("DE bearing temp should decrease when stopped: after 200 ticks got %.1f", model.bearingDETemp)
	}
	if model.bearingNDETemp >= 160.0 {
		t.Errorf("NDE bearing temp should decrease when stopped: after 200 ticks got %.1f", model.bearingNDETemp)
	}
}

// TestCompressorModel_SurgeProtection_ForcedDuringStartup verifies surge_protection_active
// is forced true while speed is below 70% of rated speed (3150 RPM surge threshold).
// Pre-lube requires compPreLubeTicks+1 ticks: compPreLubeTicks to count down and one
// state-transition tick (PreLube -> Running, speed stays 0).
func TestCompressorModel_SurgeProtection_ForcedDuringStartup(t *testing.T) {
	store := makeCompressorStore()
	model := NewCompressorModel(store, nil)
	writeCoil(t, store, compCoilRunCmd, true)

	// Complete pre-lube including the state-transition tick (PreLube -> Running, speed=0).
	for i := 0; i <= compPreLubeTicks; i++ {
		model.Tick()
	}
	// Tick the first speed ramp tick: speed = 450 RPM (below 3150 RPM surge threshold).
	model.Tick()
	if !readCoil(t, store, compCoilSurgeProtect) {
		t.Error("surge_protection_active should be true during acceleration below 3150 RPM")
	}
}

// TestCompressorModel_StatusWord_RunningBitSet verifies bit0 reflects running state.
// lubeOilPressure is set to nominal running pressure to prevent low-lube-oil interlock.
func TestCompressorModel_StatusWord_RunningBitSet(t *testing.T) {
	store := makeCompressorStore()
	model := NewCompressorModel(store, nil)

	// Force running state with safe lube oil pressure to avoid interlock trip.
	model.compressorSpeed = compMaxSpeed
	model.startupState = compStateRunning
	model.lubeOilPressure = 35.0 // nominal running pressure, above compTripLubeOilPSI (10 PSIG)
	writeCoil(t, store, compCoilRunCmd, true)
	model.Tick()

	sw := readHoldingRaw(t, store, compRegStatusWord)
	if sw&(1<<0) == 0 {
		t.Errorf("status word bit0 (running) should be set: sw=%04x", sw)
	}
}

// ---- MeteringModel tests ----

// TestMeteringModel_AGA3_NominalConditions verifies AGA-3 flow at nominal conditions
// produces ~2500 MSCFH (within ±5% tolerance for register quantization and drift).
func TestMeteringModel_AGA3_NominalConditions(t *testing.T) {
	store := makeMeteringStore()
	model := NewMeteringModel(store, nil)

	// Set to exactly nominal conditions and compute directly.
	model.dpCurrent = 50.0
	model.pressureCurrent = 600.0
	model.tempCurrent = 60.0

	flow := model.computeAGA3Flow(true)
	if math.Abs(flow-2500.0) > 125.0 { // ±5%
		t.Errorf("AGA-3 nominal flow: got %.1f MSCFH, want ~2500 MSCFH (±5%%)", flow)
	}
}

// TestMeteringModel_DisabledRun_ZeroFlow verifies flow is 0 when meter_run_1_enabled is false.
func TestMeteringModel_DisabledRun_ZeroFlow(t *testing.T) {
	store := makeMeteringStore()
	model := NewMeteringModel(store, nil)

	// Disable meter run 1.
	writeCoil(t, store, meterCoilRun1Enabled, false)

	for i := 0; i < 5; i++ {
		model.Tick()
	}
	flow := readHoldingEng(t, store, meterRegFlowRate, 0, 5000)
	if flow > 1.0 {
		t.Errorf("flow should be 0 when meter_run_1_enabled is false: got %.1f MSCFH", flow)
	}
}

// TestMeteringModel_HigherDP_IncreasesFlow verifies AGA-3 sensitivity: DP increase -> flow increase.
func TestMeteringModel_HigherDP_IncreasesFlow(t *testing.T) {
	lowFlow := (&MeteringModel{dpCurrent: 50.0, pressureCurrent: 600.0, tempCurrent: 60.0}).computeAGA3Flow(true)
	highFlow := (&MeteringModel{dpCurrent: 80.0, pressureCurrent: 600.0, tempCurrent: 60.0}).computeAGA3Flow(true)
	if highFlow <= lowFlow {
		t.Errorf("higher DP should produce higher flow: DP=50 -> %.1f, DP=80 -> %.1f MSCFH", lowFlow, highFlow)
	}
}

// TestMeteringModel_VolumeAccumulates verifies volume increases each tick when run is enabled.
// Coil InitValue is not applied per SOW-002.0; meter_run_1_enabled must be written explicitly.
// At nominal flow ~2500 MSCFH, each tick accumulates ~0.694 MCF. After 3600 ticks (1 hour),
// volume reaches ~2500 MCF. The test uses a large number of ticks (3600) so the accumulated
// volume exceeds register quantization resolution (100000 MCF / 32767 steps ≈ 3.05 MCF/step).
func TestMeteringModel_VolumeAccumulates(t *testing.T) {
	store := makeMeteringStore()
	model := NewMeteringModel(store, nil)
	// Coil InitValue is ignored per SOW-002.0; write explicitly to enable meter run 1.
	writeCoil(t, store, meterCoilRun1Enabled, true)

	initialVol := readHoldingEng(t, store, meterRegVolumeToday, 0, 100000)
	// Run 3600 ticks (1 simulated hour) so accumulated volume (~2500 MCF) exceeds
	// the register quantization step (~3.05 MCF).
	for i := 0; i < 3600; i++ {
		model.Tick()
	}
	finalVol := readHoldingEng(t, store, meterRegVolumeToday, 0, 100000)
	if finalVol <= initialVol {
		t.Errorf("volume should accumulate: initial=%.4f final=%.4f MCF", initialVol, finalVol)
	}
}

// TestMeteringModel_ContractDayRollover verifies volume resets to 0 after 86400 ticks.
func TestMeteringModel_ContractDayRollover(t *testing.T) {
	store := makeMeteringStore()
	model := NewMeteringModel(store, nil)

	// Fast-forward to just before rollover.
	model.contractDayTick = meterContractDayTicks - 2
	model.volumeToday = 50000.0

	model.Tick() // tick to contractDayTick = meterContractDayTicks - 1
	model.Tick() // tick to contractDayTick = meterContractDayTicks -> rollover
	if model.volumeToday >= 5000.0 {
		t.Errorf("volume should reset at contract day rollover: got %.1f MCF", model.volumeToday)
	}
	if model.contractDayTick != 0 {
		t.Errorf("contractDayTick should reset to 0 at rollover: got %d", model.contractDayTick)
	}
}

// TestMeteringModel_StationTotalMirrorsRun1 verifies station_total_flow equals meter run 1 flow.
// Coil InitValue is ignored per SOW-002.0; meter_run_1_enabled must be written explicitly.
func TestMeteringModel_StationTotalMirrorsRun1(t *testing.T) {
	store := makeMeteringStore()
	model := NewMeteringModel(store, nil)
	writeCoil(t, store, meterCoilRun1Enabled, true)

	model.Tick()
	run1Flow := readHoldingEng(t, store, meterRegFlowRate, 0, 5000)
	stationFlow := readHoldingEng(t, store, meterRegTotalFlow, 0, 20000)
	// Station total scale is 4x meter run scale; allow ±2% for quantization.
	tolerance := run1Flow * 0.02
	if math.Abs(run1Flow-stationFlow) > tolerance+5 {
		t.Errorf("station total (%.1f) should match run 1 flow (%.1f) MSCFH", stationFlow, run1Flow)
	}
}

// ---- StationMonitorModel tests ----

// TestStationMonitorModel_InletValve_Opens verifies motor-operated block valve ramps to 100% in 10 ticks.
func TestStationMonitorModel_InletValve_Opens(t *testing.T) {
	store := makeStationStore()
	model := NewStationMonitorModel(store, nil)
	// Start with valve fully closed.
	model.inletValvePos = 0.0
	writeCoil(t, store, stmonCoilInletValveCmd, true)

	for tick := 1; tick <= 10; tick++ {
		model.Tick()
	}
	pos := readHoldingEng(t, store, stmonRegInletValvePos, 0, 100)
	if pos < 99.0 {
		t.Errorf("inlet valve should be at 100%% after 10 ticks: got %.1f%%", pos)
	}
}

// TestStationMonitorModel_InletValve_Closes verifies valve ramps to 0% in 10 ticks.
func TestStationMonitorModel_InletValve_Closes(t *testing.T) {
	store := makeStationStore()
	model := NewStationMonitorModel(store, nil)
	writeCoil(t, store, stmonCoilInletValveCmd, false)

	for tick := 1; tick <= 10; tick++ {
		model.Tick()
	}
	pos := readHoldingEng(t, store, stmonRegInletValvePos, 0, 100)
	if pos > 1.0 {
		t.Errorf("inlet valve should be at 0%% after 10 ticks: got %.1f%%", pos)
	}
}

// TestStationMonitorModel_ESD_ClosesESDValveInOneTick verifies spring-return ESD closure.
// [OT-REVIEW] Spring-return ESD valves slam closed in under 1 second. FR-22.
func TestStationMonitorModel_ESD_ClosesESDValveInOneTick(t *testing.T) {
	store := makeStationStore()
	model := NewStationMonitorModel(store, nil)

	// Force ESD active.
	model.esdTimer = stmonESDLatchTicks

	model.Tick()
	esdPos := readHoldingEng(t, store, stmonRegESDValvePos, 0, 100)
	if esdPos > 0.5 {
		t.Errorf("ESD valve should close in 1 tick (spring-return): got %.1f%%", esdPos)
	}
}

// TestStationMonitorModel_ESD_ClosesBlockValvesSlowly verifies motor-operated block valves
// close at 10%/tick during ESD (not instantaneously).
func TestStationMonitorModel_ESD_ClosesBlockValvesSlowly(t *testing.T) {
	store := makeStationStore()
	model := NewStationMonitorModel(store, nil)
	// Force ESD and record valve position after 1 tick (should be 90%, not 0%).
	model.esdTimer = stmonESDLatchTicks

	model.Tick()
	inletPos := readHoldingEng(t, store, stmonRegInletValvePos, 0, 100)
	// After 1 ESD tick: 100% - 10%/tick = 90%.
	if inletPos < 85.0 || inletPos > 95.0 {
		t.Errorf("inlet block valve after 1 ESD tick: got %.1f%%, want ~90%%", inletPos)
	}
}

// TestStationMonitorModel_ESD_LatchesFor120Ticks verifies ESD latch duration.
func TestStationMonitorModel_ESD_LatchesFor120Ticks(t *testing.T) {
	store := makeStationStore()
	model := NewStationMonitorModel(store, nil)
	model.esdTimer = stmonESDLatchTicks // 120

	for i := 0; i < stmonESDLatchTicks-1; i++ {
		model.Tick()
	}
	// Should still be active just before clearing.
	if model.esdTimer <= 0 {
		t.Error("ESD should still be latched at tick 119")
	}

	model.Tick() // tick 120: timer reaches 0
	esdStatus := readCoil(t, store, stmonCoilESDActiveStatus)
	if esdStatus {
		t.Error("ESD status should clear after 120 ticks")
	}
}

// TestStationMonitorModel_OutletPressure_IsInletTimes1Point4 verifies the compression
// ratio estimate applied at the station monitor (independent model, FR-19).
func TestStationMonitorModel_OutletPressure_IsInletTimes1Point4(t *testing.T) {
	store := makeStationStore()
	model := NewStationMonitorModel(store, nil)
	model.inletPressure = 600.0

	model.Tick()
	outletRaw, err := store.ReadHolding(stmonRegOutletPressure, 1)
	if err != nil {
		t.Fatalf("ReadHolding outlet: %v", err)
	}
	outlet := toEng(outletRaw[0], 0, 1500)
	expected := 600.0 * 1.4
	// Allow ±5% for noise.
	if math.Abs(outlet-expected) > expected*0.05+5 {
		t.Errorf("outlet pressure: got %.1f PSIG, want ~%.1f (inlet*1.4)", outlet, expected)
	}
}

// ---- GasAnalysisModel tests ----

// TestGasAnalysisModel_CompositionStatic_BetweenCycles verifies composition does not change
// between NGC analysis cycles. FR-24.
func TestGasAnalysisModel_CompositionStatic_BetweenCycles(t *testing.T) {
	store := makeGasStore()
	model := NewGasAnalysisModel(store, nil)
	model.Tick() // write initial values

	initialMethane := readHoldingEng(t, store, gasRegMethane, 0, 100)
	// Tick 50 times but stay well within the minimum cycle length (300 ticks).
	for i := 0; i < 50; i++ {
		model.Tick()
	}
	laterMethane := readHoldingEng(t, store, gasRegMethane, 0, 100)
	// Allow tiny quantization difference but not a real composition update.
	if math.Abs(laterMethane-initialMethane) > 0.5 {
		t.Errorf("methane should be static between cycles: initial=%.2f later=%.2f", initialMethane, laterMethane)
	}
}

// TestGasAnalysisModel_AnalysisCycle_StateTransitions verifies the state machine:
// idle (0) -> analyzing (1) -> complete (2) -> idle (0). FR-25.
func TestGasAnalysisModel_AnalysisCycle_StateTransitions(t *testing.T) {
	store := makeGasStore()
	model := NewGasAnalysisModel(store, nil)

	// Force cycle to start immediately.
	model.cycleTimer = model.cycleLength - 1

	// Trigger cycle start.
	model.Tick()
	status := readHoldingRaw(t, store, gasRegCycleStatus)
	if status != gasCycleAnalyzing {
		t.Errorf("after cycle trigger: status=%d, want %d (analyzing)", status, gasCycleAnalyzing)
	}

	// Tick through the analysis phase (30 ticks).
	for i := 0; i < gasAnalyzeTicks; i++ {
		model.Tick()
	}
	status = readHoldingRaw(t, store, gasRegCycleStatus)
	if status != gasCycleComplete {
		t.Errorf("after 30 analysis ticks: status=%d, want %d (complete)", status, gasCycleComplete)
	}

	// Tick through the complete hold period (5 ticks).
	for i := 0; i < gasCompleteLatch; i++ {
		model.Tick()
	}
	status = readHoldingRaw(t, store, gasRegCycleStatus)
	if status != gasCycleIdle {
		t.Errorf("after complete hold: status=%d, want %d (idle)", status, gasCycleIdle)
	}
}

// TestGasAnalysisModel_BTU_InNominalRange verifies heating value at nominal composition.
func TestGasAnalysisModel_BTU_InNominalRange(t *testing.T) {
	model := &GasAnalysisModel{
		methanePct: 92.0, ethanePct: 4.0, propanePct: 1.0,
		co2Pct: 0.5, nitrogenPct: 2.0,
	}
	btu := model.computeBTU()
	if btu < 1020.0 || btu > 1050.0 {
		t.Errorf("BTU at nominal composition: got %.1f BTU/SCF, want 1020-1050", btu)
	}
}

// TestGasAnalysisModel_SG_InNominalRange verifies specific gravity at nominal composition.
func TestGasAnalysisModel_SG_InNominalRange(t *testing.T) {
	model := &GasAnalysisModel{
		methanePct: 92.0, ethanePct: 4.0, propanePct: 1.0,
		co2Pct: 0.5, nitrogenPct: 2.0,
	}
	sg := model.computeSG()
	if sg < 0.58 || sg > 0.65 {
		t.Errorf("SG at nominal composition: got %.4f, want 0.58-0.65", sg)
	}
}

// TestGasAnalysisModel_MoistureAlarm_LatchesAboveTariff verifies moisture alarm at > 7 lb/MMSCF.
func TestGasAnalysisModel_MoistureAlarm_LatchesAboveTariff(t *testing.T) {
	store := makeGasStore()
	model := NewGasAnalysisModel(store, nil)
	// Force moisture above the tariff limit.
	model.moistureContent = gasMoistureTariff + 1.0

	model.Tick()
	if !readCoil(t, store, gasCoilMoistureAlarm) {
		t.Error("moisture_alarm_active should be true when moisture > 7 lb/MMSCF")
	}
}

// TestGasAnalysisModel_GCAlarm_Independent verifies gc_alarm is separate from moisture_alarm.
func TestGasAnalysisModel_GCAlarm_Independent(t *testing.T) {
	store := makeGasStore()
	model := NewGasAnalysisModel(store, nil)
	model.gcAlarmTimer = 5 // simulate active GC fault

	model.Tick()
	gcAlarm := readCoil(t, store, gasCoilGCAlarm)
	moistureAlarm := readCoil(t, store, gasCoilMoistureAlarm)
	if !gcAlarm {
		t.Error("gc_alarm_active should be true when gcAlarmTimer > 0")
	}
	if moistureAlarm {
		t.Error("moisture_alarm_active should be independent of gc_alarm (no moisture exceedance set)")
	}
}

// TestGasAnalysisModel_AnalysisInProgress_ReflectsAnalyzingPhase verifies coil is true
// only during analyzing state, not during idle or complete. FR-30.
func TestGasAnalysisModel_AnalysisInProgress_ReflectsAnalyzingPhase(t *testing.T) {
	store := makeGasStore()
	model := NewGasAnalysisModel(store, nil)
	model.Tick()

	// Initially idle: analysis_in_progress should be false.
	if readCoil(t, store, gasCoilAnalysisInProgress) {
		t.Error("analysis_in_progress should be false during idle phase")
	}

	// Force to analyzing state.
	model.cycleTimer = model.cycleLength - 1
	model.Tick()
	if !readCoil(t, store, gasCoilAnalysisInProgress) {
		t.Error("analysis_in_progress should be true during analyzing phase")
	}

	// Complete the analysis cycle.
	for i := 0; i < gasAnalyzeTicks; i++ {
		model.Tick()
	}
	if readCoil(t, store, gasCoilAnalysisInProgress) {
		t.Error("analysis_in_progress should be false after analysis completes")
	}
}

// TestGasAnalysisModel_CompositionValues_InRealisticRanges verifies composition is
// within specified bounds after drift. FR-26.
func TestGasAnalysisModel_CompositionValues_InRealisticRanges(t *testing.T) {
	model := NewGasAnalysisModel(nil, nil)
	// Run 10 composition updates (simulating 10 analysis cycles).
	for i := 0; i < 10; i++ {
		model.updateComposition()
	}
	if model.methanePct < 90 || model.methanePct > 94 {
		t.Errorf("methane out of range: %.2f%%", model.methanePct)
	}
	if model.ethanePct < 3 || model.ethanePct > 5 {
		t.Errorf("ethane out of range: %.2f%%", model.ethanePct)
	}
	if model.propanePct < 0.5 || model.propanePct > 1.5 {
		t.Errorf("propane out of range: %.2f%%", model.propanePct)
	}
	if model.co2Pct < 0.2 || model.co2Pct > 0.8 {
		t.Errorf("CO2 out of range: %.2f%%", model.co2Pct)
	}
	if model.nitrogenPct < 1 || model.nitrogenPct > 3 {
		t.Errorf("nitrogen out of range: %.2f%%", model.nitrogenPct)
	}
}
