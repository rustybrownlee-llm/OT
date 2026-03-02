package process

import (
	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// AerationModel simulates the CompactLogix L33ER ww-plc-03 aeration blower control process.
// CompactLogix L33ER with ww-aeration variant (zero-based addressing).
//
// The DO PID loop is the primary simulation feature:
//   When do_control_enable coil is set, the model computes error = do_setpoint - dissolved_oxygen.
//   Blower speed setpoint ramps toward a value that drives measured DO toward the setpoint.
//   Response time: blower speed changes ~1% per tick; DO responds over ~60 ticks (1 minute).
//
// VFD teaching point: blower_speed_setpoint (%) is what the PLC commands.
//   blower_speed_feedback (RPM) is what the VFD reports. These are separate registers.
//   A 5-tick lag between setpoint change and feedback response models VFD acceleration time.
//
// ATTACK SURFACES (educational):
//   - do_setpoint (addr 2): PRIMARY -- writing 0.0-0.5 mg/L creates anoxic conditions;
//     PLC reduces blower speed to match; biomass crash takes days to weeks to recover
//   - blower_speed_setpoint (addr 0): SECONDARY -- writing 0% with DO control disabled
//     stalls blowers and creates anoxic conditions directly
//
// PROTOTYPE-DEBT: [td-aeration-041] No cross-model coupling. MLSS does not receive WAS
// rate from EffluentModel. SOUR recovery does not correlate with effluent ammonia. TD-041.
// TODO-FUTURE: Shared process state bus (Beta 0.5+).
//
// Implemented in SOW-018.0.

// aeration holding register addresses (zero-based, CompactLogix ww-aeration variant).
const (
	aerRegBlowerSpSP    = 0  // %, writable setpoint
	aerRegBlowerSpFB    = 1  // RPM, read-only feedback
	aerRegDOSetpoint    = 2  // mg/L, writable
	aerRegDO            = 3  // mg/L, read-only
	aerRegAirFlow       = 4  // SCFM, read-only
	aerRegBlowerDP      = 5  // PSI, read-only
	aerRegInletTemp     = 6  // degF, read-only
	aerRegMotorCurrent  = 7  // A, read-only
	aerRegBasinLevel    = 8  // ft, read-only
	aerRegMLSS          = 9  // mg/L, read-only
	aerRegSOUR          = 10 // mg O2/g MLSS/hr, read-only
	aerRegRunHours      = 11 // hours, read-only (counter)
)

// aeration coil addresses (zero-based, CompactLogix ww-aeration variant).
const (
	aerCoilBlower1Run  = 0 // blower_1_run, writable
	aerCoilBlower2Run  = 1 // blower_2_run, writable
	aerCoilValveAuto   = 2 // aeration_valve_auto, writable
	aerCoilDOControl   = 3 // do_control_enable, writable
)

// aeration simulation constants.
const (
	aerBlowerMaxRPM      = 3600.0 // RPM at 100% speed (typical direct-drive aeration blower)
	aerBlowerRampRate    = 1.0    // %/tick blower speed setpoint change in PID mode
	aerVFDLagTicks       = 5      // ticks for VFD feedback to catch up to setpoint
	aerDOTransferRate    = 0.04   // mg/L per % blower speed per tick (air-to-liquid O2 transfer)
	aerDODemandPerMLSS   = 0.0015 // mg/L O2 consumed per mg/L MLSS per tick (oxygen demand)
	aerMLSSTarget        = 3000.0 // mg/L target mixed liquor suspended solids
	aerMLSSRatePerTick   = 0.001  // % MLSS drift rate per tick
	aerSOURDecayThresh   = 1.0    // mg/L DO below which SOUR begins to decay (anoxic limit)
	aerSOURRecovTicks    = 120    // ticks for SOUR to recover after DO is restored
	aerSOURNormal        = 11.0   // mg O2/g MLSS/hr normal SOUR at steady state
	aerMaxMotorCurrentA  = 180.0  // A at 100% blower speed
	aerBasinLevelTarget  = 12.0   // ft nominal aeration basin water surface depth
)

// AerationModel tracks simulation state for the CompactLogix aeration blower control.
type AerationModel struct {
	store           *mbstore.RegisterStore
	profile         *device.DeviceProfile
	blowerSpeedSP   float64 // % blower speed setpoint (PLC command to VFD)
	blowerSpeedFB   float64 // % blower speed feedback (VFD reported actual)
	vfdLagBuf       [aerVFDLagTicks]float64 // ring buffer for VFD lag simulation
	vfdBufIdx       int     // current write index in VFD lag buffer
	doMeasured      float64 // mg/L dissolved oxygen measurement
	mlss            float64 // mg/L mixed liquor suspended solids
	sourValue       float64 // mg O2/g MLSS/hr specific oxygen uptake rate
	sourDepressedTicks int  // ticks SOUR has been in depressed (anoxic) state
	sourRecovering  bool    // true when SOUR is recovering after DO restoration
	blowerRunHours  float64 // cumulative run hours (blower 1)
	basinLevel      float64 // ft aeration basin water level
	inletTemp       float64 // degF blower inlet air temperature
}

// NewAerationModel creates an AerationModel for the given store and profile.
// Initializes at normal operating steady state: DO 2.0 mg/L, MLSS 3000 mg/L, SOUR 11 mg O2/g/hr.
func NewAerationModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *AerationModel {
	m := &AerationModel{
		store:         store,
		profile:       profile,
		blowerSpeedSP: 60.0,
		blowerSpeedFB: 60.0,
		doMeasured:    2.0,
		mlss:          3000.0,
		sourValue:     aerSOURNormal,
		basinLevel:    aerBasinLevelTarget,
		inletTemp:     65.0,
	}
	// Initialize VFD lag buffer to current speed.
	for i := range m.vfdLagBuf {
		m.vfdLagBuf[i] = 60.0
	}
	return m
}

// Name returns the model identifier for logging.
func (m *AerationModel) Name() string { return "ww-aeration" }

// Tick advances the aeration simulation by one second.
func (m *AerationModel) Tick() {
	coils, err := m.store.ReadCoils(aerCoilBlower1Run, 4)
	if err != nil {
		return
	}
	holdingRegs, err := m.store.ReadHolding(aerRegBlowerSpSP, 3)
	if err != nil {
		return
	}

	blower1On := coils[aerCoilBlower1Run]
	doControlOn := coils[aerCoilDOControl]
	spRaw := holdingRegs[0]  // blower_speed_setpoint
	doSPRaw := holdingRegs[2] // do_setpoint

	blowersRunning := blower1On || coils[aerCoilBlower2Run]
	m.updateBlowerSpeed(blowersRunning, doControlOn, spRaw, doSPRaw)
	m.updateDO(blowersRunning)
	m.updateMLSS()
	m.updateSOUR()
	m.updateRunHours(blower1On)
	m.updateBasinLevel()
	m.updateInletTemp()
	m.writeRegisters(doSPRaw)
}

// updateBlowerSpeed applies the DO PID loop or manual mode to the blower speed setpoint.
// PID: error = do_setpoint - do_measured. Speed ramps at 1%/tick toward a value that minimizes error.
// Response time: ~60 ticks to see DO change from blower adjustment (FR-013).
func (m *AerationModel) updateBlowerSpeed(blowersRunning, doControlOn bool, spRaw, doSPRaw uint16) {
	if !blowersRunning {
		m.blowerSpeedSP = ramp(m.blowerSpeedSP, 0.0, aerBlowerRampRate)
	} else if doControlOn {
		doSetpoint := toEng(doSPRaw, 0, 8)
		doError := doSetpoint - m.doMeasured
		// Proportional: ramp blower speed up when DO is below setpoint, down when above.
		target := m.blowerSpeedSP + doError*5.0 // gain of 5%/%DO error
		target = clamp(target, 20.0, 100.0)     // clamp: never below 20% to avoid stall
		m.blowerSpeedSP = ramp(m.blowerSpeedSP, target, aerBlowerRampRate)
	}
	// else: manual mode -- blower_speed_setpoint written directly by operator (pass-through)

	// VFD feedback lags setpoint by aerVFDLagTicks ticks.
	m.vfdLagBuf[m.vfdBufIdx] = m.blowerSpeedSP
	m.vfdBufIdx = (m.vfdBufIdx + 1) % aerVFDLagTicks
	m.blowerSpeedFB = m.vfdLagBuf[m.vfdBufIdx]
}

// updateDO advances dissolved oxygen measurement based on blower speed and MLSS oxygen demand.
// DO transfer: proportional to blower speed feedback.
// DO demand: proportional to MLSS (biomass oxygen consumption).
func (m *AerationModel) updateDO(blowersRunning bool) {
	oxygenIn := 0.0
	if blowersRunning {
		oxygenIn = m.blowerSpeedFB * aerDOTransferRate
	}
	oxygenDemand := m.mlss * aerDODemandPerMLSS
	m.doMeasured = clamp(addNoise(m.doMeasured+oxygenIn-oxygenDemand, 12.0), 0, 12)
}

// updateMLSS advances mixed liquor suspended solids with slow drift around target.
// [td-aeration-041] No WAS rate coupling from EffluentModel.
func (m *AerationModel) updateMLSS() {
	m.mlss = clamp(addNoise(drift(m.mlss, aerMLSSTarget, aerMLSSTarget*aerMLSSRatePerTick/100.0), 10000.0), 0, 10000)
}

// updateSOUR advances specific oxygen uptake rate calculation.
// SOUR decays toward near-zero when DO < 1.0 mg/L for extended period (anoxic suppression).
// Recovery is slow (~120 ticks) when DO is restored after an anoxic event.
func (m *AerationModel) updateSOUR() {
	if m.doMeasured < aerSOURDecayThresh {
		m.sourDepressedTicks++
		m.sourRecovering = false
		// Progressive depression: SOUR decays as anoxic duration increases
		suppressionFactor := clamp(float64(m.sourDepressedTicks)/300.0, 0, 1.0)
		target := aerSOURNormal * (1.0 - suppressionFactor)
		m.sourValue = drift(m.sourValue, target, 0.05)
	} else if m.sourRecovering {
		// Slow recovery after anoxic event.
		m.sourValue = drift(m.sourValue, aerSOURNormal, aerSOURNormal/float64(aerSOURRecovTicks))
		if m.sourValue >= aerSOURNormal-0.1 {
			m.sourRecovering = false
			m.sourDepressedTicks = 0
		}
	} else if m.sourDepressedTicks > 0 {
		// DO just came back above threshold; start recovery phase.
		m.sourRecovering = true
	} else {
		// Normal operation: SOUR varies around nominal with noise and MLSS influence.
		mlssFactor := m.mlss / aerMLSSTarget
		normalSOUR := aerSOURNormal * mlssFactor
		m.sourValue = clamp(addNoise(drift(m.sourValue, normalSOUR, 0.02), 30.0), 0, 30)
	}
}

// updateRunHours accumulates blower 1 run hours (1 hour per 3600 ticks).
// Counter wraps naturally on uint16 overflow (educational: unpredictable resets after ~65535 hours).
func (m *AerationModel) updateRunHours(blower1On bool) {
	if blower1On {
		m.blowerRunHours += 1.0 / 3600.0
	}
}

// updateBasinLevel advances aeration basin level with slow variation.
func (m *AerationModel) updateBasinLevel() {
	m.basinLevel = clamp(addNoise(drift(m.basinLevel, aerBasinLevelTarget, 0.001), 15.0), 0, 15)
}

// updateInletTemp advances blower inlet air temperature with ambient noise.
func (m *AerationModel) updateInletTemp() {
	m.inletTemp = clamp(addNoise(drift(m.inletTemp, 65.0, 0.005), 120.0), 30, 120)
}

// writeRegisters writes all aeration holding registers to the store (zero-based, start at addr 0).
// blower_speed_setpoint (addr 0) is writable; the model writes its computed SP so the register
// reflects PID output in automatic mode and the last operator write in manual mode.
// doSPRaw is the current do_setpoint value read in Tick() -- passed through unchanged.
func (m *AerationModel) writeRegisters(doSPRaw uint16) {
	effectiveSP := toRaw(m.blowerSpeedSP, 0, 100)
	feedbackRPM := m.blowerSpeedFB / 100.0 * aerBlowerMaxRPM
	airFlowSCFM := m.blowerSpeedFB / 100.0 * 5000.0
	blowerDP := m.blowerSpeedFB / 100.0 * 8.0 // 0-8 PSI range at 100% speed
	motorCurrent := m.blowerSpeedFB / 100.0 * aerMaxMotorCurrentA
	runHoursRaw := uint16(m.blowerRunHours) // wraps naturally on uint16 overflow
	sourNoisy := clamp(addNoise(m.sourValue, 30.0), 0, 30)

	regs := []uint16{
		effectiveSP,
		toRaw(feedbackRPM, 0, aerBlowerMaxRPM),
		doSPRaw, // pass-through writable do_setpoint unchanged
		toRaw(m.doMeasured, 0, 12),
		toRaw(airFlowSCFM, 0, 5000),
		toRaw(blowerDP, 0, 15),
		toRaw(m.inletTemp, 30, 120),
		toRaw(motorCurrent, 0, aerMaxMotorCurrentA),
		toRaw(m.basinLevel, 0, 15),
		toRaw(m.mlss, 0, 10000),
		toRaw(sourNoisy, 0, 30),
		runHoursRaw,
	}
	_ = m.store.WriteHoldingInternal(aerRegBlowerSpSP, regs)
}

// getStoredDOSetpoint reads the do_setpoint register from the store (addr 2, zero-based).
// Returns the raw register value; caller converts with toEng if needed.
// The DO setpoint is operator-writable; the model reads it to drive the PID loop.
func (m *AerationModel) getStoredDOSetpoint() uint16 {
	vals, err := m.store.ReadHolding(aerRegDOSetpoint, 1)
	if err != nil || len(vals) == 0 {
		return toRaw(2.0, 0, 8) // default 2.0 mg/L if read fails
	}
	return vals[0]
}

