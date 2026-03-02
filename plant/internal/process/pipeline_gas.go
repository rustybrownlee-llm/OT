package process

import (
	"math/rand"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// GasAnalysisModel simulates the ABB TotalFlow G5 gas chromatograph interface for ps-fc-01.
// ABB TotalFlow G5 with gas-analysis variant (one-based addressing).
//
// Gas composition registers (methane, ethane, propane, CO2, nitrogen) update only when the
// NGC completes an analysis cycle (every 300-600 ticks, i.e., 5-10 minutes).
// Between cycles, values are static -- stale-but-valid is normal NGC behavior.
// Moisture content updates every tick from a separate continuous sensor.
//
// Analysis cycle state machine: idle (0) -> analyzing (1) -> complete (2, held 5 ticks) -> idle.
// Error state (3) on rare GC hardware fault; triggers gc_alarm latch.
// gc_alarm and moisture_alarm are independent: instrument failure vs gas quality issue.
//
// PROTOTYPE-DEBT: [td-gas-019] Cycle length (300-600 ticks) is hardcoded.
// TODO-FUTURE: Make configurable via device atom metadata.
//
// Implemented in SOW-009.0.

// gas analysis holding register addresses (one-based, ABB TotalFlow G5 gas-analysis variant).
const (
	gasRegMethane     = 1 // %, read-only
	gasRegEthane      = 2 // %, read-only (scale 0-20)
	gasRegPropane     = 3 // %, read-only (scale 0-10)
	gasRegCO2         = 4 // %, read-only (scale 0-5)
	gasRegNitrogen    = 5 // %, read-only (scale 0-10)
	gasRegBTU         = 6 // BTU/SCF, read-only (scale 900-1200)
	gasRegSG          = 7 // SG, read-only (scale 0.50-1.00)
	gasRegMoisture    = 8 // lb/MMSCF, read-only (scale 0-20)
	gasRegCycleStatus = 9 // enum, read-only (0=idle, 1=analyzing, 2=complete, 3=error)
)

// gas analysis coil addresses (one-based, ABB TotalFlow G5 gas-analysis variant).
const (
	gasCoilAnalysisInProgress = 1 // analysis_in_progress, NOT writable [OT-REVIEW]
	gasCoilGCAlarm            = 2 // gc_alarm_active, read-only (GC hardware faults only)
	gasCoilMoistureAlarm      = 3 // moisture_alarm_active, read-only (tariff exceedance)
)

// gas analysis cycle state values (analysis_cycle_status register).
const (
	gasCycleIdle      = 0 // idle between analysis cycles
	gasCycleAnalyzing = 1 // analysis in progress (peak detection phase)
	gasCycleComplete  = 2 // cycle complete, composition values updated
	gasCycleError     = 3 // chromatograph hardware fault
)

// gas analysis constants.
const (
	gasAnalyzeTicks       = 30   // ticks for one analysis cycle (~30 seconds of peak detection)
	gasCycleMinTicks      = 300  // minimum cycle interval (5 minutes)
	gasCycleRandRange     = 300  // additional random interval range (up to 10 minute total)
	gasCompleteLatch      = 5    // ticks status=2 is held before returning to idle [OT-REVIEW]
	gasGCAlarmClear       = 60   // ticks before gc_alarm clears after fault
	gasMoistureTariff     = 7.0  // lb/MMSCF -- pipeline tariff exceedance threshold [OT-REVIEW]
	gasMoistureAlarmClear = 60   // ticks before moisture_alarm clears
)

// GasAnalysisModel tracks NGC analysis state and composition values for the gas-analysis variant.
type GasAnalysisModel struct {
	store              *mbstore.RegisterStore
	profile            *device.DeviceProfile
	methanePct         float64
	ethanePct          float64
	propanePct         float64
	co2Pct             float64
	nitrogenPct        float64
	moistureContent    float64 // lb/MMSCF (updated every tick by continuous sensor)
	btuValue           float64 // BTU/SCF (updated on analysis cycle completion)
	sgValue            float64 // SG (updated on analysis cycle completion)
	cycleTimer         int     // ticks elapsed since last cycle start
	cycleLength        int     // ticks until next analysis begins (randomly chosen)
	analyzeTimer       int     // ticks remaining in current analysis phase
	completeTimer      int     // ticks status=2 is held [OT-REVIEW]
	analysisStatus     int     // current analysis_cycle_status value
	gcAlarmTimer       int     // ticks remaining in GC hardware fault alarm latch
	moistureAlarmTimer int     // ticks remaining in moisture tariff alarm latch
}

// NewGasAnalysisModel creates a GasAnalysisModel for the given store and profile.
// Initializes composition at realistic pipeline gas values and computes initial BTU and SG.
func NewGasAnalysisModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *GasAnalysisModel {
	m := &GasAnalysisModel{
		store:           store,
		profile:         profile,
		methanePct:      92.0,
		ethanePct:       4.0,
		propanePct:      1.0,
		co2Pct:          0.5,
		nitrogenPct:     2.0,
		moistureContent: 4.0,
		cycleLength:     gasCycleMinTicks + rand.Intn(gasCycleRandRange), //nolint:gosec
	}
	m.btuValue = m.computeBTU()
	m.sgValue = m.computeSG()
	return m
}

// Name returns the model identifier for logging.
func (m *GasAnalysisModel) Name() string { return "gas-analysis" }

// Tick advances the gas analysis simulation by one second.
func (m *GasAnalysisModel) Tick() {
	m.updateMoisture()
	m.updateAlarmTimers()
	m.updateAnalysisCycle()
	m.writeRegisters()
	m.writeCoils()
}

// updateMoisture advances moisture drift (independent continuous sensor, updates every tick).
// Latches moisture_alarm when tariff limit (7 lb/MMSCF) is exceeded. [OT-REVIEW]
func (m *GasAnalysisModel) updateMoisture() {
	m.moistureContent = clamp(addNoise(drift(m.moistureContent, 4.0, 0.01), 20.0), 0, 20)
	if m.moistureContent > gasMoistureTariff && m.moistureAlarmTimer == 0 {
		m.moistureAlarmTimer = gasMoistureAlarmClear
	}
}

// updateAlarmTimers decrements gc and moisture alarm timers each tick.
func (m *GasAnalysisModel) updateAlarmTimers() {
	if m.gcAlarmTimer > 0 {
		m.gcAlarmTimer--
	}
	if m.moistureAlarmTimer > 0 {
		m.moistureAlarmTimer--
	}
}

// updateAnalysisCycle manages the NGC analysis state machine each tick.
// State: idle -> analyzing -> complete (held 5 ticks) -> idle.
// [OT-REVIEW] completeTimer holds status=2 so polling clients observe the completion event.
func (m *GasAnalysisModel) updateAnalysisCycle() {
	if m.completeTimer > 0 {
		m.completeTimer--
		if m.completeTimer == 0 {
			m.analysisStatus = gasCycleIdle
		}
		return
	}
	if m.analyzeTimer > 0 {
		m.advanceAnalysis()
		return
	}
	m.advanceIdle()
}

// advanceAnalysis counts down the active analysis phase and updates composition on completion.
func (m *GasAnalysisModel) advanceAnalysis() {
	m.analyzeTimer--
	if m.analyzeTimer == 0 {
		m.updateComposition()
		m.analysisStatus = gasCycleComplete
		m.completeTimer = gasCompleteLatch
		m.cycleTimer = 0
		m.cycleLength = gasCycleMinTicks + rand.Intn(gasCycleRandRange) //nolint:gosec
	}
}

// advanceIdle counts up the idle period and triggers the next analysis cycle.
// Rare GC hardware faults (0.01%/tick) set the error state and gc_alarm latch.
func (m *GasAnalysisModel) advanceIdle() {
	m.cycleTimer++
	if m.cycleTimer >= m.cycleLength {
		m.analysisStatus = gasCycleAnalyzing
		m.analyzeTimer = gasAnalyzeTicks
	}
	if rand.Float64() < 0.0001 { //nolint:gosec -- rare fault
		m.analysisStatus = gasCycleError
		m.gcAlarmTimer = gasGCAlarmClear
	}
}

// updateComposition applies drift to each component and recalculates BTU and SG.
// Called on analysis cycle completion. Values drift from their previous analyzed results.
func (m *GasAnalysisModel) updateComposition() {
	m.methanePct = clamp(drift(m.methanePct, 92.0, 0.1), 90, 94)
	m.ethanePct = clamp(drift(m.ethanePct, 4.0, 0.05), 3, 5)
	m.propanePct = clamp(drift(m.propanePct, 1.0, 0.02), 0.5, 1.5)
	m.co2Pct = clamp(drift(m.co2Pct, 0.5, 0.01), 0.2, 0.8)
	m.nitrogenPct = clamp(drift(m.nitrogenPct, 2.0, 0.02), 1, 3)
	m.btuValue = m.computeBTU()
	m.sgValue = m.computeSG()
}

// computeBTU calculates gross heating value from composition in BTU/SCF.
// [OT-REVIEW] GPSA Engineering Data Book heating values for pure components.
// C4+ fractions not included (TD-008).
func (m *GasAnalysisModel) computeBTU() float64 {
	return 1010.0*(m.methanePct/100.0) +
		1770.0*(m.ethanePct/100.0) +
		2516.0*(m.propanePct/100.0)
}

// computeSG calculates specific gravity relative to air from composition.
// [OT-REVIEW] MW ratios: MW_component / MW_air (MW_air = 28.964 g/mol).
// CO2 and N2 included; C4+ not included (TD-008).
func (m *GasAnalysisModel) computeSG() float64 {
	return 0.5539*(m.methanePct/100.0) +
		1.0382*(m.ethanePct/100.0) +
		1.5226*(m.propanePct/100.0) +
		1.5196*(m.co2Pct/100.0) +
		0.9672*(m.nitrogenPct/100.0)
}

// writeRegisters writes all gas analysis holding registers (one-based, start at addr 1).
// Composition registers hold the last analysis result (no noise -- discrete chromatograph output).
// Moisture is continuously noised from the independent sensor.
func (m *GasAnalysisModel) writeRegisters() {
	regs := []uint16{
		toRaw(m.methanePct, 0, 100),
		toRaw(m.ethanePct, 0, 20),
		toRaw(m.propanePct, 0, 10),
		toRaw(m.co2Pct, 0, 5),
		toRaw(m.nitrogenPct, 0, 10),
		toRaw(m.btuValue, 900, 1200),
		toRaw(m.sgValue, 0.50, 1.00),
		toRaw(m.moistureContent, 0, 20),
		uint16(m.analysisStatus),
	}
	_ = m.store.WriteHoldingInternal(gasRegMethane, regs)
}

// writeCoils writes analysis_in_progress, gc_alarm_active, and moisture_alarm_active.
// [OT-REVIEW] analysis_in_progress = true only during analyzing phase (status=1).
// [OT-REVIEW] gc_alarm_active = GC hardware fault only (not moisture quality).
// [OT-REVIEW] moisture_alarm_active = tariff exceedance only (not GC hardware fault).
// These are independent alarms per FR-31.
func (m *GasAnalysisModel) writeCoils() {
	_ = m.store.WriteCoilsInternal(gasCoilAnalysisInProgress, []bool{
		m.analysisStatus == gasCycleAnalyzing, // analysis_in_progress
		m.gcAlarmTimer > 0,                    // gc_alarm_active
		m.moistureAlarmTimer > 0,              // moisture_alarm_active
	})
}
