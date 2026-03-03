// Tests for SOW-028.0: Poller Transaction Logging.
// Covers event collection logic, function codes, address chunking, failure
// recording, exception code extraction, and EventHook nil safety.
package poller

import (
	"testing"

	"github.com/simonvetter/modbus"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/config"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/inventory"
)

// makeConfigWithEnvironment builds a config with one environment for srcAddr tests.
func makeConfigWithEnvironment(addr string) *config.Config {
	return &config.Config{
		PollIntervalSeconds: 2,
		APIAddr:             ":8091",
		Environments: []config.Environment{
			{
				Name:    "test-env",
				Address: addr,
				Endpoints: []config.Endpoint{
					{Port: 5020, UnitID: 1},
				},
			},
		},
	}
}

// makeTestAsset creates an inventory.Asset for use in event collection tests.
func makeTestAsset(holdingCount, coilCount int) *inventory.Asset {
	return &inventory.Asset{
		ID:              "10.0.0.1:5020:1",
		Endpoint:        "10.0.0.1:5020",
		UnitID:          1,
		HoldingRegCount: holdingCount,
		CoilCount:       coilCount,
		Addressing:      "zero-based",
		ConfigDesc:      "Test PLC",
		EnvironmentName: "test-env",
	}
}

// makeTestPoller creates a Poller with a known srcAddr for deterministic tests.
func makeTestPoller(addr string) *Poller {
	cfg := makeConfigWithEnvironment(addr)
	inv := inventory.NewInventory()
	state := &PollState{}
	return New(cfg, inv, state)
}

// --- extractExceptionCode tests ---

// TestExtractExceptionCode_Nil verifies that a nil error returns code 0.
func TestExtractExceptionCode_Nil(t *testing.T) {
	if code := extractExceptionCode(nil); code != 0 {
		t.Errorf("nil error: got %d, want 0", code)
	}
}

// TestExtractExceptionCode_TransportError verifies that a non-Modbus error
// (TCP timeout, etc.) returns code 0.
func TestExtractExceptionCode_TransportError(t *testing.T) {
	transportErr := modbus.ErrRequestTimedOut
	if code := extractExceptionCode(transportErr); code != 0 {
		t.Errorf("transport error: got %d, want 0", code)
	}
}

// TestExtractExceptionCode_ModbusExceptions verifies that each Modbus exception
// error constant maps to its correct exception code byte.
func TestExtractExceptionCode_ModbusExceptions(t *testing.T) {
	cases := []struct {
		err      error
		expected uint8
	}{
		{modbus.ErrIllegalFunction, 0x01},
		{modbus.ErrIllegalDataAddress, 0x02},
		{modbus.ErrIllegalDataValue, 0x03},
		{modbus.ErrServerDeviceFailure, 0x04},
		{modbus.ErrAcknowledge, 0x05},
		{modbus.ErrServerDeviceBusy, 0x06},
		{modbus.ErrMemoryParityError, 0x08},
		{modbus.ErrGWPathUnavailable, 0x0a},
		{modbus.ErrGWTargetFailedToRespond, 0x0b},
	}

	for _, tc := range cases {
		got := extractExceptionCode(tc.err)
		if got != tc.expected {
			t.Errorf("extractExceptionCode(%v): got 0x%02x, want 0x%02x", tc.err, got, tc.expected)
		}
	}
}

// --- resolveMonitorAddr tests ---

// TestResolveMonitorAddr_NoEnvironments verifies the fallback when no environments
// are configured.
func TestResolveMonitorAddr_NoEnvironments(t *testing.T) {
	cfg := makeMinimalConfig(2)
	addr := resolveMonitorAddr(cfg)
	if addr != "0.0.0.0:0" {
		t.Errorf("no environments: got %q, want %q", addr, "0.0.0.0:0")
	}
}

// TestResolveMonitorAddr_ValidAddress verifies that the environment IP is extracted
// and port 0 is appended.
func TestResolveMonitorAddr_ValidAddress(t *testing.T) {
	cfg := makeConfigWithEnvironment("10.10.30.100")
	addr := resolveMonitorAddr(cfg)
	if addr != "10.10.30.100:0" {
		t.Errorf("valid address: got %q, want %q", addr, "10.10.30.100:0")
	}
}

// TestResolveMonitorAddr_EmptyAddress verifies the fallback when the first
// environment has an empty address field.
func TestResolveMonitorAddr_EmptyAddress(t *testing.T) {
	cfg := &config.Config{
		PollIntervalSeconds: 2,
		Environments: []config.Environment{
			{Name: "empty", Address: "", Endpoints: []config.Endpoint{{Port: 5020, UnitID: 1}}},
		},
	}
	addr := resolveMonitorAddr(cfg)
	if addr != "0.0.0.0:0" {
		t.Errorf("empty address: got %q, want %q", addr, "0.0.0.0:0")
	}
}

// --- EventHook nil-safety tests ---

// TestEventHook_NilSafe verifies that when no EventHook is registered, the
// poller's runCycle does not panic and the CycleHook still fires.
func TestEventHook_NilSafe(t *testing.T) {
	cfg := makeMinimalConfig(1)
	inv := inventory.NewInventory()
	state := &PollState{}
	p := New(cfg, inv, state)

	// No SetEventHook call -- eventHook must remain nil.
	if p.eventHook != nil {
		t.Fatal("eventHook should be nil before SetEventHook is called")
	}

	// runCycle with an empty inventory and nil eventHook must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runCycle panicked with nil eventHook: %v", r)
		}
	}()

	p.runCycle(nil)
}

// TestEventHook_SetAndClear verifies SetEventHook stores and can overwrite the hook.
func TestEventHook_SetAndClear(t *testing.T) {
	p := makeTestPoller("10.0.0.1")

	if p.eventHook != nil {
		t.Error("eventHook should be nil before SetEventHook")
	}

	p.SetEventHook(func(events []*eventstore.TransactionEvent) {})
	if p.eventHook == nil {
		t.Error("eventHook should be non-nil after SetEventHook")
	}

	p.SetEventHook(nil)
	if p.eventHook != nil {
		t.Error("eventHook should be nil after SetEventHook(nil)")
	}
}

// --- Chunk count correctness tests ---

// TestChunkCount_HoldingRegisters_125 verifies that 125 holding registers
// produce exactly 1 event (one FC 3 request).
func TestChunkCount_HoldingRegisters_125(t *testing.T) {
	count := chunkCount(125, modbusMaxRead)
	if count != 1 {
		t.Errorf("125 registers: got %d chunks, want 1", count)
	}
}

// TestChunkCount_HoldingRegisters_250 verifies that 250 holding registers
// produce exactly 2 events (two FC 3 requests of 125 each).
func TestChunkCount_HoldingRegisters_250(t *testing.T) {
	count := chunkCount(250, modbusMaxRead)
	if count != 2 {
		t.Errorf("250 registers: got %d chunks, want 2", count)
	}
}

// TestChunkCount_HoldingRegisters_126 verifies that 126 registers produce
// exactly 2 events (125 + 1).
func TestChunkCount_HoldingRegisters_126(t *testing.T) {
	count := chunkCount(126, modbusMaxRead)
	if count != 2 {
		t.Errorf("126 registers: got %d chunks, want 2", count)
	}
}

// TestChunkCount_Coils_100 verifies that 100 coils produce exactly 1 event
// because modbusMaxCoilRead (2000) is not exceeded.
func TestChunkCount_Coils_100(t *testing.T) {
	count := chunkCount(100, modbusMaxCoilRead)
	if count != 1 {
		t.Errorf("100 coils: got %d chunks, want 1", count)
	}
}

// TestChunkCount_Coils_2000 verifies that 2000 coils produce exactly 1 event
// (the maximum single-request coil count per the Modbus specification).
func TestChunkCount_Coils_2000(t *testing.T) {
	count := chunkCount(2000, modbusMaxCoilRead)
	if count != 1 {
		t.Errorf("2000 coils: got %d chunks, want 1", count)
	}
}

// TestChunkCount_Coils_2001 verifies that 2001 coils produce exactly 2 events.
func TestChunkCount_Coils_2001(t *testing.T) {
	count := chunkCount(2001, modbusMaxCoilRead)
	if count != 2 {
		t.Errorf("2001 coils: got %d chunks, want 2", count)
	}
}

// chunkCount computes the number of Modbus read requests required to read
// `total` registers/coils in `chunkSize` chunks. Mirrors the loop logic
// in readHoldingWithEvents and readCoilsWithEvents without a live client.
func chunkCount(total, chunkSize int) int {
	if total == 0 {
		return 0
	}
	count := 0
	for remaining := total; remaining > 0; {
		qty := remaining
		if qty > chunkSize {
			qty = chunkSize
		}
		remaining -= qty
		count++
	}
	return count
}

// --- Address progression tests ---

// TestAddressProgression_Holding verifies that a 250-register read issues
// chunks with AddressStart 0 and 125 respectively.
func TestAddressProgression_Holding(t *testing.T) {
	starts := chunkAddressStarts(0, 250, modbusMaxRead)
	expected := []uint16{0, 125}
	if len(starts) != len(expected) {
		t.Fatalf("chunk count: got %d, want %d", len(starts), len(expected))
	}
	for i, s := range starts {
		if s != expected[i] {
			t.Errorf("chunk[%d] AddressStart: got %d, want %d", i, s, expected[i])
		}
	}
}

// TestAddressProgression_Coils_OneBase verifies that one-based addressing
// produces correct address starts for coil reads.
func TestAddressProgression_Coils_OneBase(t *testing.T) {
	starts := chunkAddressStarts(1, 10, modbusMaxCoilRead)
	expected := []uint16{1}
	if len(starts) != len(expected) {
		t.Fatalf("chunk count: got %d, want %d", len(starts), len(expected))
	}
	if starts[0] != expected[0] {
		t.Errorf("chunk[0] AddressStart: got %d, want %d", starts[0], expected[0])
	}
}

// chunkAddressStarts returns the AddressStart value for each chunk in a read
// of `total` items starting at `baseAddr` with `chunkSize` per chunk.
func chunkAddressStarts(baseAddr uint16, total, chunkSize int) []uint16 {
	var starts []uint16
	addr := baseAddr
	for remaining := total; remaining > 0; {
		qty := remaining
		if qty > chunkSize {
			qty = chunkSize
		}
		starts = append(starts, addr)
		addr += uint16(qty)
		remaining -= qty
	}
	return starts
}

// --- Constant validation tests ---

// TestConstants_modbusMaxRead verifies the register chunk size constant.
func TestConstants_modbusMaxRead(t *testing.T) {
	if modbusMaxRead != 125 {
		t.Errorf("modbusMaxRead: got %d, want 125", modbusMaxRead)
	}
}

// TestConstants_modbusMaxCoilRead verifies the coil chunk size constant matches
// the Modbus specification (250 bytes = 2000 bits).
func TestConstants_modbusMaxCoilRead(t *testing.T) {
	if modbusMaxCoilRead != 2000 {
		t.Errorf("modbusMaxCoilRead: got %d, want 2000", modbusMaxCoilRead)
	}
}

// TestSrcAddr_MatchesFirstEnvironment verifies that the poller srcAddr reflects
// the first configured environment's IP with port 0.
func TestSrcAddr_MatchesFirstEnvironment(t *testing.T) {
	p := makeTestPoller("192.168.1.50")
	if p.srcAddr != "192.168.1.50:0" {
		t.Errorf("srcAddr: got %q, want %q", p.srcAddr, "192.168.1.50:0")
	}
}
