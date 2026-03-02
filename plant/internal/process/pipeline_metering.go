package process

import (
	"math"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// MeteringModel simulates the custody-transfer gas metering ROC800 for ps-rtu-01.
// Emerson ROC800 with pipeline-metering variant (one-based addressing).
//
// AGA-3 simplified flow calculation: flow = K * sqrt(DP * SP_abs / T_abs)
// where K is calibrated for 2500 MSCFH at nominal conditions (DP=50 inH2O, SP=600 PSIG, T=60 degF).
// The financial attack surface: manipulating DP, SP, or T directly alters calculated volumes
// used for custody-transfer billing.
//
// PROTOTYPE-DEBT: [td-metering-018] Only meter run 1 has a full AGA-3 model.
// Meter runs 2-4 coils exist for attack-surface demonstration only.
// TODO-FUTURE: Expand per-run registers when TD-007 is resolved.
//
// PROTOTYPE-DEBT: [td-pipeline-017] AGA-3 is simplified (no expansion factor,
// supercompressibility, isentropic exponent). TD-017.
//
// Implemented in SOW-009.0.

// metering holding register addresses (one-based, Emerson ROC800 pipeline-metering variant).
const (
	meterRegFlowRate    = 1 // MSCFH, read-only (AGA-3 calculated)
	meterRegVolumeToday = 2 // MCF, read-only (accumulator, no noise applied)
	meterRegPressure    = 3 // PSIG, read-only
	meterRegTemperature = 4 // degF, read-only
	meterRegDP          = 5 // inH2O, read-only
	meterRegTotalFlow   = 6 // MSCFH, read-only (station aggregate)
	meterRegTotalVolume = 7 // MCF, read-only (station aggregate accumulator)
)

// metering coil addresses (one-based, Emerson ROC800 pipeline-metering variant).
const (
	meterCoilRun1Enabled = 1 // meter_run_1_enabled, writable
	meterCoilRun2Enabled = 2 // meter_run_2_enabled, writable (attack surface only)
	meterCoilRun3Enabled = 3 // meter_run_3_enabled, writable (attack surface only)
	meterCoilRun4Enabled = 4 // meter_run_4_enabled, writable (attack surface only)
)

// meterK is the AGA-3 orifice coefficient calibrated for 2500 MSCFH at nominal conditions.
// At DP=50 inH2O, SP=600 PSIG, T=60 degF:
//
//	K = 2500 / sqrt(50 * (600+14.696) / (60+459.67))
//	K = 2500 / sqrt(59.13) ≈ 325.1
//
// [OT-REVIEW] K is a dimensioned constant (MSCFH / sqrt(inH2O * PSIA / Rankine)).
// Its value absorbs unit conversions and orifice geometry.
const meterK = 325.1

// meterContractDayTicks is the number of ticks in a 24-hour contract day.
// [OT-REVIEW] Rollover at midnight for simplicity. Real ROC800 allows configurable rollover.
// TD-019.
const meterContractDayTicks = 86400

// MeteringModel tracks measurement state for the pipeline-metering variant.
type MeteringModel struct {
	store           *mbstore.RegisterStore
	profile         *device.DeviceProfile
	dpCurrent       float64 // inH2O, differential pressure across orifice plate
	pressureCurrent float64 // PSIG, static line pressure
	tempCurrent     float64 // degF, flowing temperature
	volumeToday     float64 // MCF, accumulated volume since contract day rollover
	contractDayTick int     // ticks since last rollover
}

// NewMeteringModel creates a MeteringModel for the given store and profile.
func NewMeteringModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *MeteringModel {
	return &MeteringModel{
		store:           store,
		profile:         profile,
		dpCurrent:       50.0,
		pressureCurrent: 600.0,
		tempCurrent:     60.0,
	}
}

// Name returns the model identifier for logging.
func (m *MeteringModel) Name() string { return "pipeline-metering" }

// Tick advances the metering simulation by one second.
func (m *MeteringModel) Tick() {
	coils, err := m.store.ReadCoils(meterCoilRun1Enabled, 4)
	if err != nil {
		return
	}
	// coils[0] = coil addr 1 (meter_run_1_enabled)
	run1Enabled := coils[0]

	m.updateMeasurements()
	flowRate := m.computeAGA3Flow(run1Enabled)
	m.updateVolumeAccumulator(flowRate)
	m.writeRegisters(flowRate)
}

// updateMeasurements advances DP, pressure, and temperature drift with sensor noise.
func (m *MeteringModel) updateMeasurements() {
	m.dpCurrent = clamp(addNoise(drift(m.dpCurrent, 50.0, 0.2), 200.0), 0, 200)
	m.pressureCurrent = clamp(addNoise(drift(m.pressureCurrent, 600.0, 0.5), 1500.0), 0, 1500)
	// Temperature drift rate 0.005 degF/tick models diurnal ambient temperature changes.
	m.tempCurrent = clamp(addNoise(drift(m.tempCurrent, 60.0, 0.005), 170.0), -20, 150)
}

// computeAGA3Flow returns flow rate in MSCFH using the simplified AGA-3 formula.
// Returns 0 when run is disabled or inputs are non-physical.
// [OT-REVIEW] Simplified approximation. TD-017.
func (m *MeteringModel) computeAGA3Flow(enabled bool) float64 {
	if !enabled {
		return 0
	}
	spAbs := m.pressureCurrent + 14.696 // PSIG to PSIA
	tAbs := m.tempCurrent + 459.67      // degF to Rankine
	if tAbs <= 0 || spAbs <= 0 || m.dpCurrent < 0 {
		return 0
	}
	return clamp(meterK*math.Sqrt(m.dpCurrent*spAbs/tAbs), 0, 5000)
}

// updateVolumeAccumulator adds flow to the daily accumulator and handles rollover.
// [OT-REVIEW] volume += flow / 3600 is only correct for a 1-second tick rate. FR-15, TD-014.
func (m *MeteringModel) updateVolumeAccumulator(flowRate float64) {
	m.volumeToday += flowRate / 3600.0
	m.contractDayTick++
	if m.contractDayTick >= meterContractDayTicks {
		m.volumeToday = 0
		m.contractDayTick = 0
	}
}

// writeRegisters writes all metering holding registers (one-based, start at addr 1).
// station_total_flow and station_total_volume mirror meter run 1 (single active run, TD-007).
func (m *MeteringModel) writeRegisters(flowRate float64) {
	regs := []uint16{
		toRaw(flowRate, 0, 5000),
		toRaw(m.volumeToday, 0, 100000),
		toRaw(m.pressureCurrent, 0, 1500),
		toRaw(m.tempCurrent, -20, 150),
		toRaw(m.dpCurrent, 0, 200),
		toRaw(flowRate, 0, 20000),       // station_total_flow = run 1 (TD-007)
		toRaw(m.volumeToday, 0, 100000), // station_total_volume mirrors run 1 (TD-007)
	}
	_ = m.store.WriteHoldingInternal(meterRegFlowRate, regs)
}
