package process

import (
	"math/rand"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// InfluentModel simulates the SLC-500 ww-plc-01 influent screening and primary treatment process.
// SLC-500/05 with ww-influent variant (one-based addressing via ProSoft MVI46-MCM Modbus module).
//
// Simulates bar screening, grit removal, primary clarification, and sludge withdrawal.
// All holding registers use 16-bit scaled integers (N7: integer file convention).
// IEEE 754 floats are NOT used -- ProSoft MVI46-MCM maps N7: files, not F8: float files.
//
// ATTACK SURFACES (educational):
//   - influent_flow_total (addr 2): writable reset attacked to spoof permit compliance records
//   - sludge_pump_cycle_time (addr 7): writing 0 stops sludge withdrawal; carryover in hours
//
// PROTOTYPE-DEBT: [td-influent-041] No cross-model coupling to effluent flow.
// Influent flow does not drive effluent flow. Independent simulation per TD-041.
// TODO-FUTURE: Add shared process state bus (Beta 0.5+).
//
// Implemented in SOW-018.0.

// influent holding register addresses (one-based, SLC-500 ww-influent variant).
const (
	inflRegFlowRate     = 1  // GPM, read-only
	inflRegFlowTotal    = 2  // gal x1000, writable (permit reset attack surface)
	inflRegBarScreenDP  = 3  // in WC, read-only
	inflRegGritLevel    = 4  // %, read-only
	inflRegClarLevel    = 5  // ft, read-only
	inflRegSludgeBlankt = 6  // ft, read-only
	inflRegPumpCycleSet = 7  // min, writable
	inflRegPH           = 8  // pH, read-only
	inflRegTemp         = 9  // degF, read-only
)

// influent coil addresses (one-based, SLC-500 ww-influent variant).
const (
	inflCoilBarScreen   = 1 // bar_screen_motor, writable
	inflCoilGritPump    = 2 // grit_pump, writable
	inflCoilSludgePump  = 3 // primary_sludge_pump, writable
	inflCoilInflGate    = 4 // influent_gate, writable
)

// influent simulation constants.
const (
	inflDiurnalHalfTicks = 28800 // ticks per half-cycle: 8 hours peak to valley (28800 = 8 * 3600)
	inflBaseFlowGPM      = 2000.0 // base influent flow (typical small municipal WWTP)
	inflFlowAmplitude    = 0.30   // ±30% diurnal variation
	inflPHExcursionProb = 0.001 // 0.1%/tick chance of industrial discharge pH excursion
)

// InfluentModel tracks simulation state for the SLC-500 influent screening and primary treatment.
type InfluentModel struct {
	store         *mbstore.RegisterStore
	profile       *device.DeviceProfile
	diurnalTick   int     // current position in 24-hour diurnal cycle
	barScreenDP   float64 // current bar screen differential pressure (in WC)
	gritLevel     float64 // current grit chamber level (%)
	clarLevel     float64 // current clarifier level (ft)
	sludgeBlanket float64 // current sludge blanket depth (ft)
	phCurrent     float64 // current influent pH
	tempCurrent   float64 // current influent temperature (degF)
	phExcursion   int     // ticks remaining in pH excursion event
	phTarget      float64 // pH excursion target (5.5 or 9.0)
	flowTotal     float64 // accumulated daily flow (gal x1000), wraps at 99999
}

// NewInfluentModel creates an InfluentModel for the given store and profile.
func NewInfluentModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *InfluentModel {
	return &InfluentModel{
		store:         store,
		profile:       profile,
		barScreenDP:   0.3,
		gritLevel:     5.0,
		clarLevel:     2.5,
		sludgeBlanket: 1.0,
		phCurrent:     7.0,
		tempCurrent:   61.0,
		phTarget:      7.0,
	}
}

// Name returns the model identifier for logging.
func (m *InfluentModel) Name() string { return "ww-influent" }

// Tick advances the influent simulation by one second.
func (m *InfluentModel) Tick() {
	coils, err := m.store.ReadCoils(inflCoilBarScreen, 4)
	if err != nil {
		return
	}
	pumpCycleRaw, err := m.store.ReadHolding(inflRegPumpCycleSet, 1)
	if err != nil {
		return
	}

	flowRate := m.computeFlowRate()
	m.updateBarScreen(coils[0])
	m.updateGrit(coils[1])
	m.updateClarifier(flowRate, coils[2])
	m.updateSludgeBlanket(coils[2], pumpCycleRaw[0])
	m.updatePH()
	m.updateTemp()
	m.updateFlowTotal(flowRate)
	m.writeRegisters(flowRate, pumpCycleRaw[0])
	m.diurnalTick++
}

// computeFlowRate returns the current influent flow in GPM using a diurnal pattern.
// Two-peak daily cycle (morning and evening) modeled as a parabolic approximation over
// half-period intervals. Amplitude: ±30% of base 2000 GPM.
func (m *InfluentModel) computeFlowRate() float64 {
	halfCycle := m.diurnalTick % inflDiurnalHalfTicks
	peakFraction := float64(halfCycle) / float64(inflDiurnalHalfTicks)
	// Parabolic approximation: 1.0 at start/end of half-period, minimum at midpoint.
	// 1 - 4*f*(1-f) = 1 at f=0 and f=1; -1 at f=0.5.
	diurnalFactor := 1.0 - 4.0*peakFraction*(1.0-peakFraction)
	flow := inflBaseFlowGPM * (1.0 + inflFlowAmplitude*diurnalFactor)
	return clamp(addNoise(flow, 5000.0), 0, 5000)
}

// updateBarScreen advances bar screen differential pressure.
// Accumulates when rake motor is off (debris builds up); ramps toward 0.3 in WC when on (raking).
// Never reaches 0 -- mechanical rake does not clear all debris.
func (m *InfluentModel) updateBarScreen(rakeOn bool) {
	if rakeOn {
		m.barScreenDP = ramp(m.barScreenDP, 0.3, 0.02)
	} else {
		m.barScreenDP = clamp(m.barScreenDP+0.0001, 0, 12)
	}
}

// updateGrit advances grit chamber level.
// Accumulates slowly (0.01% per tick); resets when grit pump runs.
func (m *InfluentModel) updateGrit(gritPumpOn bool) {
	if gritPumpOn {
		m.gritLevel = ramp(m.gritLevel, 0.0, 2.0)
	} else {
		m.gritLevel = clamp(m.gritLevel+0.01, 0, 100)
	}
}

// updateClarifier advances clarifier level based on flow and sludge pump activity.
func (m *InfluentModel) updateClarifier(flowRate float64, sludgePumpOn bool) {
	// Flow influences level: higher flow raises clarifier level slightly.
	flowInfluence := (flowRate - inflBaseFlowGPM) * 0.0001
	target := 2.5 + flowInfluence
	if sludgePumpOn {
		target -= 0.3 // sludge withdrawal lowers clarifier level
	}
	m.clarLevel = clamp(drift(m.clarLevel, target, 0.001), 0, 5)
}

// updateSludgeBlanket advances primary sludge blanket depth.
// Rises 0.001 ft per tick without pumping; falls when primary_sludge_pump is on
// and sludge_pump_cycle_time setpoint is non-zero.
func (m *InfluentModel) updateSludgeBlanket(sludgePumpOn bool, pumpCycleRaw uint16) {
	pumpCycleEng := toEng(pumpCycleRaw, 0, 120)
	pumpActive := sludgePumpOn && pumpCycleEng > 0
	if pumpActive {
		m.sludgeBlanket = clamp(m.sludgeBlanket-0.003, 0, 8)
	} else {
		m.sludgeBlanket = clamp(m.sludgeBlanket+0.001, 0, 8)
	}
}

// updatePH advances influent pH with noise and occasional excursion events.
// [OT-REVIEW] pH 5.5/9.0 excursion models industrial discharge event.
func (m *InfluentModel) updatePH() {
	if m.phExcursion > 0 {
		m.phExcursion--
		m.phCurrent = drift(m.phCurrent, m.phTarget, 0.005)
		if m.phExcursion == 0 {
			m.phTarget = 7.0
		}
		return
	}
	m.phCurrent = clamp(addNoise(drift(m.phCurrent, 7.0, 0.01), 14.0), 0, 14)
	if rand.Float64() < inflPHExcursionProb { //nolint:gosec -- educational simulation
		m.phExcursion = 300 // 5-minute excursion
		if rand.Float64() < 0.5 { //nolint:gosec
			m.phTarget = 5.5 // acid industrial discharge
		} else {
			m.phTarget = 9.0 // caustic industrial discharge
		}
	}
}

// updateTemp advances influent temperature with diurnal drift (50-72 degF day/night cycle).
// Half-period is 28800 ticks (8 hours), creating a realistic day/night influent temperature variation.
func (m *InfluentModel) updateTemp() {
	// Alternate between warm (72 degF) and cool (50 degF) targets every 28800 ticks.
	tempPeriod := (m.diurnalTick / inflDiurnalHalfTicks) % 2
	var tempTarget float64
	if tempPeriod == 0 {
		tempTarget = 72.0
	} else {
		tempTarget = 50.0
	}
	m.tempCurrent = clamp(addNoise(drift(m.tempCurrent, tempTarget, 0.001), 90.0), 40, 90)
}

// updateFlowTotal accumulates daily influent flow in gal x1000.
// Wraps at 99999 (register scale_max) modeling the 16-bit scaled integer limit.
func (m *InfluentModel) updateFlowTotal(flowRate float64) {
	// Increment by flow this tick: GPM * 1 second = gallons/60, then convert to gal x1000.
	incrementKgal := flowRate / 60.0 / 1000.0
	m.flowTotal += incrementKgal
	if m.flowTotal >= 99999.0 {
		m.flowTotal -= 99999.0
	}
}

// writeRegisters writes all influent holding registers to the store (one-based, start at addr 1).
func (m *InfluentModel) writeRegisters(flowRate float64, pumpCycleRaw uint16) {
	regs := []uint16{
		toRaw(flowRate, 0, 5000),
		toRaw(m.flowTotal, 0, 99999),
		toRaw(m.barScreenDP, 0, 12),
		toRaw(m.gritLevel, 0, 100),
		toRaw(m.clarLevel, 0, 5),
		toRaw(m.sludgeBlanket, 0, 8),
		pumpCycleRaw, // pass through writable setpoint unchanged
		toRaw(m.phCurrent, 0, 14),
		toRaw(m.tempCurrent, 40, 90),
	}
	_ = m.store.WriteHoldingInternal(inflRegFlowRate, regs)
}
