package process

import (
	"math/rand"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// Manufacturing process models: Line A (SLC-500) and Cooling (Modicon 984).
//
// Both devices use one-based Modbus addressing. All register addresses in this file
// are the wire-level addresses as defined in the device atoms.
//
// PROTOTYPE-DEBT: [td-mfg-015] Jam and e-stop probabilities are hardcoded.
// TODO-FUTURE: Drive from device atom configuration when event simulation is enhanced.

// lineA register addresses (one-based, matching SLC-500 mfg-line-a variant).
const (
	lineARegConveyorSpeed = 1 // ft/min, writable setpoint
	lineARegMotorCurrent  = 2 // A, read-only
	lineARegProductCount  = 3 // units, read-only (counter)
	lineARegRejectCount   = 4 // units, read-only (counter)
	lineARegLineTemp      = 5 // degF, read-only
	lineARegCycleTime     = 6 // s, read-only
	lineARegStatusWord    = 7 // bitmask, read-only
)

// lineA coil addresses (one-based, matching SLC-500 mfg-line-a variant).
const (
	lineACoilConveyorRun = 1 // conveyor_run, writable
	lineACoilDirection   = 2 // conveyor_direction, writable
	lineACoilEStop       = 3 // e_stop_active, read-only (hardwired safety circuit)
	lineACoilJam         = 4 // jam_detected, read-only
)

// lineAShiftTicks is the number of ticks in an 8-hour shift (8 * 3600 = 28800).
// [OT-REVIEW] Shift-change reset is via PLC ladder logic timer, not Modbus write.
const lineAShiftTicks = 28800

// LineAModel simulates the SLC-500 Line A conveyor and assembly process.
type LineAModel struct {
	store        *mbstore.RegisterStore
	profile      *device.DeviceProfile
	conveyorRamp float64
	jamTimer     int
	eStopTimer   int
	productCount uint16
	rejectCount  uint16
	productTimer int
	shiftTick    int
	lineTempCurr float64
}

// NewLineAModel creates a LineAModel for the given store and profile.
func NewLineAModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *LineAModel {
	return &LineAModel{
		store:        store,
		profile:      profile,
		lineTempCurr: 72.0,
		productTimer: 3,
	}
}

// Name returns the model identifier for logging.
func (m *LineAModel) Name() string { return "mfg-line-a" }

// Tick advances the Line A simulation by one second.
func (m *LineAModel) Tick() {
	coils, err := m.store.ReadCoils(lineACoilConveyorRun, 4)
	if err != nil {
		return
	}
	speedVals, err := m.store.ReadHolding(lineARegConveyorSpeed, 1)
	if err != nil {
		return
	}
	eStopActive, jamDetected := m.updateFaults(coils[0], coils[2], coils[3])
	running := coils[0] && !eStopActive
	m.conveyorRamp = ramp(m.conveyorRamp, boolToFloat(running), 0.2)
	speedEng := toEng(speedVals[0], 0, 200)
	motorCurrent := m.computeMotorCurrent(running, speedEng, jamDetected)
	m.updateCounters(running, jamDetected)
	m.updateLineTemp(running, speedEng)
	cycleTime := m.computeCycleTime(running, jamDetected)
	statusWord := m.buildStatusWord(running, jamDetected, eStopActive)
	m.writeRegisters(speedVals[0], motorCurrent, cycleTime, statusWord)
	_ = m.store.WriteCoilsInternal(lineACoilEStop, []bool{eStopActive, jamDetected})
}

// updateFaults manages e-stop and jam latches, returning current state.
func (m *LineAModel) updateFaults(conveyorOn, eStopCoil, jamCoil bool) (eStopActive, jamDetected bool) {
	// E-stop: 0.05% per tick (~every 33 minutes). [OT-REVIEW]
	if m.eStopTimer == 0 && !eStopCoil {
		if rand.Float64() < 0.0005 { //nolint:gosec
			m.eStopTimer = 30
		}
	}
	if m.eStopTimer > 0 {
		m.eStopTimer--
		eStopActive = true
	}
	// Jam: 0.1% per tick when running (~every 17 minutes). [OT-REVIEW]
	if m.jamTimer == 0 && conveyorOn && !eStopActive {
		if rand.Float64() < 0.001 { //nolint:gosec
			m.jamTimer = 20
		}
	}
	if m.jamTimer > 0 {
		m.jamTimer--
		jamDetected = true
	}
	return
}

// computeMotorCurrent returns motor current in Amperes.
func (m *LineAModel) computeMotorCurrent(running bool, speedEng float64, jammed bool) float64 {
	if !running && m.conveyorRamp == 0 {
		return 0
	}
	current := 5.0 + speedEng/40.0
	if jammed {
		current += 15.0
	}
	return clamp(addNoise(current, 30.0), 0, 30)
}

// updateCounters increments product and reject counts, applying shift-change reset.
func (m *LineAModel) updateCounters(running, jammed bool) {
	m.shiftTick++
	if m.shiftTick >= lineAShiftTicks {
		m.shiftTick = 0
		m.productCount = 0
		m.rejectCount = 0
	}
	if running && !jammed {
		m.productTimer--
		if m.productTimer <= 0 {
			m.productTimer = 3
			m.productCount++ // wraps naturally on uint16 overflow (FR-6)
			if rand.Float64() < 0.02 { //nolint:gosec
				m.rejectCount++
			}
		}
	}
}

// updateLineTemp advances line temperature drift based on running state.
func (m *LineAModel) updateLineTemp(running bool, speedEng float64) {
	target := 72.0
	if running {
		target = 72.0 + speedEng*0.05
		m.lineTempCurr += 0.5
	} else {
		m.lineTempCurr -= 0.2
	}
	m.lineTempCurr = clamp(addNoise(drift(m.lineTempCurr, target, 0.1), 200.0), 0, 200)
}

// computeCycleTime returns the current cycle time in seconds.
func (m *LineAModel) computeCycleTime(running, jammed bool) float64 {
	if !running {
		return 0
	}
	t := 45.0
	if jammed {
		t = 55.0
	}
	return clamp(addNoise(t, 120.0), 0, 120)
}

// buildStatusWord constructs the status bitmask.
func (m *LineAModel) buildStatusWord(running, jammed, eStop bool) uint16 {
	var sw uint16
	if running {
		sw |= 1 << 0
	}
	if jammed {
		sw |= 1 << 2
	}
	if eStop {
		sw |= 1 << 3
	}
	return sw
}

// writeRegisters writes all holding registers to the store (one-based, start at addr 1).
func (m *LineAModel) writeRegisters(speedRaw uint16, motorCurrent, cycleTime float64, statusWord uint16) {
	regs := []uint16{
		speedRaw,
		toRaw(motorCurrent, 0, 30),
		m.productCount,
		m.rejectCount,
		toRaw(m.lineTempCurr, 0, 200),
		toRaw(cycleTime, 0, 120),
		statusWord,
	}
	_ = m.store.WriteHoldingInternal(lineARegConveyorSpeed, regs)
}

// boolToFloat converts a bool to 0.0 or 1.0 for ramp target calculations.
func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// cooling register addresses (one-based, matching Modicon 984 mfg-cooling variant).
const (
	coolingRegSupplyTemp   = 1 // degF, read-only
	coolingRegReturnTemp   = 2 // degF, read-only
	coolingRegFlowRate     = 3 // GPM, read-only
	coolingRegPumpPressure = 4 // PSI, read-only
	coolingRegTankLevel    = 5 // %, read-only
	coolingRegSetpointTemp = 6 // degF, writable (attack surface)
	coolingRegPumpRunHours = 7 // hours, read-only (counter, wraps on 16-bit overflow)
)

// cooling coil addresses (one-based, matching Modicon 984 mfg-cooling variant).
const (
	coolingCoilPump1Run   = 1 // pump_1_run, writable
	coolingCoilPump2Run   = 2 // pump_2_run, writable
	coolingCoilLowCoolant = 3 // low_coolant_alarm, read-only
	coolingCoilHighTemp   = 4 // high_temp_alarm, read-only
)

// CoolingModel simulates the Modicon 984 cooling water system.
type CoolingModel struct {
	store             *mbstore.RegisterStore
	profile           *device.DeviceProfile
	pump1Ramp         float64
	pump2Ramp         float64
	supplyTemp        float64
	tankLevel         float64
	runtimeHours      float64
	lowCoolantLatched bool
	highTempLatched   bool
}

// NewCoolingModel creates a CoolingModel for the given store and profile.
func NewCoolingModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *CoolingModel {
	return &CoolingModel{
		store:        store,
		profile:      profile,
		supplyTemp:   65.0,
		tankLevel:    80.0,
		runtimeHours: 2400,
	}
}

// Name returns the model identifier for logging.
func (m *CoolingModel) Name() string { return "mfg-cooling" }

// Tick advances the cooling simulation by one second.
func (m *CoolingModel) Tick() {
	coils, err := m.store.ReadCoils(coolingCoilPump1Run, 4)
	if err != nil {
		return
	}
	setpointVals, err := m.store.ReadHolding(coolingRegSetpointTemp, 1)
	if err != nil {
		return
	}
	pump1On, pump2On := coils[0], coils[1]
	m.pump1Ramp = ramp(m.pump1Ramp, boolToFloat(pump1On), 0.2)
	m.pump2Ramp = ramp(m.pump2Ramp, boolToFloat(pump2On), 0.2)
	setpoint := toEng(setpointVals[0], 40, 80)
	m.supplyTemp = ramp(m.supplyTemp, setpoint, 0.5)
	supplyNoisy := clamp(addNoise(m.supplyTemp, 80.0), 40, 120)
	flowRate := clamp(addNoise((m.pump1Ramp*150+m.pump2Ramp*150), 500.0), 0, 500)
	returnTemp := m.computeReturnTemp(flowRate)
	pumpPressure := clamp(addNoise(10.0+flowRate/10.0, 80.0), 0, 80)
	m.updateTankLevel(pump1On, pump2On)
	m.updateRuntimeHours(pump1On)
	m.updateAlarms(m.tankLevel, returnTemp)
	m.writeRegisters(supplyNoisy, returnTemp, flowRate, pumpPressure, setpointVals[0])
	_ = m.store.WriteCoilsInternal(coolingCoilLowCoolant, []bool{m.lowCoolantLatched, m.highTempLatched})
}

// computeReturnTemp returns the coolant return temperature in degF.
// delta_T ranges from 5-15 degF based on flow (lower flow = higher delta_T).
func (m *CoolingModel) computeReturnTemp(flowRate float64) float64 {
	deltaT := 15.0
	if flowRate > 0 {
		deltaT = 15.0 - (flowRate/300.0)*10.0
	}
	return clamp(addNoise(m.supplyTemp+deltaT, 80.0), 40, 120)
}

// updateTankLevel advances tank level, applying auto-refill when critically low.
func (m *CoolingModel) updateTankLevel(pump1On, pump2On bool) {
	if pump1On || pump2On {
		m.tankLevel -= 0.02
	}
	if m.tankLevel < 10.0 {
		m.tankLevel = 95.0
	}
}

// updateRuntimeHours accumulates pump 1 runtime in fractional hours.
// Counter wraps naturally on uint16 overflow (FR-6, [OT-REVIEW]).
func (m *CoolingModel) updateRuntimeHours(pump1On bool) {
	if pump1On {
		m.runtimeHours += 1.0 / 3600.0
	}
}

// updateAlarms applies hysteresis to low coolant (20%/30%) and high temp (85F/75F) alarms.
// [OT-REVIEW] High temp threshold lowered to 85 degF so setpoint attack triggers alarm.
func (m *CoolingModel) updateAlarms(tankLevel, returnTemp float64) {
	if tankLevel < 20.0 {
		m.lowCoolantLatched = true
	} else if tankLevel > 30.0 {
		m.lowCoolantLatched = false
	}
	if returnTemp > 85.0 {
		m.highTempLatched = true
	} else if returnTemp < 75.0 {
		m.highTempLatched = false
	}
}

// writeRegisters writes all holding registers to the store (one-based, start at addr 1).
func (m *CoolingModel) writeRegisters(supply, returnTemp, flow, pressure float64, setpointRaw uint16) {
	tankNoisy := clamp(addNoise(m.tankLevel, 100.0), 0, 100)
	rawRuntime := uint16(m.runtimeHours)
	regs := []uint16{
		toRaw(supply, 40, 120),
		toRaw(returnTemp, 40, 120),
		toRaw(flow, 0, 500),
		toRaw(pressure, 0, 80),
		toRaw(tankNoisy, 0, 100),
		setpointRaw,
		rawRuntime,
	}
	_ = m.store.WriteHoldingInternal(coolingRegSupplyTemp, regs)
}
