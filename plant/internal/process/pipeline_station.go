package process

import (
	"math/rand"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// StationMonitorModel simulates the station-wide monitoring ROC800 for ps-rtu-02.
// Emerson ROC800 with station-monitoring variant (one-based addressing).
//
// Monitors pipeline inlet/outlet conditions and controls block valves.
// The ESD valve is hardwired de-energize-to-trip (DOT 49 CFR 192) -- not commandable via Modbus.
// ESD valve uses a spring-return actuator that slams closed in 1 tick (100%/tick).
// Block valves are motor-operated and ramp at 10%/tick (10 seconds full stroke).
//
// PROTOTYPE-DEBT: [td-pipeline-016] Inlet pressure simulates independently from
// CompressorModel suction pressure. Cross-device coupling deferred.
//
// Implemented in SOW-009.0.

// station monitor holding register addresses (one-based, Emerson ROC800 station-monitoring variant).
const (
	stmonRegInletPressure  = 1 // PSIG, read-only
	stmonRegOutletPressure = 2 // PSIG, read-only
	stmonRegInletTemp      = 3 // degF, read-only
	stmonRegOutletTemp     = 4 // degF, read-only
	stmonRegInletValvePos  = 5 // %, read-only
	stmonRegOutletValvePos = 6 // %, read-only
	stmonRegESDValvePos    = 7 // %, read-only
	stmonRegStatusWord     = 8 // bitmask, read-only
)

// station monitor coil addresses (one-based, Emerson ROC800 station-monitoring variant).
const (
	stmonCoilESDActivate     = 1 // esd_activate, NOT writable (hardwired safety, DOT 49 CFR 192)
	stmonCoilInletValveCmd   = 2 // inlet_block_valve_cmd, writable
	stmonCoilOutletValveCmd  = 3 // outlet_block_valve_cmd, writable
	stmonCoilESDActiveStatus = 4 // esd_active_status, read-only
)

// station monitor constants.
const (
	stmonESDLatchTicks     = 120  // ticks ESD remains latched (2 minutes, operator reset per DOT 49 CFR 192)
	stmonValveRampRate     = 10.0 // %/tick for motor-operated block valves (10 ticks = full stroke)
	stmonESDValveCloseRate = 100.0 // %/tick for spring-return ESD valve (1 tick = full closure)
)

// StationMonitorModel tracks valve positions and pressure/temperature for the station.
type StationMonitorModel struct {
	store          *mbstore.RegisterStore
	profile        *device.DeviceProfile
	inletPressure  float64 // PSIG
	inletTemp      float64 // degF
	inletValvePos  float64 // %
	outletValvePos float64 // %
	esdValvePos    float64 // %
	esdTimer       int     // ticks remaining in ESD latch
	commFailTimer  int     // ticks remaining in comm-failure alarm latch
	powerFailTimer int     // ticks remaining in power-failure alarm latch
	fgAlarmTimer   int     // ticks remaining in F&G alarm latch
}

// NewStationMonitorModel creates a StationMonitorModel for the given store and profile.
// Valves initialize open (100%) -- station starts in normal flowing configuration.
func NewStationMonitorModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *StationMonitorModel {
	return &StationMonitorModel{
		store:          store,
		profile:        profile,
		inletPressure:  600.0,
		inletTemp:      60.0,
		inletValvePos:  100.0,
		outletValvePos: 100.0,
		esdValvePos:    100.0,
	}
}

// Name returns the model identifier for logging.
func (m *StationMonitorModel) Name() string { return "station-monitoring" }

// Tick advances the station monitor simulation by one second.
func (m *StationMonitorModel) Tick() {
	// ROC800 one-based: read coils at addr 1 (4 coils: esd_activate, inlet_cmd, outlet_cmd, esd_status).
	coils, err := m.store.ReadCoils(stmonCoilESDActivate, 4)
	if err != nil {
		return
	}
	// coils[0]=esd_activate (hardware-driven, read-only), [1]=inlet_cmd, [2]=outlet_cmd
	inletCmd := coils[1]
	outletCmd := coils[2]

	m.updateESD()
	esdActive := m.esdTimer > 0
	m.updatePressuresAndTemps()
	m.updateValvePositions(inletCmd, outletCmd, esdActive)
	m.updateAlarmTimers()
	m.writeRegisters(esdActive)
	m.writeCoils(esdActive)
}

// updateESD handles the ESD random trigger and 120-tick latch.
// [OT-REVIEW] esd_activate (coil addr 1) is hardware-driven. Model NEVER writes it.
// [OT-REVIEW] ESD fires ~once every 167 minutes at 0.01%/tick -- trainers see at least one event.
func (m *StationMonitorModel) updateESD() {
	if m.esdTimer > 0 {
		m.esdTimer--
		return
	}
	if rand.Float64() < 0.0001 { //nolint:gosec -- educational simulation
		m.esdTimer = stmonESDLatchTicks
	}
}

// updatePressuresAndTemps advances inlet pressure and temperature drift.
// Outlet values are derived at write time to avoid storing redundant state.
func (m *StationMonitorModel) updatePressuresAndTemps() {
	m.inletPressure = clamp(addNoise(drift(m.inletPressure, 600.0, 0.5), 1500.0), 0, 1500)
	m.inletTemp = clamp(addNoise(drift(m.inletTemp, 60.0, 0.005), 170.0), -20, 150)
}

// updateValvePositions ramps block valves and snaps ESD valve during ESD event.
// Block valves (motor-operated): 10%/tick. ESD valve (spring-return): 100%/tick (1 tick).
// [OT-REVIEW] Spring-return ESD valves slam closed using stored mechanical energy.
func (m *StationMonitorModel) updateValvePositions(inletCmd, outletCmd, esdActive bool) {
	if esdActive {
		m.inletValvePos = clamp(m.inletValvePos-stmonValveRampRate, 0, 100)
		m.outletValvePos = clamp(m.outletValvePos-stmonValveRampRate, 0, 100)
		m.esdValvePos = clamp(m.esdValvePos-stmonESDValveCloseRate, 0, 100)
		return
	}
	if inletCmd {
		m.inletValvePos = clamp(m.inletValvePos+stmonValveRampRate, 0, 100)
	} else {
		m.inletValvePos = clamp(m.inletValvePos-stmonValveRampRate, 0, 100)
	}
	if outletCmd {
		m.outletValvePos = clamp(m.outletValvePos+stmonValveRampRate, 0, 100)
	} else {
		m.outletValvePos = clamp(m.outletValvePos-stmonValveRampRate, 0, 100)
	}
	m.esdValvePos = 100.0 // spring-return valve is held open during normal operation
}

// updateAlarmTimers advances probabilistic alarm latch timers per FR-23.
// comm-failure: 0.02%/tick, 15-tick latch. power-failure: 0.005%/tick, 30-tick latch.
// F&G alarm: 0.01%/tick, 20-tick latch.
func (m *StationMonitorModel) updateAlarmTimers() {
	decrementOrTrigger(&m.commFailTimer, 0.0002, 15)
	decrementOrTrigger(&m.powerFailTimer, 0.00005, 30)
	decrementOrTrigger(&m.fgAlarmTimer, 0.0001, 20)
}

// decrementOrTrigger decrements an active latch timer, or probabilistically triggers it.
func decrementOrTrigger(timer *int, prob float64, latchTicks int) {
	if *timer > 0 {
		*timer--
		return
	}
	if rand.Float64() < prob { //nolint:gosec
		*timer = latchTicks
	}
}

// buildStationStatusWord constructs the status bitmask per FR-23.
// bit0=running, bit1=ESD-active, bit2=high-pressure, bit3=low-pressure,
// bit4=high-temp, bit5=comm-failure, bit6=power-failure, bit7=F&G-alarm.
func (m *StationMonitorModel) buildStationStatusWord(esdActive bool) uint16 {
	outletPressure := m.inletPressure * 1.4
	outletTemp := m.inletTemp + 20.0
	running := !esdActive && (m.inletValvePos > 0 || m.outletValvePos > 0)

	var sw uint16
	if running {
		sw |= 1 << 0
	}
	if esdActive {
		sw |= 1 << 1
	}
	if outletPressure > 1200.0 {
		sw |= 1 << 2
	}
	if m.inletPressure < 200.0 {
		sw |= 1 << 3
	}
	if outletTemp > 120.0 {
		sw |= 1 << 4
	}
	if m.commFailTimer > 0 {
		sw |= 1 << 5
	}
	if m.powerFailTimer > 0 {
		sw |= 1 << 6
	}
	if m.fgAlarmTimer > 0 {
		sw |= 1 << 7
	}
	return sw
}

// writeRegisters writes all station monitor holding registers (one-based, start at addr 1).
// Outlet pressure = inlet * 1.4 (fixed compression ratio estimate, TD-016).
// Outlet temperature = inlet + 20 degF (post-aftercooler residual, FR-20).
func (m *StationMonitorModel) writeRegisters(esdActive bool) {
	outletPressure := clamp(addNoise(m.inletPressure*1.4, 1500.0), 0, 1500)
	outletTemp := clamp(addNoise(m.inletTemp+20.0, 170.0), -20, 150)
	regs := []uint16{
		toRaw(m.inletPressure, 0, 1500),
		toRaw(outletPressure, 0, 1500),
		toRaw(m.inletTemp, -20, 150),
		toRaw(outletTemp, -20, 150),
		toRaw(m.inletValvePos, 0, 100),
		toRaw(m.outletValvePos, 0, 100),
		toRaw(m.esdValvePos, 0, 100),
		m.buildStationStatusWord(esdActive),
	}
	_ = m.store.WriteHoldingInternal(stmonRegInletPressure, regs)
}

// writeCoils writes esd_active_status coil (read-only, model-driven, addr 4).
// [OT-REVIEW] The model never writes esd_activate (addr 1) -- that coil is hardware-driven.
func (m *StationMonitorModel) writeCoils(esdActive bool) {
	_ = m.store.WriteCoilsInternal(stmonCoilESDActiveStatus, []bool{esdActive})
}
