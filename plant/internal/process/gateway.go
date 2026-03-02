package process

import (
	"math/rand"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// GatewayModel simulates the Moxa NPort 5150 serial-to-Ethernet gateway.
// It updates counter registers, uptime, and simulated active TCP connection count.
//
// The gateway model uses zero-based addressing (NPort 5150 device atom).
// All enum registers (serial_port_status, baud_rate, data_format, serial_mode) remain
// static at their initialized values per FR-7.
//
// [OT-REVIEW] Serial bus at 9600 baud is a contended medium: 2 tx/tick and 2 rx/tick
// models the realistic polling cadence for two downstream serial devices (SLC-500 unit 1,
// Modicon 984 unit 2).

// gateway register addresses (zero-based, matching Moxa NPort 5150 serial-gateway variant).
const (
	gatewayRegPortStatus     = 0 // enum: 0=offline, 1=online, 2=error -- static
	gatewayRegBaudRate       = 1 // enum: 0=1200...3=9600...7=115200 -- static
	gatewayRegDataFormat     = 2 // enum: 0=8N1, 1=8E1, 2=8O1, 3=7E1 -- static
	gatewayRegSerialMode     = 3 // enum: 0=RS-232, 1=RS-422, 2=RS-485-2wire, 3=RS-485-4wire -- static
	gatewayRegActiveTCP      = 4 // count: current TCP connections
	gatewayRegSerialTX       = 5 // msgs counter, wraps on 16-bit overflow
	gatewayRegSerialRX       = 6 // msgs counter, wraps on 16-bit overflow
	gatewayRegSerialError    = 7 // count counter, wraps on 16-bit overflow
	gatewayRegUptimeHours    = 8 // hours accumulator, wraps on 16-bit overflow
)

// GatewayModel tracks counter state for the NPort 5150 gateway simulation.
// Enum registers are left at their initialized values (FR-7).
// The txPerTick, rxPerTick, and errorProb fields allow parameterization for different
// deployment contexts (water/mfg vs pipeline-serial environments).
type GatewayModel struct {
	store       *mbstore.RegisterStore
	profile     *device.DeviceProfile
	name        string
	txPerTick   uint32  // messages forwarded per tick (depends on downstream device count)
	rxPerTick   uint32  // messages received per tick
	errorProb   float64 // probability of a serial error per tick
	txCount     uint32  // wider than uint16 to detect wrap-around
	rxCount     uint32
	errorCount  uint32
	uptimeTicks uint32  // ticks since simulation start
}

// newGatewayModelParams creates a GatewayModel with explicit counter rates and error probability.
// Callers use the exported constructors NewGatewayModel and NewPipelineGatewayModel.
func newGatewayModelParams(store *mbstore.RegisterStore, profile *device.DeviceProfile, name string, txPerTick, rxPerTick uint32, errorProb float64) *GatewayModel {
	var baseUptime uint32
	if vals, err := store.ReadHolding(gatewayRegUptimeHours, 1); err == nil {
		baseUptime = uint32(vals[0])
	}
	return &GatewayModel{
		store:       store,
		profile:     profile,
		name:        name,
		txPerTick:   txPerTick,
		rxPerTick:   rxPerTick,
		errorProb:   errorProb,
		uptimeTicks: baseUptime * 3600, // convert base hours to ticks
	}
}

// NewGatewayModel creates a GatewayModel for the water/mfg serial-gateway variant.
// TX/RX: 2 per tick (SLC-500 + Modicon 984). Error rate: 0.2%/tick.
// The store must be initialized with static enum values (done by NewRegisterStore
// from the device atom InitValues).
func NewGatewayModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *GatewayModel {
	// [OT-REVIEW] 2 tx/tick models realistic polling of SLC-500 + Modicon 984.
	return newGatewayModelParams(store, profile, "mfg-gateway", 2, 2, 0.002)
}

// NewPipelineGatewayModel creates a GatewayModel for the pipeline-serial variant.
// TX/RX: 1 per tick (TotalFlow G5 only). Error rate: 0.3%/tick.
// [OT-REVIEW] Higher error rate models high-EMI compressor station environment:
// large electric motors, VFDs, and gas turbine ignition systems produce significant
// electromagnetic interference on serial bus cabling.
// PROTOTYPE-DEBT: [td-gateway-020] Counter rates are hardcoded.
// TODO-FUTURE: Derive from environment placement topology (count serial devices behind each gateway).
func NewPipelineGatewayModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *GatewayModel {
	return newGatewayModelParams(store, profile, "pipeline-gateway", 1, 1, 0.003)
}

// Name returns the model identifier for logging.
func (m *GatewayModel) Name() string {
	return m.name
}

// Tick advances the gateway simulation by one second.
func (m *GatewayModel) Tick() {
	m.uptimeTicks++
	m.txCount += m.txPerTick
	m.rxCount += m.rxPerTick

	if rand.Float64() < m.errorProb { //nolint:gosec -- educational simulation, not security-critical RNG
		m.errorCount++
	}

	// Active TCP connections: random 0-2 (simulated, no real connection tracking here).
	activeTCP := uint16(rand.Intn(3)) //nolint:gosec

	// Uptime in hours. Initialized from device atom InitValue + simulation ticks/3600.
	// [OT-REVIEW] uptime_hours counter wraps to 0 on 16-bit overflow (FR-6).
	uptimeHours := uint16(m.uptimeTicks / 3600)

	// Write counter registers. Wrap to uint16 naturally (FR-6).
	regs := []uint16{
		// Addresses 0-3 (enum registers) are skipped -- they remain at initialized values.
		// We write addresses 4-8 starting at addr 4.
		activeTCP,
		uint16(m.txCount),    // wraps on 16-bit overflow
		uint16(m.rxCount),    // wraps on 16-bit overflow
		uint16(m.errorCount), // wraps on 16-bit overflow
		uptimeHours,          // wraps on 16-bit overflow
	}
	_ = m.store.WriteHoldingInternal(gatewayRegActiveTCP, regs)
}
