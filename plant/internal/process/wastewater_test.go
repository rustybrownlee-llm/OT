package process

import (
	"testing"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// ---- Store constructors ----

// makeInfluentStore creates a RegisterStore for the ww-influent variant.
// SLC-500/05, one-based addressing via ProSoft MVI46-MCM.
func makeInfluentStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "ww-plc-01",
		DeviceID:    "slc-500-05",
		Addressing:  "one-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 1, Name: "influent_flow_rate", Unit: "GPM", ScaleMin: 0, ScaleMax: 5000, Writable: false, InitValue: 0},
			{Address: 2, Name: "influent_flow_total", Unit: "gal x1000", ScaleMin: 0, ScaleMax: 99999, Writable: true, InitValue: 0},
			{Address: 3, Name: "bar_screen_differential", Unit: "in WC", ScaleMin: 0, ScaleMax: 12, Writable: false, InitValue: 0},
			{Address: 4, Name: "grit_chamber_level", Unit: "%", ScaleMin: 0, ScaleMax: 100, Writable: false, InitValue: 0},
			{Address: 5, Name: "primary_clarifier_level", Unit: "ft", ScaleMin: 0, ScaleMax: 5, Writable: false, InitValue: 16383},
			{Address: 6, Name: "sludge_blanket_depth", Unit: "ft", ScaleMin: 0, ScaleMax: 8, Writable: false, InitValue: 0},
			{Address: 7, Name: "sludge_pump_cycle_time", Unit: "min", ScaleMin: 0, ScaleMax: 120, Writable: true, InitValue: 16383},
			{Address: 8, Name: "influent_ph", Unit: "pH", ScaleMin: 0, ScaleMax: 14, Writable: false, InitValue: 16383},
			{Address: 9, Name: "influent_temperature", Unit: "degF", ScaleMin: 40, ScaleMax: 90, Writable: false, InitValue: 16383},
		},
		Coils: []device.CoilEntry{
			{Address: 1, Name: "bar_screen_motor", Writable: true},
			{Address: 2, Name: "grit_pump", Writable: true},
			{Address: 3, Name: "primary_sludge_pump", Writable: true},
			{Address: 4, Name: "influent_gate", Writable: true},
		},
	})
}

// makeEffluentStore creates a RegisterStore for the ww-effluent variant.
// SLC-500/05, one-based addressing via ProSoft MVI46-MCM.
func makeEffluentStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "ww-plc-02",
		DeviceID:    "slc-500-05",
		Addressing:  "one-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 1, Name: "effluent_flow_rate", Unit: "GPM", ScaleMin: 0, ScaleMax: 5000, Writable: false, InitValue: 0},
			{Address: 2, Name: "secondary_clarifier_level", Unit: "ft", ScaleMin: 0, ScaleMax: 5, Writable: false, InitValue: 16383},
			{Address: 3, Name: "ras_flow", Unit: "GPM", ScaleMin: 0, ScaleMax: 2500, Writable: false, InitValue: 0},
			{Address: 4, Name: "was_flow", Unit: "GPM", ScaleMin: 0, ScaleMax: 100, Writable: false, InitValue: 0},
			{Address: 5, Name: "effluent_tss", Unit: "mg/L", ScaleMin: 0, ScaleMax: 50, Writable: false, InitValue: 0},
			{Address: 6, Name: "effluent_ammonia_nitrogen", Unit: "mg/L", ScaleMin: 0, ScaleMax: 30, Writable: false, InitValue: 0},
			{Address: 7, Name: "uv_transmittance", Unit: "%", ScaleMin: 0, ScaleMax: 100, Writable: false, InitValue: 16383},
			{Address: 8, Name: "permit_interlock_status", Unit: "bitmask", ScaleMin: 0, ScaleMax: 7, Writable: false, InitValue: 0},
		},
		Coils: []device.CoilEntry{
			{Address: 1, Name: "ras_pump", Writable: true},
			{Address: 2, Name: "was_pump", Writable: true},
			{Address: 3, Name: "uv_disinfection_enable", Writable: true},
			{Address: 4, Name: "effluent_discharge_valve", Writable: true},
		},
	})
}

// makeAerationStore creates a RegisterStore for the ww-aeration variant.
// CompactLogix L33ER, zero-based addressing.
func makeAerationStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "ww-plc-03",
		DeviceID:    "compactlogix-l33er",
		Addressing:  "zero-based",
		MaxHolding:  256,
		MaxCoils:    256,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 0, Name: "blower_speed_setpoint", Unit: "%", ScaleMin: 0, ScaleMax: 100, Writable: true, InitValue: toRaw(60, 0, 100)},
			{Address: 1, Name: "blower_speed_feedback", Unit: "RPM", ScaleMin: 0, ScaleMax: 3600, Writable: false, InitValue: 0},
			{Address: 2, Name: "do_setpoint", Unit: "mg/L", ScaleMin: 0, ScaleMax: 8, Writable: true, InitValue: toRaw(2.0, 0, 8)},
			{Address: 3, Name: "dissolved_oxygen", Unit: "mg/L", ScaleMin: 0, ScaleMax: 12, Writable: false, InitValue: toRaw(2.0, 0, 12)},
			{Address: 4, Name: "air_flow_rate", Unit: "SCFM", ScaleMin: 0, ScaleMax: 5000, Writable: false, InitValue: 0},
			{Address: 5, Name: "blower_discharge_pressure", Unit: "PSI", ScaleMin: 0, ScaleMax: 15, Writable: false, InitValue: 0},
			{Address: 6, Name: "blower_inlet_temperature", Unit: "degF", ScaleMin: 30, ScaleMax: 120, Writable: false, InitValue: 16383},
			{Address: 7, Name: "blower_motor_current", Unit: "A", ScaleMin: 0, ScaleMax: 200, Writable: false, InitValue: 0},
			{Address: 8, Name: "aeration_basin_level", Unit: "ft", ScaleMin: 0, ScaleMax: 15, Writable: false, InitValue: toRaw(12.0, 0, 15)},
			{Address: 9, Name: "mlss", Unit: "mg/L", ScaleMin: 0, ScaleMax: 10000, Writable: false, InitValue: toRaw(3000, 0, 10000)},
			{Address: 10, Name: "sour", Unit: "mg O2/g MLSS/hr", ScaleMin: 0, ScaleMax: 30, Writable: false, InitValue: 0},
			{Address: 11, Name: "blower_run_hours", Unit: "hours", ScaleMin: 0, ScaleMax: 999999, Writable: false, InitValue: 0},
		},
		Coils: []device.CoilEntry{
			{Address: 0, Name: "blower_1_run", Writable: true},
			{Address: 1, Name: "blower_2_run", Writable: true},
			{Address: 2, Name: "aeration_valve_auto", Writable: true},
			{Address: 3, Name: "do_control_enable", Writable: true},
		},
	})
}

// makeCellularStore creates a RegisterStore for the vendor-remote-access variant.
// Cradlepoint IBR600, zero-based addressing.
func makeCellularStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "ww-modem-01",
		DeviceID:    "cradlepoint-ibr600",
		Addressing:  "zero-based",
		MaxHolding:  7,
		MaxCoils:    0,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 0, Name: "wan_link_status", Unit: "enum", ScaleMin: 0, ScaleMax: 2, Writable: false, InitValue: 2},
			{Address: 1, Name: "signal_strength", Unit: "dBm-magnitude", ScaleMin: 0, ScaleMax: 120, Writable: false, InitValue: 50},
			{Address: 2, Name: "data_usage_gb", Unit: "GB", ScaleMin: 0, ScaleMax: 999, Writable: false, InitValue: 0},
			{Address: 3, Name: "uptime_seconds", Unit: "s", ScaleMin: 0, ScaleMax: 65535, Writable: false, InitValue: 0},
			{Address: 4, Name: "active_lan_clients", Unit: "count", ScaleMin: 0, ScaleMax: 10, Writable: false, InitValue: 3},
			{Address: 5, Name: "nat_mode", Unit: "enum", ScaleMin: 0, ScaleMax: 1, Writable: false, InitValue: 1},
			{Address: 6, Name: "vpn_tunnel_status", Unit: "enum", ScaleMin: 0, ScaleMax: 2, Writable: false, InitValue: 2},
		},
	})
}

// ---- InfluentModel tests ----

// TestInfluentModel_Name verifies model identifier for logging.
func TestInfluentModel_Name(t *testing.T) {
	store := makeInfluentStore()
	model := NewInfluentModel(store, nil)
	if model.Name() != "ww-influent" {
		t.Errorf("Name() = %q, want %q", model.Name(), "ww-influent")
	}
}

// TestInfluentModel_Tick_ProducesValidFlowRate verifies that after ticking, the influent flow
// register contains a value within the expected diurnal range for a 2 MGD WWTP.
func TestInfluentModel_Tick_ProducesValidFlowRate(t *testing.T) {
	store := makeInfluentStore()
	model := NewInfluentModel(store, nil)

	model.Tick()

	flowGPM := readHoldingEng(t, store, inflRegFlowRate, 0, 5000)
	// Diurnal range: 2000 GPM * (1 ± 0.30) = 1400-2600 GPM. Allow generous noise margin.
	if flowGPM < 500 || flowGPM > 5000 {
		t.Errorf("influent flow out of range: got %v GPM, expected 500-5000", flowGPM)
	}
}

// TestInfluentModel_BarScreenDP_AccumulatesWhenMotorOff verifies bar screen differential pressure
// accumulates when the rake motor is off and decreases toward setpoint when motor is on.
func TestInfluentModel_BarScreenDP_AccumulatesWhenMotorOff(t *testing.T) {
	store := makeInfluentStore()
	model := NewInfluentModel(store, nil)

	// Motor off: DP should accumulate over 100 ticks.
	_ = store.WriteCoils(inflCoilBarScreen, []bool{false})
	dpBefore := model.barScreenDP
	for i := 0; i < 100; i++ {
		model.Tick()
	}
	dpAfter := model.barScreenDP
	if dpAfter <= dpBefore {
		t.Errorf("bar screen DP did not accumulate: before=%v after=%v", dpBefore, dpAfter)
	}

	// Motor on: DP should decrease toward 0.3 in WC.
	_ = store.WriteCoils(inflCoilBarScreen, []bool{true})
	dpHighPoint := model.barScreenDP
	for i := 0; i < 50; i++ {
		model.Tick()
	}
	dpAfterRake := model.barScreenDP
	if dpAfterRake >= dpHighPoint {
		t.Errorf("bar screen DP did not decrease when motor on: high=%v after=%v", dpHighPoint, dpAfterRake)
	}
}

// TestInfluentModel_SludgeBlanket_RisesWithoutPumping verifies sludge blanket depth increases
// when primary sludge pump is off.
func TestInfluentModel_SludgeBlanket_RisesWithoutPumping(t *testing.T) {
	store := makeInfluentStore()
	model := NewInfluentModel(store, nil)

	_ = store.WriteCoils(inflCoilSludgePump, []bool{false})
	before := model.sludgeBlanket
	for i := 0; i < 200; i++ {
		model.Tick()
	}
	if model.sludgeBlanket <= before {
		t.Errorf("sludge blanket did not rise: before=%v after=%v", before, model.sludgeBlanket)
	}
}

// TestInfluentModel_FlowTotal_Accumulates verifies that the flow totalizer increments each tick.
func TestInfluentModel_FlowTotal_Accumulates(t *testing.T) {
	store := makeInfluentStore()
	model := NewInfluentModel(store, nil)

	// Tick several times and verify totalizer is accumulating.
	for i := 0; i < 10; i++ {
		model.Tick()
	}
	if model.flowTotal <= 0 {
		t.Errorf("flow totalizer did not accumulate: got %v", model.flowTotal)
	}
}

// ---- EffluentModel tests ----

// TestEffluentModel_Name verifies model identifier.
func TestEffluentModel_Name(t *testing.T) {
	store := makeEffluentStore()
	model := NewEffluentModel(store, nil)
	if model.Name() != "ww-effluent" {
		t.Errorf("Name() = %q, want %q", model.Name(), "ww-effluent")
	}
}

// TestEffluentModel_PermitInterlock_AllBitsSet_WhenUVOnAndFlowOK verifies the permit interlock
// bitmask has all 3 bits set when UV is enabled and flow is within permit limits.
func TestEffluentModel_PermitInterlock_AllBitsSet_WhenUVOnAndFlowOK(t *testing.T) {
	store := makeEffluentStore()
	model := NewEffluentModel(store, nil)

	// Enable UV system.
	_ = store.WriteCoils(effCoilUVEnable, []bool{true})

	// Tick several times to stabilize.
	for i := 0; i < 5; i++ {
		model.Tick()
	}

	permitRaw := readHoldingRaw(t, store, effRegPermitStatus)
	// All 3 bits (UV, flow, NPDES) should be set = value 7.
	if permitRaw != 7 {
		t.Errorf("permit_interlock_status: got %d, want 7 (all bits set when UV on and flow normal)", permitRaw)
	}
}

// TestEffluentModel_PermitInterlock_UVBitClears_WhenUVOff verifies that disabling the UV system
// clears bit0 of the permit interlock. This is the first step of the educational discharge attack.
func TestEffluentModel_PermitInterlock_UVBitClears_WhenUVOff(t *testing.T) {
	store := makeEffluentStore()
	model := NewEffluentModel(store, nil)

	// UV off.
	_ = store.WriteCoils(effCoilUVEnable, []bool{false})

	model.Tick()

	permitRaw := readHoldingRaw(t, store, effRegPermitStatus)
	// bit0 (UV active) should be 0. Bits 1 and 2 may still be set.
	if permitRaw&uint16(permitBitUVActive) != 0 {
		t.Errorf("UV bit should be 0 when UV disabled: permit = %d", permitRaw)
	}
}

// TestEffluentModel_RASFlow_DecreasesWhenPumpOff verifies that when the RAS pump is off,
// the RAS flow decreases from its initial value over time (decays at 5 GPM/tick).
func TestEffluentModel_RASFlow_DecreasesWhenPumpOff(t *testing.T) {
	store := makeEffluentStore()
	model := NewEffluentModel(store, nil)

	// RAS pump off -- flow should decay.
	_ = store.WriteCoils(effCoilRASPump, []bool{false})
	initialRAS := model.rasFlow
	for i := 0; i < 30; i++ {
		model.Tick()
	}
	afterRAS := model.rasFlow

	if afterRAS >= initialRAS {
		t.Errorf("RAS flow should decay when pump off: initial=%v after=%v", initialRAS, afterRAS)
	}
}

// ---- AerationModel tests ----

// TestAerationModel_Name verifies model identifier.
func TestAerationModel_Name(t *testing.T) {
	store := makeAerationStore()
	model := NewAerationModel(store, nil)
	if model.Name() != "ww-aeration" {
		t.Errorf("Name() = %q, want %q", model.Name(), "ww-aeration")
	}
}

// TestAerationModel_DOPIDLoop_BlowerAdjustsToSetpoint verifies that when DO control is
// enabled and DO setpoint is written, the blower speed adjusts over ~60 ticks (FR-013).
// This is the primary composability proof for the aeration model.
func TestAerationModel_DOPIDLoop_BlowerAdjustsToSetpoint(t *testing.T) {
	store := makeAerationStore()
	model := NewAerationModel(store, nil)

	// Enable blower 1 and DO control.
	_ = store.WriteCoils(aerCoilBlower1Run, []bool{true})
	_ = store.WriteCoils(aerCoilDOControl, []bool{true})

	// Record initial blower speed.
	spBefore := model.blowerSpeedSP

	// Write a higher DO setpoint (3.0 mg/L, above current 2.0 mg/L).
	_ = store.WriteHolding(aerRegDOSetpoint, []uint16{toRaw(3.0, 0, 8)})

	// Tick 60 times (1 minute). Blower speed should have ramped up.
	for i := 0; i < 60; i++ {
		model.Tick()
	}

	spAfter := model.blowerSpeedSP
	if spAfter <= spBefore {
		t.Errorf("blower speed should increase when DO setpoint raised: before=%v after=%v", spBefore, spAfter)
	}
}

// TestAerationModel_DODropsWhenBlowerStopped verifies that dissolved oxygen decreases
// when the blower is stopped (no oxygen input, biomass demand continues).
func TestAerationModel_DODropsWhenBlowerStopped(t *testing.T) {
	store := makeAerationStore()
	model := NewAerationModel(store, nil)

	// Blowers off.
	_ = store.WriteCoils(aerCoilBlower1Run, []bool{false})
	_ = store.WriteCoils(aerCoilBlower2Run, []bool{false})

	doBefore := model.doMeasured

	// Tick 30 times with blowers off.
	for i := 0; i < 30; i++ {
		model.Tick()
	}

	doAfter := model.doMeasured
	if doAfter >= doBefore {
		t.Errorf("DO did not drop when blowers stopped: before=%v after=%v", doBefore, doAfter)
	}
}

// TestAerationModel_RunHours_Increments verifies blower run hour counter advances.
func TestAerationModel_RunHours_Increments(t *testing.T) {
	store := makeAerationStore()
	model := NewAerationModel(store, nil)

	// Blower 1 on.
	_ = store.WriteCoils(aerCoilBlower1Run, []bool{true})

	hoursBefore := model.blowerRunHours

	// Tick 3600 times (1 simulated hour).
	for i := 0; i < 3600; i++ {
		model.Tick()
	}

	hoursAfter := model.blowerRunHours
	if hoursAfter < hoursBefore+0.99 {
		t.Errorf("blower run hours did not increment by ~1 in 3600 ticks: before=%v after=%v", hoursBefore, hoursAfter)
	}
}

// TestAerationModel_SOURDecays_UnderAnoxicConditions verifies that SOUR begins to decay
// when DO falls below 1.0 mg/L (anoxic threshold). This models activated sludge depression.
func TestAerationModel_SOURDecays_UnderAnoxicConditions(t *testing.T) {
	store := makeAerationStore()
	model := NewAerationModel(store, nil)

	// Force anoxic conditions: set DO below threshold.
	model.doMeasured = 0.5

	sourBefore := model.sourValue

	// Tick 50 ticks with DO below threshold.
	for i := 0; i < 50; i++ {
		model.doMeasured = 0.5 // keep DO low
		model.Tick()
	}

	if model.sourValue >= sourBefore {
		t.Errorf("SOUR did not decay under anoxic conditions: before=%v after=%v", sourBefore, model.sourValue)
	}
}

// ---- WastewaterGatewayModel tests ----

// TestWastewaterGatewayModel_Name verifies model identifier.
func TestWastewaterGatewayModel_Name(t *testing.T) {
	store := makeGatewayStore()
	model := NewWastewaterGatewayModel(store, nil)
	if model.Name() != "ww-gateway" {
		t.Errorf("Name() = %q, want %q", model.Name(), "ww-gateway")
	}
}

// TestWastewaterGatewayModel_TXRXCounters_IncrementByTwo verifies that TX and RX counters
// increment by 2 per tick (two SLC-500 PLCs at unit IDs 1 and 2 behind the gateway).
func TestWastewaterGatewayModel_TXRXCounters_IncrementByTwo(t *testing.T) {
	store := makeGatewayStore()
	model := NewWastewaterGatewayModel(store, nil)

	txBefore := readGatewayReg(t, store, gatewayRegSerialTX)
	rxBefore := readGatewayReg(t, store, gatewayRegSerialRX)

	model.Tick()

	txAfter := readGatewayReg(t, store, gatewayRegSerialTX)
	rxAfter := readGatewayReg(t, store, gatewayRegSerialRX)

	if int(txAfter)-int(txBefore) != 2 {
		t.Errorf("ww-gateway TX delta: got %d, want 2 (two SLC-500 PLCs behind gateway)", int(txAfter)-int(txBefore))
	}
	if int(rxAfter)-int(rxBefore) != 2 {
		t.Errorf("ww-gateway RX delta: got %d, want 2 (two SLC-500 PLCs behind gateway)", int(rxAfter)-int(rxBefore))
	}
}

// ---- CellularModemModel tests ----

// TestCellularModemModel_Name verifies model identifier.
func TestCellularModemModel_Name(t *testing.T) {
	store := makeCellularStore()
	model := NewCellularModemModel(store, nil)
	if model.Name() != "ww-cellular-modem" {
		t.Errorf("Name() = %q, want %q", model.Name(), "ww-cellular-modem")
	}
}

// TestCellularModemModel_WANStatus_StartsConnected verifies the initial WAN state is connected.
func TestCellularModemModel_WANStatus_StartsConnected(t *testing.T) {
	store := makeCellularStore()
	model := NewCellularModemModel(store, nil)

	model.Tick()

	wanRaw := readHoldingRaw(t, store, cellRegWANStatus)
	// Initial state should be connected (2) immediately after first tick.
	if wanRaw != cellWANConnected {
		t.Errorf("WAN status: got %d, want %d (connected)", wanRaw, cellWANConnected)
	}
}

// TestCellularModemModel_DataUsage_Accumulates verifies data usage internal state increments.
// At 0.0001 GB/tick, after 5000 ticks the internal accumulator should be ~0.5 GB.
// The register value (integer floor) shows 0 until 1 GB is reached; test the internal state.
func TestCellularModemModel_DataUsage_Accumulates(t *testing.T) {
	store := makeCellularStore()
	model := NewCellularModemModel(store, nil)

	// Tick 5000 times to accumulate measurable data.
	for i := 0; i < 5000; i++ {
		model.Tick()
	}

	// Internal accumulator should be ~0.5 GB (5000 * 0.0001).
	if model.dataUsageGB < 0.4 {
		t.Errorf("data usage internal state did not accumulate: got %v GB after 5000 ticks", model.dataUsageGB)
	}

	// Tick to 2 GB and verify register shows >= 1.
	for i := 0; i < 15001; i++ {
		model.Tick()
	}

	usageRaw := readHoldingRaw(t, store, cellRegDataUsageGB)
	if usageRaw < 1 {
		t.Errorf("data usage register should show >= 1 GB after 20000 ticks: got %d GB", usageRaw)
	}
}

// TestCellularModemModel_UptimeWraps_At65535 verifies uptime counter wraps at 65535.
func TestCellularModemModel_UptimeWraps_At65535(t *testing.T) {
	store := makeCellularStore()
	model := NewCellularModemModel(store, nil)

	// Force uptime to near wrap point.
	model.uptimeSecs = 65535

	model.Tick()

	// After one more tick, should have wrapped to 0.
	if model.uptimeSecs != 0 {
		t.Errorf("uptime should wrap to 0 at 65535: got %d", model.uptimeSecs)
	}
}

// TestCellularModemModel_FixedRegisters_StaticValues verifies that fixed-value registers
// (NAT mode, VPN status, client count) remain at their configured values after ticks.
func TestCellularModemModel_FixedRegisters_StaticValues(t *testing.T) {
	store := makeCellularStore()
	model := NewCellularModemModel(store, nil)

	for i := 0; i < 10; i++ {
		model.Tick()
	}

	natMode := readHoldingRaw(t, store, cellRegNATMode)
	vpnStatus := readHoldingRaw(t, store, cellRegVPNStatus)
	lanClients := readHoldingRaw(t, store, cellRegActiveLANClients)

	if natMode != cellNATEnabled {
		t.Errorf("nat_mode: got %d, want %d (always enabled)", natMode, cellNATEnabled)
	}
	if vpnStatus != cellVPNActive {
		t.Errorf("vpn_tunnel_status: got %d, want %d (vendor VPN active)", vpnStatus, cellVPNActive)
	}
	if lanClients != cellActiveLANClients {
		t.Errorf("active_lan_clients: got %d, want %d", lanClients, cellActiveLANClients)
	}
}

// TestWastewaterModels_ImplementProcessModelInterface verifies that all five wastewater models
// implement the ProcessModel interface. This is a compile-time check enforced at test time.
func TestWastewaterModels_ImplementProcessModelInterface(t *testing.T) {
	var _ ProcessModel = NewInfluentModel(makeInfluentStore(), nil)
	var _ ProcessModel = NewEffluentModel(makeEffluentStore(), nil)
	var _ ProcessModel = NewAerationModel(makeAerationStore(), nil)
	var _ ProcessModel = NewWastewaterGatewayModel(makeGatewayStore(), nil)
	var _ ProcessModel = NewCellularModemModel(makeCellularStore(), nil)
}
