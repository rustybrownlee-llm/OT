package process

import (
	"math/rand"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// Water treatment process models: intake, treatment, distribution.
//
// Each model holds a reference to its RegisterStore and maintains internal state.
// Register addresses are zero-based per the CompactLogix L33ER device atom.
// All models simulate independently (cross-variant coupling is TD-013).
//
// PROTOTYPE-DEBT: [td-water-013] Cross-variant coupling not implemented.
// Intake flow does not feed treatment inlet pressure. Each variant simulates independently.
// TODO-FUTURE: Process coupling engine in a subsequent SOW.

// intakeRegs maps the water-intake variant holding register addresses (zero-based).
const (
	intakeRegFlowRate   = 0 // L/s, read-only
	intakeRegPumpSpeed  = 1 // %, writable setpoint
	intakeRegPHraw      = 2 // pH, read-only
	intakeRegTurbidity  = 3 // NTU, read-only
	intakeRegWaterTemp  = 4 // degC, read-only
)

// intakeCoils maps the water-intake variant coil addresses (zero-based).
const (
	intakeCoilPump01  = 0 // intake_pump_01_run, writable
	intakeCoilPump02  = 1 // intake_pump_02_run, writable
	intakeCoilScreen  = 2 // screen_wash_active, read-only (simulation-driven)
	intakeCoilLowWell = 3 // low_well_level_alarm, read-only (simulation-driven)
)

// IntakeModel simulates the raw water intake process for wt-plc-01.
type IntakeModel struct {
	store       *mbstore.RegisterStore
	profile     *device.DeviceProfile
	pump1Ramp   float64
	pump2Ramp   float64
	phCurrent   float64
	turbCurrent float64
	tempCurrent float64
	screenTimer int
}

// NewIntakeModel creates an IntakeModel for the given store and profile.
func NewIntakeModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *IntakeModel {
	return &IntakeModel{
		store:       store,
		profile:     profile,
		phCurrent:   7.0,
		turbCurrent: 35.0,
		tempCurrent: 18.0,
	}
}

// Name returns the model identifier for logging.
func (m *IntakeModel) Name() string { return "water-intake" }

// Tick advances the intake simulation by one second.
func (m *IntakeModel) Tick() {
	coils, err := m.store.ReadCoils(0, 4)
	if err != nil {
		return
	}
	speedVals, err := m.store.ReadHolding(intakeRegPumpSpeed, 1)
	if err != nil {
		return
	}
	m.updatePumpRamps(coils)
	flowRate := m.computeFlow(speedVals[0])
	m.updateDrift()
	m.updateScreenWash()
	m.writeRegisters(flowRate, speedVals[0])
	m.writeAlarmCoils(flowRate)
}

// updatePumpRamps advances ramp positions toward their targets (on=1.0, off=0.0).
func (m *IntakeModel) updatePumpRamps(coils []bool) {
	target1, target2 := 0.0, 0.0
	if coils[intakeCoilPump01] {
		target1 = 1.0
	}
	if coils[intakeCoilPump02] {
		target2 = 1.0
	}
	m.pump1Ramp = ramp(m.pump1Ramp, target1, 0.2)
	m.pump2Ramp = ramp(m.pump2Ramp, target2, 0.2)
}

// computeFlow returns flow in L/s for the current pump speed and ramp state.
// [OT-REVIEW] Each pump contributes 50 L/s at full ramp -- parallel centrifugal pumps add flow.
func (m *IntakeModel) computeFlow(speedRaw uint16) float64 {
	speedEng := toEng(speedRaw, 0, 100)
	flowRate := (speedEng / 100.0) * (m.pump1Ramp + m.pump2Ramp) * 50.0
	flowRate = addNoise(flowRate, 100.0)
	return clamp(flowRate, 0, 100)
}

// updateDrift advances pH, turbidity, and temperature drift values.
func (m *IntakeModel) updateDrift() {
	m.phCurrent = clamp(addNoise(drift(m.phCurrent, 7.2, 0.01), 14.0), 0, 14)
	m.turbCurrent = clamp(addNoise(drift(m.turbCurrent, 35.0, 0.1), 100.0), 0, 100)
	m.tempCurrent = clamp(addNoise(drift(m.tempCurrent, 18.0, 0.02), 40.0), 0, 40)
}

// updateScreenWash advances the screen wash cycle timer with a 5% activation chance.
func (m *IntakeModel) updateScreenWash() {
	if m.screenTimer > 0 {
		m.screenTimer--
	} else if rand.Float64() < 0.05 { //nolint:gosec
		m.screenTimer = 3
	}
}

// writeRegisters writes all holding registers to the store.
func (m *IntakeModel) writeRegisters(flowRate float64, speedRaw uint16) {
	regs := []uint16{
		toRaw(flowRate, 0, 100),
		speedRaw,
		toRaw(m.phCurrent, 0, 14),
		toRaw(m.turbCurrent, 0, 100),
		toRaw(m.tempCurrent, 0, 40),
	}
	_ = m.store.WriteHoldingInternal(0, regs)
}

// writeAlarmCoils writes the screen wash and low well level coils.
func (m *IntakeModel) writeAlarmCoils(flowRate float64) {
	screenActive := m.screenTimer > 0
	lowWell := (100.0 - flowRate) < 30.0
	_ = m.store.WriteCoilsInternal(intakeCoilScreen, []bool{screenActive})
	_ = m.store.WriteCoilsInternal(intakeCoilLowWell, []bool{lowWell})
}

// treatmentRegs maps the water-treatment variant holding register addresses (zero-based).
const (
	treatRegInletPressure    = 0 // kPa, read-only
	treatRegOutletPressure   = 1 // kPa, read-only
	treatRegDiffPressure     = 2 // kPa, read-only
	treatRegUVIntensity      = 3 // mW/cm2, read-only
	treatRegChemFeedRate     = 4 // mL/min, writable setpoint
	treatRegChlorineResidual = 5 // mg/L, read-only
	treatRegTurbidityPost    = 6 // NTU, read-only
)

// treatmentCoils maps the water-treatment variant coil addresses (zero-based).
const (
	treatCoilBackwash = 0 // filter_backwash_command, writable
	treatCoilFeedPump = 1 // chemical_feed_pump_run, writable
	treatCoilUVActive = 2 // uv_system_active, read-only (writable: false per device atom)
	treatCoilHighDP   = 3 // high_dp_alarm, read-only
)

// TreatmentModel simulates the water treatment process for wt-plc-02.
type TreatmentModel struct {
	store          *mbstore.RegisterStore
	profile        *device.DeviceProfile
	filterFouling  float64
	chlorineLevel  float64
	uvIntensity    float64
	backwashTimer  int
	highDPLatched  bool
	inletPressure  float64
}

// NewTreatmentModel creates a TreatmentModel for the given store and profile.
func NewTreatmentModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *TreatmentModel {
	return &TreatmentModel{
		store:         store,
		profile:       profile,
		filterFouling: 0.1,
		chlorineLevel: 1.0,
		uvIntensity:   85.0,
	}
}

// Name returns the model identifier for logging.
func (m *TreatmentModel) Name() string { return "water-treatment" }

// Tick advances the treatment simulation by one second.
func (m *TreatmentModel) Tick() {
	coils, err := m.store.ReadCoils(0, 4)
	if err != nil {
		return
	}
	feedVals, err := m.store.ReadHolding(treatRegChemFeedRate, 1)
	if err != nil {
		return
	}
	m.updateBackwash(coils[treatCoilBackwash])
	inlet, outlet, diff := m.computePressures()
	m.updateDPAlarm(diff)
	m.updateUV(coils[treatCoilUVActive])
	m.updateChlorine(coils[treatCoilFeedPump], toEng(feedVals[0], 0, 500))
	turbPost := m.computeTurbidityPost()
	m.writeRegisters(inlet, outlet, diff, turbPost, feedVals[0])
	_ = m.store.WriteCoilsInternal(treatCoilHighDP, []bool{m.highDPLatched})
}

// updateBackwash manages the filter backwash cycle.
// [OT-REVIEW] 20 seconds is compressed; real backwash takes 3-15 minutes.
func (m *TreatmentModel) updateBackwash(backwashCmd bool) {
	if backwashCmd && m.backwashTimer == 0 {
		m.backwashTimer = 20
	}
	if m.backwashTimer > 0 {
		m.filterFouling = ramp(m.filterFouling, 0, 0.05)
		m.backwashTimer--
		if m.backwashTimer == 0 {
			_ = m.store.WriteCoilsInternal(treatCoilBackwash, []bool{false})
		}
	} else {
		m.filterFouling = clamp(m.filterFouling+0.0001, 0, 1)
	}
}

// computePressures returns inlet, outlet, and differential pressure in kPa.
func (m *TreatmentModel) computePressures() (float64, float64, float64) {
	inlet := clamp(addNoise(50.0+15.0*0.3, 500.0), 0, 500)
	m.inletPressure = inlet
	outlet := clamp(addNoise(inlet-(m.filterFouling*50.0), 500.0), 0, 500)
	diff := clamp(inlet-outlet, 0, 50)
	return inlet, outlet, diff
}

// updateDPAlarm applies hysteresis to the high DP alarm: triggers at 25 kPa, clears at 15 kPa.
func (m *TreatmentModel) updateDPAlarm(diff float64) {
	if diff > 25.0 {
		m.highDPLatched = true
	} else if diff < 15.0 {
		m.highDPLatched = false
	}
}

// updateUV advances UV lamp intensity (slow aging when on, rapid decay when off).
// [OT-REVIEW] Models lamp at ~80% life; slow aging teaches UV as a monitored parameter.
func (m *TreatmentModel) updateUV(uvOn bool) {
	if uvOn {
		m.uvIntensity -= 0.0001
		m.uvIntensity = clamp(addNoise(m.uvIntensity, 2.0), 0, 100)
	} else {
		m.uvIntensity = ramp(m.uvIntensity, 0, 0.5)
	}
}

// updateChlorine applies decay and convergence toward the feed pump target.
func (m *TreatmentModel) updateChlorine(feedPumpOn bool, feedRateEng float64) {
	if feedPumpOn {
		target := feedRateEng * 0.005
		m.chlorineLevel = ramp(m.chlorineLevel, target, 0.005)
	}
	m.chlorineLevel = clamp(m.chlorineLevel-0.001, 0, 5)
}

// computeTurbidityPost computes turbidity after filtration with backwash disturbance.
func (m *TreatmentModel) computeTurbidityPost() float64 {
	filterEfficiency := 1.0 - m.filterFouling*0.8
	turbPost := 35.0 * (1.0 - filterEfficiency)
	if m.backwashTimer > 10 {
		turbPost += 1.5
	}
	return clamp(addNoise(turbPost, 5.0), 0, 5)
}

// writeRegisters writes all holding registers to the store.
func (m *TreatmentModel) writeRegisters(inlet, outlet, diff, turbPost float64, feedRaw uint16) {
	regs := []uint16{
		toRaw(inlet, 0, 500),
		toRaw(outlet, 0, 500),
		toRaw(diff, 0, 50),
		toRaw(m.uvIntensity, 0, 100),
		feedRaw,
		toRaw(m.chlorineLevel, 0, 5),
		toRaw(turbPost, 0, 5),
	}
	_ = m.store.WriteHoldingInternal(0, regs)
}

// distributionRegs maps the water-distribution variant holding register addresses (zero-based).
const (
	distRegWellLevel  = 0 // %, read-only
	distRegFlowRate   = 1 // L/s, read-only
	distRegPressure   = 2 // kPa, read-only
	distRegChlorine   = 3 // mg/L, read-only
	distRegWaterTemp  = 4 // degC, read-only
)

// distributionCoils maps the water-distribution variant coil addresses (zero-based).
const (
	distCoilPump01  = 0 // distribution_pump_01_run, writable
	distCoilPump02  = 1 // distribution_pump_02_run, writable
	distCoilBooster = 2 // booster_pump_run, writable
)

// DistributionModel simulates the water distribution process for wt-plc-03.
type DistributionModel struct {
	store     *mbstore.RegisterStore
	profile   *device.DeviceProfile
	pump1Ramp float64
	pump2Ramp float64
	wellLevel float64
	chlorine  float64
	waterTemp float64
}

// NewDistributionModel creates a DistributionModel for the given store and profile.
func NewDistributionModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *DistributionModel {
	return &DistributionModel{
		store:     store,
		profile:   profile,
		wellLevel: 75.0,
		chlorine:  0.8,
		waterTemp: 18.5,
	}
}

// Name returns the model identifier for logging.
func (m *DistributionModel) Name() string { return "water-distribution" }

// Tick advances the distribution simulation by one second.
func (m *DistributionModel) Tick() {
	coils, err := m.store.ReadCoils(0, 3)
	if err != nil {
		return
	}
	m.updatePumpRamps(coils)
	flowRate := m.computeFlow()
	pressure := m.computePressure(flowRate, coils[distCoilBooster])
	m.updateWellLevel(flowRate)
	m.updateChlorineAndTemp()
	m.writeRegisters(flowRate, pressure)
}

// updatePumpRamps advances ramp positions toward targets.
func (m *DistributionModel) updatePumpRamps(coils []bool) {
	target1, target2 := 0.0, 0.0
	if coils[distCoilPump01] {
		target1 = 1.0
	}
	if coils[distCoilPump02] {
		target2 = 1.0
	}
	m.pump1Ramp = ramp(m.pump1Ramp, target1, 0.2)
	m.pump2Ramp = ramp(m.pump2Ramp, target2, 0.2)
}

// computeFlow returns distribution flow in L/s.
func (m *DistributionModel) computeFlow() float64 {
	return clamp(addNoise((m.pump1Ramp+m.pump2Ramp)*75.0, 150.0), 0, 150)
}

// computePressure returns distribution pressure in kPa.
// [OT-REVIEW] Centrifugal pump curve: pressure DECREASES as flow increases.
func (m *DistributionModel) computePressure(flowRate float64, boosterOn bool) float64 {
	pressure := 500.0 - flowRate*1.0
	if boosterOn {
		pressure += 50.0
	}
	return clamp(addNoise(pressure, 700.0), 0, 700)
}

// updateWellLevel advances the clear well level based on inflow/outflow balance.
func (m *DistributionModel) updateWellLevel(flowRate float64) {
	inflow := 50.0 // estimated, TD-013: no cross-variant coupling
	outflow := flowRate * 0.001
	m.wellLevel = clamp(m.wellLevel+(inflow-outflow)*0.01, 0, 100)
}

// updateChlorineAndTemp advances chlorine decay and temperature drift.
func (m *DistributionModel) updateChlorineAndTemp() {
	m.chlorine = clamp(m.chlorine-0.0005, 0, 5)
	m.waterTemp = clamp(addNoise(drift(m.waterTemp, 19.0, 0.02), 40.0), 0, 40)
}

// writeRegisters writes all holding registers to the store.
func (m *DistributionModel) writeRegisters(flowRate, pressure float64) {
	chlorineNoisy := clamp(addNoise(m.chlorine, 5.0), 0, 5)
	regs := []uint16{
		toRaw(m.wellLevel, 0, 100),
		toRaw(flowRate, 0, 150),
		toRaw(pressure, 0, 700),
		toRaw(chlorineNoisy, 0, 5),
		toRaw(m.waterTemp, 0, 40),
	}
	_ = m.store.WriteHoldingInternal(0, regs)
}
