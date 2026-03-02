package process

import (
	"math/rand"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// CompressorModel simulates the gas turbine-driven centrifugal compressor for ps-plc-01.
// CompactLogix L33ER with compressor-control variant (zero-based addressing).
//
// Startup sequence: stopped -> pre-lube (30 ticks, lube oil ramps 0->25 PSIG) ->
// running (speed ramps 450 RPM/tick to 4500 RPM, surge protection forced until 3150 RPM).
// E-stop: immediate fuel cutoff + shaft brake, speed goes to 0 in current tick.
// Trip: latches on safety interlock violation, clears after 60 ticks (operator reset).
//
// PROTOTYPE-DEBT: [td-pipeline-016] Cross-device coupling not implemented.
// Suction pressure simulates independently from StationMonitorModel inlet pressure.
// TODO-FUTURE: Shared process state bus for pipeline station devices.
//
// Implemented in SOW-009.0.

// compressor holding register addresses (zero-based, CompactLogix compressor-control variant).
const (
	compRegSpeed             = 0 // RPM, read-only
	compRegSuctionPressure   = 1 // PSIG, read-only
	compRegDischargePressure = 2 // PSIG, read-only
	compRegBearingDE         = 3 // degF, read-only
	compRegBearingNDE        = 4 // degF, read-only
	compRegVibration         = 5 // mils, read-only
	compRegLubeOilPressure   = 6 // PSIG, read-only
	compRegLubeOilTemp       = 7 // degF, read-only
	compRegStatusWord        = 8 // bitmask, read-only
)

// compressor coil addresses (zero-based, CompactLogix compressor-control variant).
const (
	compCoilRunCmd       = 0 // compressor_run_cmd, writable
	compCoilEStop        = 1 // compressor_e_stop, read-only (hardwired safety)
	compCoilSurgeProtect = 2 // surge_protection_active, read-only
	compCoilTripped      = 3 // compressor_tripped, read-only
)

// compressor operational constants.
const (
	compMaxSpeed         = 4500.0 // RPM (gas turbine-driven centrifugal compressor rated speed)
	compSpeedRampRate    = 450.0  // RPM/tick (10 ticks to full speed)
	compSurgeThreshold   = 3150.0 // RPM (70% of max; above this, probabilistic surge control)
	compPreLubeTicks     = 30     // ticks for lube oil pre-lube before speed ramp begins
	compTripClearTicks   = 60     // ticks before tripped status clears (local operator reset)
	compSurgeLatchTicks  = 10     // ticks surge protection latches when probabilistically triggered
	compVibSpikeDecay    = 20     // ticks for vibration spike to decay back to baseline
	compTripVibMils      = 5.0   // mils -- trip threshold per API 670
	compAlarmVibMils     = 3.0   // mils -- alarm threshold per API 670
	compTripBearingTemp  = 300.0 // degF -- trip threshold
	compAlarmBearingTemp = 275.0 // degF -- alarm threshold
	compTripLubeOilPSI   = 10.0  // PSIG -- low lube oil trip threshold
	compAlarmLubeOilPSI  = 15.0  // PSIG -- low lube oil alarm threshold
	compReadyLubeOilPSI  = 20.0  // PSIG -- minimum to satisfy "ready" permissive
	compPreLubeMaxPSI    = 25.0  // PSIG -- target lube oil pressure at end of pre-lube phase
)

// compressor startup state machine values.
const (
	compStateStopped  = 0 // stopped, no active sequence
	compStatePreLube  = 1 // pre-lube phase (timer counting down)
	compStateRunning  = 2 // at operating speed or ramping up
	compStateRampDown = 3 // ramping down after stop command
)

// CompressorModel tracks compressor operating state for the compressor-control variant.
type CompressorModel struct {
	store           *mbstore.RegisterStore
	profile         *device.DeviceProfile
	compressorSpeed float64 // RPM, current actual speed
	startupState    int     // compStateStopped/PreLube/Running/RampDown
	preLubeTimer    int     // ticks remaining in pre-lube phase
	surgeTimer      int     // ticks remaining in surge protection latch
	tripTimer       int     // ticks remaining before trip clears (operator reset)
	vibSpikeTimer   int     // ticks remaining in vibration spike decay
	vibration       float64 // mils pk-pk
	bearingDETemp   float64 // degF, drive-end bearing
	bearingNDETemp  float64 // degF, non-drive-end bearing
	lubeOilPressure float64 // PSIG
	lubeOilTemp     float64 // degF
	tripped         bool
}

// NewCompressorModel creates a CompressorModel for the given store and profile.
// Bearing temperatures initialize to 175/160 degF (residual heat from prior operation).
// [OT-REVIEW] Compressor station bearings retain heat after shutdown.
func NewCompressorModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *CompressorModel {
	return &CompressorModel{
		store:          store,
		profile:        profile,
		bearingDETemp:  175.0,
		bearingNDETemp: 160.0,
		lubeOilTemp:    80.0,
	}
}

// Name returns the model identifier for logging.
func (m *CompressorModel) Name() string { return "compressor-control" }

// Tick advances the compressor simulation by one second.
func (m *CompressorModel) Tick() {
	coils, err := m.store.ReadCoils(compCoilRunCmd, 4)
	if err != nil {
		return
	}
	runCmd := coils[compCoilRunCmd]
	eStop := coils[compCoilEStop]

	m.updateStateMachine(runCmd, eStop)
	m.updateVibration()
	m.updateBearingTemps()
	m.updateLubeOil()
	m.checkSafetyInterlocks(eStop)
	m.updateSurgeProtection()
	m.writeRegisters(eStop)
	m.writeCoils()
}

// updateStateMachine manages the startup sequence, speed ramp, and e-stop.
// Transitions: Stopped -> PreLube (run_cmd=true) -> Running -> RampDown (run_cmd=false) -> Stopped.
// E-stop overrides all states with immediate zero-speed. Not a ramp.
// [OT-REVIEW] E-stop = simultaneous fuel cutoff + shaft brake.
func (m *CompressorModel) updateStateMachine(runCmd, eStop bool) {
	if m.tripped && m.tripTimer > 0 {
		m.tripTimer--
		if m.tripTimer == 0 {
			m.tripped = false
		}
	}
	if eStop {
		m.compressorSpeed = 0
		m.startupState = compStateStopped
		m.preLubeTimer = 0
		return
	}
	if m.tripped {
		m.compressorSpeed = clamp(m.compressorSpeed-compSpeedRampRate, 0, compMaxSpeed)
		if m.compressorSpeed == 0 {
			m.startupState = compStateStopped
		}
		return
	}
	m.advanceStartupState(runCmd)
}

// advanceStartupState applies the state machine transitions for non-tripped, non-estop states.
func (m *CompressorModel) advanceStartupState(runCmd bool) {
	switch m.startupState {
	case compStateStopped:
		if runCmd {
			m.startupState = compStatePreLube
			m.preLubeTimer = compPreLubeTicks
		}
	case compStatePreLube:
		if !runCmd {
			m.startupState = compStateStopped
			m.preLubeTimer = 0
			return
		}
		m.preLubeTimer--
		if m.preLubeTimer <= 0 {
			m.preLubeTimer = 0
			m.startupState = compStateRunning
		}
	case compStateRunning:
		if runCmd {
			m.compressorSpeed = clamp(m.compressorSpeed+compSpeedRampRate, 0, compMaxSpeed)
		} else {
			m.startupState = compStateRampDown
		}
	case compStateRampDown:
		m.compressorSpeed = clamp(m.compressorSpeed-compSpeedRampRate, 0, compMaxSpeed)
		if m.compressorSpeed == 0 {
			m.startupState = compStateStopped
		}
		if runCmd {
			m.startupState = compStateRunning
		}
	}
}

// updateVibration applies drift, noise, and random spike events.
// [OT-REVIEW] API 670: vibration measured as shaft displacement (mils pk-pk) by proximity probes.
func (m *CompressorModel) updateVibration() {
	if m.compressorSpeed == 0 {
		m.vibration = 0
		m.vibSpikeTimer = 0
		return
	}
	if m.vibSpikeTimer > 0 {
		m.vibSpikeTimer--
		// Decay spike linearly from 4.0 back toward 1.5 mils over compVibSpikeDecay ticks.
		m.vibration = 1.5 + float64(m.vibSpikeTimer)/float64(compVibSpikeDecay)*2.5
		return
	}
	m.vibration = clamp(addNoise(drift(m.vibration, 1.5, 0.05), 10.0), 0, 10)
	if rand.Float64() < 0.0001 { //nolint:gosec -- educational simulation
		m.vibration = 4.0
		m.vibSpikeTimer = compVibSpikeDecay
	}
}

// updateBearingTemps applies drift toward running/stopped targets for each bearing.
// Drive-end bearing is 10-20 degF hotter (proximity to gas turbine). [OT-REVIEW]
func (m *CompressorModel) updateBearingTemps() {
	if m.compressorSpeed > 0 {
		m.bearingDETemp = clamp(addNoise(drift(m.bearingDETemp, 180.0, 0.3), 350.0), 0, 350)
		m.bearingNDETemp = clamp(addNoise(drift(m.bearingNDETemp, 165.0, 0.3), 350.0), 0, 350)
	} else {
		m.bearingDETemp = clamp(drift(m.bearingDETemp, 80.0, 0.1), 0, 350)
		m.bearingNDETemp = clamp(drift(m.bearingNDETemp, 80.0, 0.1), 0, 350)
	}
}

// updateLubeOil manages lube oil pressure and temperature based on startup state.
// Pre-lube: linear ramp 0->25 PSIG over compPreLubeTicks ticks.
// Running: drifts toward 35 PSIG nominal.
// Stopped: decays to 0.
func (m *CompressorModel) updateLubeOil() {
	switch {
	case m.compressorSpeed > 0:
		m.lubeOilPressure = clamp(addNoise(drift(m.lubeOilPressure, 35.0, 0.5), 100.0), 0, 100)
		m.lubeOilTemp = clamp(addNoise(drift(m.lubeOilTemp, 130.0, 0.2), 200.0), 0, 200)
	case m.startupState == compStatePreLube:
		// Fraction complete = (compPreLubeTicks - preLubeTimer) / compPreLubeTicks
		// At timer=30 (start): fraction=0, pressure=0. At timer=0 (end): fraction=1, pressure=25 PSIG.
		fraction := 1.0 - float64(m.preLubeTimer)/float64(compPreLubeTicks)
		m.lubeOilPressure = fraction * compPreLubeMaxPSI
		m.lubeOilTemp = drift(m.lubeOilTemp, 80.0, 0.1)
	default:
		m.lubeOilPressure = clamp(drift(m.lubeOilPressure, 0.0, 1.0), 0, 100)
		m.lubeOilTemp = clamp(drift(m.lubeOilTemp, 80.0, 0.1), 0, 200)
	}
}

// checkSafetyInterlocks trips the compressor on threshold violations.
func (m *CompressorModel) checkSafetyInterlocks(eStop bool) {
	if m.tripped {
		return
	}
	running := m.compressorSpeed > 0
	highVib := m.vibration >= compTripVibMils
	highTemp := m.bearingDETemp >= compTripBearingTemp || m.bearingNDETemp >= compTripBearingTemp
	lowOil := running && m.lubeOilPressure < compTripLubeOilPSI
	if highVib || highTemp || lowOil || eStop {
		m.tripped = true
		m.tripTimer = compTripClearTicks
		m.compressorSpeed = 0
		m.startupState = compStateStopped
	}
}

// updateSurgeProtection manages the anti-surge recycle valve latch.
// Forced active during acceleration through surge zone (below 70% of rated speed).
// Probabilistic (0.05%/tick) when above the surge threshold. 10-tick latch.
func (m *CompressorModel) updateSurgeProtection() {
	belowThreshold := m.compressorSpeed > 0 && m.compressorSpeed < compSurgeThreshold
	if belowThreshold {
		m.surgeTimer = 1 // force active; will be reset each tick while below threshold
		return
	}
	if m.surgeTimer > 0 {
		m.surgeTimer--
		return
	}
	if m.compressorSpeed > 0 && rand.Float64() < 0.0005 { //nolint:gosec
		m.surgeTimer = compSurgeLatchTicks
	}
}

// buildCompressorStatusWord constructs the status bitmask per FR-7.
// bit0=running, bit1=ready, bit2=tripped, bit3=surge-detected,
// bit4=high-vibration, bit5=high-bearing-temp, bit6=low-lube-oil, bit7=ESD-active.
func (m *CompressorModel) buildCompressorStatusWord(eStop bool) uint16 {
	running := m.compressorSpeed > 0
	var sw uint16
	if running {
		sw |= 1 << 0
	}
	if m.lubeOilPressure >= compReadyLubeOilPSI && !m.tripped {
		sw |= 1 << 1
	}
	if m.tripped {
		sw |= 1 << 2
	}
	if m.surgeTimer > 0 {
		sw |= 1 << 3
	}
	if m.vibration >= compAlarmVibMils {
		sw |= 1 << 4
	}
	if m.bearingDETemp >= compAlarmBearingTemp || m.bearingNDETemp >= compAlarmBearingTemp {
		sw |= 1 << 5
	}
	if running && m.lubeOilPressure < compAlarmLubeOilPSI {
		sw |= 1 << 6
	}
	if eStop {
		sw |= 1 << 7
	}
	return sw
}

// writeRegisters writes all compressor holding registers to the store (zero-based, addr 0-8).
func (m *CompressorModel) writeRegisters(eStop bool) {
	loadFactor := m.compressorSpeed / compMaxSpeed
	suctionPressure := clamp(addNoise(600.0-loadFactor*15.0, 1500.0), 0, 1500)
	compressionRatio := 1.0 + loadFactor*0.5 // 1.0 stopped -> 1.5 at rated speed
	dischargePressure := suctionPressure * compressionRatio
	if m.surgeTimer > 0 && m.compressorSpeed > 0 {
		dischargePressure *= 0.95 // 5% reduction during surge protection event
	}
	regs := []uint16{
		toRaw(m.compressorSpeed, 0, 6000),
		toRaw(clamp(suctionPressure, 0, 1500), 0, 1500),
		toRaw(clamp(dischargePressure, 0, 1500), 0, 1500),
		toRaw(m.bearingDETemp, 0, 350),
		toRaw(m.bearingNDETemp, 0, 350),
		toRaw(m.vibration, 0, 10),
		toRaw(m.lubeOilPressure, 0, 100),
		toRaw(m.lubeOilTemp, 0, 200),
		m.buildCompressorStatusWord(eStop),
	}
	_ = m.store.WriteHoldingInternal(compRegSpeed, regs)
}

// writeCoils writes the read-only coil outputs for surge protection and trip status.
// The model never writes compCoilRunCmd (writable by operator) or compCoilEStop (hardwired).
func (m *CompressorModel) writeCoils() {
	_ = m.store.WriteCoilsInternal(compCoilSurgeProtect, []bool{m.surgeTimer > 0, m.tripped})
}
