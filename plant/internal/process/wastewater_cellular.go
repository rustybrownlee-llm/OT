package process

import (
	"math/rand"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// CellularModemModel simulates the Cradlepoint IBR600 cellular gateway for ww-modem-01.
// Cradlepoint IBR600 with vendor-remote-access variant (zero-based addressing).
//
// EDUCATIONAL ABSTRACTION: Real Cradlepoint IBR600 devices expose management via HTTPS and
// SNMP -- they have no Modbus TCP interface. This model simulates gateway management status
// as Modbus holding registers to make the device discoverable by the monitoring module.
// Teaching value: the cellular gateway is visible on the OT network during IP discovery.
// Its presence as a "surprise" during network discovery exercises is the core teaching point.
//
// Register map: all read-only (management via HTTPS, not Modbus).
// WAN status, signal strength, data usage, uptime, client count, NAT mode, VPN status.
//
// Attack awareness (educational):
//   - Default credentials (admin/admin) never changed
//   - NAT enabled: vendor cloud can reach all devices on ww-flat
//   - VPN active: encrypted tunnel bypasses all on-premises monitoring
//   - "Temporary" installation: still present four years later
//
// Implemented in SOW-018.0.

// cellular holding register addresses (zero-based, Cradlepoint vendor-remote-access variant).
const (
	cellRegWANStatus     = 0 // enum: 0=down, 1=connecting, 2=connected
	cellRegSignalStr     = 1 // dBm-magnitude (0=no signal, 120=perfect; good LTE: <60)
	cellRegDataUsageGB   = 2 // GB this billing cycle (0-999)
	cellRegUptimeSecs    = 3 // s uptime counter (wraps at 65535 ~= 18.2 hours)
	cellRegActiveLANClients = 4 // count: active LAN clients
	cellRegNATMode       = 5 // enum: 0=disabled, 1=enabled (always 1 -- default config)
	cellRegVPNStatus     = 6 // enum: 0=none, 1=inactive, 2=active
)

// cellular simulation constants.
const (
	cellWANConnected     = 2    // WAN link state: connected and data session active
	cellWANConnecting    = 1    // WAN link state: connecting/authenticating
	cellWANDropProb      = 0.0001 // 0.01%/tick chance of brief WAN drop
	cellWANDropTicks     = 30   // ticks WAN remains in drop/reconnect state
	cellSignalTarget     = 50.0 // good LTE signal magnitude (~-50 dBm)
	cellSignalNoise      = 5.0  // ±5 units per tick signal fluctuation
	cellDataRatePerTick  = 0.0001 // GB per tick (~8.6 GB/day: SCADA polling + VPN overhead)
	cellDataMax          = 999.0  // scale_max for data_usage_gb register
	cellUptimeWrapAt     = 65535 // uptime wraps at 16-bit max
	cellActiveLANClients = 3    // fixed: CompactLogix + Moxa + Cradlepoint itself
	cellNATEnabled       = 1    // always 1 (factory default, never changed)
	cellVPNActive        = 2    // vendor VPN to cloud is active
)

// CellularModemModel tracks simulation state for the Cradlepoint IBR600 cellular gateway.
type CellularModemModel struct {
	store         *mbstore.RegisterStore
	profile       *device.DeviceProfile
	wanStatus     int     // current WAN link state
	wanDropTicks  int     // ticks remaining in WAN drop/reconnect event
	signalStrength float64 // dBm-magnitude (0-120, good LTE: ~50)
	dataUsageGB   float64 // GB accumulated this billing cycle
	uptimeSecs    int     // uptime counter in seconds (wraps at 65535)
}

// NewCellularModemModel creates a CellularModemModel for the given store and profile.
// Initializes at steady-state connected condition: WAN up, good signal, VPN active.
func NewCellularModemModel(store *mbstore.RegisterStore, profile *device.DeviceProfile) *CellularModemModel {
	return &CellularModemModel{
		store:          store,
		profile:        profile,
		wanStatus:      cellWANConnected,
		signalStrength: cellSignalTarget,
		dataUsageGB:    0.0,
		uptimeSecs:     0,
	}
}

// Name returns the model identifier for logging.
func (m *CellularModemModel) Name() string { return "ww-cellular-modem" }

// Tick advances the cellular modem simulation by one second.
func (m *CellularModemModel) Tick() {
	m.updateWANStatus()
	m.updateSignalStrength()
	m.updateDataUsage()
	m.updateUptime()
	m.writeRegisters()
}

// updateWANStatus manages the WAN connection state with rare drop events.
// [OT-REVIEW] 0.01%/tick drop probability models real-world cellular handoff events.
// WAN remains in drop/connecting state for 30 ticks (~30 seconds) before reconnecting.
func (m *CellularModemModel) updateWANStatus() {
	if m.wanDropTicks > 0 {
		m.wanDropTicks--
		m.wanStatus = cellWANConnecting
		if m.wanDropTicks == 0 {
			m.wanStatus = cellWANConnected
		}
		return
	}
	if rand.Float64() < cellWANDropProb { //nolint:gosec -- educational simulation
		m.wanDropTicks = cellWANDropTicks
		m.wanStatus = cellWANConnecting
	}
}

// updateSignalStrength advances cellular signal strength with noise.
// Good LTE: magnitude ~50 (representing ~-50 dBm). Fluctuates ±5 units per tick.
func (m *CellularModemModel) updateSignalStrength() {
	m.signalStrength = clamp(addNoise(drift(m.signalStrength, cellSignalTarget, 0.1), 120.0), 0, 120)
}

// updateDataUsage accumulates cellular data usage this billing cycle.
// Rate: ~0.0001 GB/tick = ~8.6 GB/day (periodic SCADA polling + VPN overhead).
// Clamps at cellDataMax (999 GB) -- does not auto-reset (billing cycle reset is manual).
func (m *CellularModemModel) updateDataUsage() {
	m.dataUsageGB = clamp(m.dataUsageGB+cellDataRatePerTick, 0, cellDataMax)
}

// updateUptime increments the uptime counter each tick. Wraps at 65535 (16-bit).
// Very low uptime values indicate recent reboot or power interruption.
func (m *CellularModemModel) updateUptime() {
	m.uptimeSecs++
	if m.uptimeSecs > cellUptimeWrapAt {
		m.uptimeSecs = 0
	}
}

// writeRegisters writes all cellular modem holding registers to the store (zero-based, addr 0-6).
// All registers are read-only (management via HTTPS/SNMP, not Modbus).
// dataUsageGB stored as integer GB (floor) to match scale_min=0, scale_max=999 integer convention.
// signalStrength stored as integer magnitude (floor) to avoid fractional register encoding.
func (m *CellularModemModel) writeRegisters() {
	regs := []uint16{
		uint16(m.wanStatus),
		uint16(m.signalStrength), // floor to integer magnitude
		uint16(m.dataUsageGB),    // floor to integer GB
		uint16(m.uptimeSecs),     // wraps naturally at 65535
		cellActiveLANClients,     // fixed: 3 clients on flat OT network
		cellNATEnabled,           // fixed: 1 (always enabled, factory default)
		cellVPNActive,            // fixed: 2 (vendor VPN active)
	}
	_ = m.store.WriteHoldingInternal(cellRegWANStatus, regs)
}
