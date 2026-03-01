package process

import (
	"testing"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// makeLineAStore creates a RegisterStore for the mfg-line-a variant (SLC-500, one-based).
func makeLineAStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "mfg-plc-01",
		DeviceID:    "slc-500-05",
		Addressing:  "one-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 1, Name: "conveyor_speed", Unit: "ft/min", ScaleMin: 0, ScaleMax: 200, Writable: true, InitValue: 0},
			{Address: 2, Name: "motor_current", Unit: "A", ScaleMin: 0, ScaleMax: 30, Writable: false, InitValue: 0},
			{Address: 3, Name: "product_count", Unit: "units", ScaleMin: 0, ScaleMax: 10000, Writable: false, InitValue: 0},
			{Address: 4, Name: "reject_count", Unit: "units", ScaleMin: 0, ScaleMax: 500, Writable: false, InitValue: 0},
			{Address: 5, Name: "line_temperature", Unit: "degF", ScaleMin: 0, ScaleMax: 200, Writable: false, InitValue: 0},
			{Address: 6, Name: "cycle_time", Unit: "s", ScaleMin: 0, ScaleMax: 120, Writable: false, InitValue: 0},
			{Address: 7, Name: "status_word", Unit: "bitmask", ScaleMin: 0, ScaleMax: 65535, Writable: false, InitValue: 0},
		},
		Coils: []device.CoilEntry{
			{Address: 1, Name: "conveyor_run", Writable: true},
			{Address: 2, Name: "conveyor_direction", Writable: true},
			{Address: 3, Name: "e_stop_active", Writable: false},
			{Address: 4, Name: "jam_detected", Writable: false},
		},
	})
}

// makeCoolingStore creates a RegisterStore for the mfg-cooling variant (Modicon 984, one-based).
func makeCoolingStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "mfg-plc-02",
		DeviceID:    "modicon-984",
		Addressing:  "one-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 1, Name: "supply_temp", Unit: "degF", ScaleMin: 40, ScaleMax: 120, Writable: false, InitValue: 16383},
			{Address: 2, Name: "return_temp", Unit: "degF", ScaleMin: 40, ScaleMax: 120, Writable: false, InitValue: 16383},
			{Address: 3, Name: "flow_rate", Unit: "GPM", ScaleMin: 0, ScaleMax: 500, Writable: false, InitValue: 0},
			{Address: 4, Name: "pump_pressure", Unit: "PSI", ScaleMin: 0, ScaleMax: 80, Writable: false, InitValue: 0},
			{Address: 5, Name: "tank_level", Unit: "%", ScaleMin: 0, ScaleMax: 100, Writable: false, InitValue: 16383},
			{Address: 6, Name: "setpoint_temp", Unit: "degF", ScaleMin: 40, ScaleMax: 80, Writable: true, InitValue: 16383},
			{Address: 7, Name: "pump_runtime_hours", Unit: "hours", ScaleMin: 0, ScaleMax: 65535, Writable: false, InitValue: 0},
		},
		Coils: []device.CoilEntry{
			{Address: 1, Name: "pump_1_run", Writable: true},
			{Address: 2, Name: "pump_2_run", Writable: true},
			{Address: 3, Name: "low_coolant_alarm", Writable: false},
			{Address: 4, Name: "high_temp_alarm", Writable: false},
		},
	})
}

// readMotorCurrent reads the motor current register in engineering units (A).
func readMotorCurrent(t *testing.T, store *mbstore.RegisterStore) float64 {
	t.Helper()
	vals, err := store.ReadHolding(lineARegMotorCurrent, 1)
	if err != nil {
		t.Fatalf("ReadHolding motor_current: %v", err)
	}
	return toEng(vals[0], 0, 30)
}

// TestLineAModel_ConveyorOn_MotorCurrentRises verifies that enabling the conveyor
// causes motor current to increase from near zero.
func TestLineAModel_ConveyorOn_MotorCurrentRises(t *testing.T) {
	store := makeLineAStore()
	model := NewLineAModel(store, nil)

	// Set speed to 100 ft/min.
	_ = store.WriteHolding(lineARegConveyorSpeed, []uint16{toRaw(100, 0, 200)})
	_ = store.WriteCoils(lineACoilConveyorRun, []bool{true})

	initialCurrent := readMotorCurrent(t, store) // should be 0 initially

	for i := 0; i < 10; i++ {
		model.Tick()
	}

	finalCurrent := readMotorCurrent(t, store)
	if finalCurrent <= initialCurrent {
		t.Errorf("motor current did not rise: initial=%v final=%v", initialCurrent, finalCurrent)
	}
	if finalCurrent < 5.0 {
		t.Errorf("motor current too low after conveyor on: got %v A, want >= 5 A", finalCurrent)
	}
}

// TestCoolingModel_SetpointChange_SupplyTempConverges verifies that writing a new
// setpoint_temp causes supply_temp to converge toward it over several ticks.
func TestCoolingModel_SetpointChange_SupplyTempConverges(t *testing.T) {
	store := makeCoolingStore()
	model := NewCoolingModel(store, nil)

	// Set a new setpoint to 70 degF (current supply is 65 degF).
	newSetpoint := toRaw(70.0, 40, 80)
	_ = store.WriteHolding(coolingRegSetpointTemp, []uint16{newSetpoint})

	// Tick until supply temp approaches setpoint (0.5 degF/tick, ~10 ticks for 5 degF).
	for i := 0; i < 15; i++ {
		model.Tick()
	}

	supplyVals, err := store.ReadHolding(coolingRegSupplyTemp, 1)
	if err != nil {
		t.Fatalf("ReadHolding supply_temp: %v", err)
	}
	supplyEng := toEng(supplyVals[0], 40, 120)

	if supplyEng < 67.0 {
		t.Errorf("supply_temp did not converge toward setpoint 70: got %v degF after 15 ticks", supplyEng)
	}
}

// TestCoolingModel_LowCoolantAlarm_Hysteresis verifies alarm triggers at 20% tank level
// and clears at 30%.
func TestCoolingModel_LowCoolantAlarm_Hysteresis(t *testing.T) {
	store := makeCoolingStore()
	model := NewCoolingModel(store, nil)

	// Set tank level below alarm threshold.
	model.tankLevel = 15.0

	model.Tick()
	coils, _ := store.ReadCoils(coolingCoilLowCoolant, 1)
	if !coils[0] {
		t.Error("low_coolant_alarm did not fire at 15% tank level")
	}

	// Raise tank level above hysteresis clear threshold.
	model.tankLevel = 35.0
	model.lowCoolantLatched = true

	model.Tick()
	coils, _ = store.ReadCoils(coolingCoilLowCoolant, 1)
	if coils[0] {
		t.Error("low_coolant_alarm did not clear at 35% tank level")
	}
}

// TestCoolingModel_HighTempAlarm_Hysteresis verifies alarm triggers at 85 degF return
// and clears at 75 degF.
// [OT-REVIEW] Threshold 85/75 ensures setpoint attack (write 80 degF) actually triggers alarm.
func TestCoolingModel_HighTempAlarm_Hysteresis(t *testing.T) {
	store := makeCoolingStore()
	model := NewCoolingModel(store, nil)

	// Force supply temp high enough that return_temp exceeds 85 degF.
	model.supplyTemp = 80.0 // return = 80 + ~5 = 85+ degF

	// Tick until return temp exceeds threshold.
	fired := false
	for i := 0; i < 5; i++ {
		model.Tick()
		coils, _ := store.ReadCoils(coolingCoilHighTemp, 1)
		if coils[0] {
			fired = true
			break
		}
	}
	if !fired {
		t.Error("high_temp_alarm did not fire with supply_temp=80 degF")
	}

	// Cool down: set supply temp low so return drops below 75 degF.
	model.supplyTemp = 55.0
	model.highTempLatched = true

	cleared := false
	for i := 0; i < 20; i++ {
		model.Tick()
		coils, _ := store.ReadCoils(coolingCoilHighTemp, 1)
		if !coils[0] {
			cleared = true
			break
		}
	}
	if !cleared {
		t.Error("high_temp_alarm did not clear after supply_temp dropped to 55 degF")
	}
}

// TestCoolingModel_PumpRuntimeHours_Increments verifies hours accumulate when pump 1 is running.
func TestCoolingModel_PumpRuntimeHours_Increments(t *testing.T) {
	store := makeCoolingStore()
	model := NewCoolingModel(store, nil)
	model.runtimeHours = 0

	_ = store.WriteCoils(coolingCoilPump1Run, []bool{true})

	for i := 0; i < 3600; i++ {
		model.Tick()
	}

	// After 3600 ticks (1 hour), raw runtime should have advanced by 1.
	// Initial hours = 0, after 3600 ticks = 1.0 hour.
	vals, _ := store.ReadHolding(coolingRegPumpRunHours, 1)
	rawHours := vals[0]
	if rawHours < 1 {
		t.Errorf("pump_runtime_hours did not increment after 3600 ticks: got %d", rawHours)
	}
}

// TestLineAModel_ProductCount_ResetAtShiftChange verifies product_count resets to 0
// at the 28800-tick boundary (8-hour shift change via PLC ladder logic).
func TestLineAModel_ProductCount_ResetAtShiftChange(t *testing.T) {
	store := makeLineAStore()
	model := NewLineAModel(store, nil)

	_ = store.WriteHolding(lineARegConveyorSpeed, []uint16{toRaw(100, 0, 200)})
	_ = store.WriteCoils(lineACoilConveyorRun, []bool{true})

	// Advance to just before shift change.
	model.shiftTick = lineAShiftTicks - 1
	model.productCount = 500 // non-zero to verify reset

	// One more tick crosses the shift boundary.
	model.Tick()

	vals, _ := store.ReadHolding(lineARegProductCount, 1)
	if vals[0] != 0 {
		t.Errorf("product_count did not reset at shift change: got raw %d", vals[0])
	}
}
