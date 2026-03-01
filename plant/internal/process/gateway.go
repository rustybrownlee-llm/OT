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
type GatewayModel struct {
	store       *mbstore.RegisterStore
	profile     *device.DeviceProfile
	txCount     uint32  // wider than uint16 to detect wrap-around
	rxCount     uint32
	errorCount  uint32
	uptimeTicks uint32  // ticks since simulation start
}

// NewGatewayModel creates a GatewayModel for the given store and profile.
// The store must be initialized with static enum values (done by NewRegisterStore
// from the device atom InitValues).
func NewGatewayModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *GatewayModel {
	// Read the initialized uptime hours from the store to preserve continuity.
	var baseUptime uint32
	if vals, err := store.ReadHolding(gatewayRegUptimeHours, 1); err == nil {
		baseUptime = uint32(vals[0])
	}
	return &GatewayModel{
		store:      store,
		profile:    profile,
		uptimeTicks: baseUptime * 3600, // convert base hours to ticks
	}
}

// Name returns the model identifier for logging.
func (m *GatewayModel) Name() string {
	return "mfg-gateway"
}

// Tick advances the gateway simulation by one second.
func (m *GatewayModel) Tick() {
	m.uptimeTicks++

	// TX and RX: 2 messages per tick (one per downstream serial device).
	// [OT-REVIEW] 2 tx/tick models realistic polling of SLC-500 + Modicon 984.
	m.txCount += 2
	m.rxCount += 2

	// Error: ~0.1% error rate (1 error per ~500 ticks at 2 tx/tick = 1 per ~250s).
	if rand.Float64() < 0.002 { //nolint:gosec -- educational simulation, not security-critical RNG
		m.errorCount++
	}

	// Active TCP connections: random 0-2 (simulated, no real connection tracking here).
	activeTCP := uint16(rand.Intn(3)) //nolint:gosec

	// Uptime in hours. Initialized to 8760 (one year) + simulation ticks/3600.
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
