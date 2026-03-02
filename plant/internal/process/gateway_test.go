package process

import (
	"testing"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbstore "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// makeGatewayStore creates a RegisterStore for the serial-gateway variant (NPort 5150, zero-based).
// Enum registers are initialized to their operational values.
func makeGatewayStore() *mbstore.RegisterStore {
	return mbstore.NewRegisterStore(&device.DeviceProfile{
		PlacementID: "mfg-gateway-01",
		DeviceID:    "moxa-nport-5150",
		Addressing:  "zero-based",
		MaxHolding:  9,
		MaxCoils:    0,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 0, Name: "serial_port_status", Unit: "enum", ScaleMin: 0, ScaleMax: 2, Writable: false, InitValue: 1},
			{Address: 1, Name: "serial_baud_rate", Unit: "enum", ScaleMin: 0, ScaleMax: 7, Writable: false, InitValue: 3},
			{Address: 2, Name: "serial_data_format", Unit: "enum", ScaleMin: 0, ScaleMax: 3, Writable: false, InitValue: 0},
			{Address: 3, Name: "serial_mode", Unit: "enum", ScaleMin: 0, ScaleMax: 3, Writable: false, InitValue: 2},
			{Address: 4, Name: "active_tcp_connections", Unit: "count", ScaleMin: 0, ScaleMax: 4, Writable: false, InitValue: 0},
			{Address: 5, Name: "serial_tx_count", Unit: "msgs", ScaleMin: 0, ScaleMax: 65535, Writable: false, InitValue: 0},
			{Address: 6, Name: "serial_rx_count", Unit: "msgs", ScaleMin: 0, ScaleMax: 65535, Writable: false, InitValue: 0},
			{Address: 7, Name: "serial_error_count", Unit: "count", ScaleMin: 0, ScaleMax: 65535, Writable: false, InitValue: 0},
			{Address: 8, Name: "uptime_hours", Unit: "hours", ScaleMin: 0, ScaleMax: 65535, Writable: false, InitValue: 8760},
		},
	})
}

// readGatewayReg reads a single zero-based gateway holding register.
func readGatewayReg(t *testing.T, store *mbstore.RegisterStore, addr uint16) uint16 {
	t.Helper()
	vals, err := store.ReadHolding(addr, 1)
	if err != nil {
		t.Fatalf("ReadHolding addr %d: %v", addr, err)
	}
	return vals[0]
}

// TestGatewayModel_TXRXCountersIncrementPerTick verifies that TX and RX counters
// increase by 2 per tick (one poll per serial device).
func TestGatewayModel_TXRXCountersIncrementPerTick(t *testing.T) {
	store := makeGatewayStore()
	model := NewGatewayModel(store, nil)

	// Initial TX/RX should be 0.
	txBefore := readGatewayReg(t, store, gatewayRegSerialTX)
	rxBefore := readGatewayReg(t, store, gatewayRegSerialRX)

	model.Tick()

	txAfter := readGatewayReg(t, store, gatewayRegSerialTX)
	rxAfter := readGatewayReg(t, store, gatewayRegSerialRX)

	txDelta := int(txAfter) - int(txBefore)
	rxDelta := int(rxAfter) - int(rxBefore)

	if txDelta != 2 {
		t.Errorf("TX counter delta: got %d, want 2", txDelta)
	}
	if rxDelta != 2 {
		t.Errorf("RX counter delta: got %d, want 2", rxDelta)
	}
}

// TestGatewayModel_UptimeHours_IncrementsCorrectly verifies that uptime_hours advances
// at a rate of 1 hour per 3600 ticks from the initial base of 8760.
func TestGatewayModel_UptimeHours_IncrementsCorrectly(t *testing.T) {
	store := makeGatewayStore()
	model := NewGatewayModel(store, nil)

	// Read initial uptime (8760 hours from device atom InitValue).
	initialUptime := readGatewayReg(t, store, gatewayRegUptimeHours)
	if initialUptime != 8760 {
		t.Fatalf("expected initial uptime 8760 from store, got %d", initialUptime)
	}

	// After 3600 ticks (1 simulated hour), uptime should advance by 1.
	for i := 0; i < 3600; i++ {
		model.Tick()
	}

	finalUptime := readGatewayReg(t, store, gatewayRegUptimeHours)
	if finalUptime < initialUptime+1 {
		t.Errorf("uptime did not increment after 3600 ticks: initial=%d final=%d", initialUptime, finalUptime)
	}
}

// TestGatewayModel_CounterWrapsOnOverflow verifies that counters wrap to 0
// on 16-bit overflow rather than saturating at 65535.
// [OT-REVIEW] Wrap to 0 mirrors real PLC counter behavior (FR-6).
func TestGatewayModel_CounterWrapsOnOverflow(t *testing.T) {
	store := makeGatewayStore()
	model := NewGatewayModel(store, nil)

	// Manually set internal counter to just below overflow threshold.
	// With 2 increments per tick, wrapping occurs at txCount = 65536+.
	model.txCount = 65534 // one tick away from wrapping (65534 + 2 = 65536 -> uint16 = 0)

	model.Tick()

	txRaw := readGatewayReg(t, store, gatewayRegSerialTX)
	// 65534 + 2 = 65536 which is 0 in uint16.
	if txRaw != 0 {
		t.Errorf("TX counter should wrap to 0 at overflow: got %d", txRaw)
	}
}

// TestGatewayModel_EnumRegisters_RemainStatic verifies that enum registers
// are not modified by the simulation tick (FR-7).
func TestGatewayModel_EnumRegisters_RemainStatic(t *testing.T) {
	store := makeGatewayStore()
	model := NewGatewayModel(store, nil)

	// Record initial enum values.
	portStatus := readGatewayReg(t, store, gatewayRegPortStatus)
	baudRate := readGatewayReg(t, store, gatewayRegBaudRate)
	dataFormat := readGatewayReg(t, store, gatewayRegDataFormat)
	serialMode := readGatewayReg(t, store, gatewayRegSerialMode)

	for i := 0; i < 100; i++ {
		model.Tick()
	}

	if readGatewayReg(t, store, gatewayRegPortStatus) != portStatus {
		t.Errorf("serial_port_status changed: initial=%d", portStatus)
	}
	if readGatewayReg(t, store, gatewayRegBaudRate) != baudRate {
		t.Errorf("serial_baud_rate changed: initial=%d", baudRate)
	}
	if readGatewayReg(t, store, gatewayRegDataFormat) != dataFormat {
		t.Errorf("serial_data_format changed: initial=%d", dataFormat)
	}
	if readGatewayReg(t, store, gatewayRegSerialMode) != serialMode {
		t.Errorf("serial_mode changed: initial=%d", serialMode)
	}
}

// TestPipelineGatewayModel_TXRXCountersIncrementByOnePerTick verifies that the pipeline
// gateway increments TX and RX counters by 1 per tick (only one downstream serial device:
// TotalFlow G5). This differs from the water/mfg gateway which increments by 2/tick.
// SOW-009.0 FR-32.
func TestPipelineGatewayModel_TXRXCountersIncrementByOnePerTick(t *testing.T) {
	store := makeGatewayStore()
	model := NewPipelineGatewayModel(store, nil)

	txBefore := readGatewayReg(t, store, gatewayRegSerialTX)
	rxBefore := readGatewayReg(t, store, gatewayRegSerialRX)

	model.Tick()

	txAfter := readGatewayReg(t, store, gatewayRegSerialTX)
	rxAfter := readGatewayReg(t, store, gatewayRegSerialRX)

	txDelta := int(txAfter) - int(txBefore)
	rxDelta := int(rxAfter) - int(rxBefore)

	if txDelta != 1 {
		t.Errorf("pipeline gateway TX counter delta: got %d, want 1 (one serial device downstream)", txDelta)
	}
	if rxDelta != 1 {
		t.Errorf("pipeline gateway RX counter delta: got %d, want 1 (one serial device downstream)", rxDelta)
	}
}

// TestPipelineGatewayModel_Name verifies the pipeline gateway reports the correct model name.
func TestPipelineGatewayModel_Name(t *testing.T) {
	store := makeGatewayStore()
	model := NewPipelineGatewayModel(store, nil)
	if model.Name() != "pipeline-gateway" {
		t.Errorf("pipeline gateway name: got %q, want %q", model.Name(), "pipeline-gateway")
	}
}

// TestGatewayModel_Name verifies the water/mfg gateway retains its original model name.
func TestGatewayModel_Name(t *testing.T) {
	store := makeGatewayStore()
	model := NewGatewayModel(store, nil)
	if model.Name() != "mfg-gateway" {
		t.Errorf("mfg gateway name: got %q, want %q", model.Name(), "mfg-gateway")
	}
}
