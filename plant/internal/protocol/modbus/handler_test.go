package modbus_test

import (
	"testing"
	"time"

	mblib "github.com/simonvetter/modbus"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	"github.com/rustybrownlee/ot-simulator/plant/internal/logging"
	mbpkg "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// makeTestProfile returns a simple DeviceProfile for use in handler tests.
func makeTestProfile(addressing string) *device.DeviceProfile {
	return &device.DeviceProfile{
		Addressing:        addressing,
		MaxHolding:        10,
		MaxCoils:          10,
		MaxInputRegisters: 10,
		MaxDiscreteInputs: 10,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 1, Name: "writable_reg", Unit: "L/s", Writable: true, InitValue: 16383},
			{Address: 2, Name: "readonly_reg", Unit: "L/s", Writable: false, InitValue: 16383},
		},
		Coils: []device.CoilEntry{
			{Address: 1, Name: "run_cmd", Writable: true, InitValue: false},
			{Address: 2, Name: "alarm", Writable: false, InitValue: false},
		},
	}
}

// makeNonGatewayHandler builds a non-gateway handler with a single store at sentinel key 0.
func makeNonGatewayHandler(addressing string) *mbpkg.Handler {
	profile := makeTestProfile(addressing)
	store := mbpkg.NewRegisterStore(profile)

	stores := map[uint8]*mbpkg.RegisterStore{0: store}
	delays := map[uint8]time.Duration{0: 0}
	jitters := map[uint8]time.Duration{0: 0}

	return mbpkg.NewHandler(stores, delays, jitters, false, logging.NewTestLogger())
}

// makeGatewayHandler builds a gateway handler with two unit IDs: 1 and 2.
func makeGatewayHandler() (*mbpkg.Handler, *mbpkg.RegisterStore, *mbpkg.RegisterStore) {
	profile1 := makeTestProfile("one-based")
	store1 := mbpkg.NewRegisterStore(profile1)

	profile2 := makeTestProfile("one-based")
	store2 := mbpkg.NewRegisterStore(profile2)

	stores := map[uint8]*mbpkg.RegisterStore{
		1: store1,
		2: store2,
	}
	delays := map[uint8]time.Duration{
		1: 0,
		2: 0,
	}
	jitters := map[uint8]time.Duration{
		1: 0,
		2: 0,
	}

	h := mbpkg.NewHandler(stores, delays, jitters, true, logging.NewTestLogger())
	return h, store1, store2
}

// --- Non-gateway handler tests ---

func TestHandler_NonGateway_AnyUnitID_Responds(t *testing.T) {
	h := makeNonGatewayHandler("one-based")

	// Non-gateway should respond to any unit ID.
	for _, uid := range []uint8{0, 1, 5, 99, 255} {
		req := &mblib.HoldingRegistersRequest{
			UnitId:   uid,
			Addr:     1,
			Quantity: 1,
		}
		vals, err := h.HandleHoldingRegisters(req)
		if err != nil {
			t.Errorf("unit ID %d: expected success, got error: %v", uid, err)
			continue
		}
		if len(vals) != 1 {
			t.Errorf("unit ID %d: expected 1 value, got %d", uid, len(vals))
		}
	}
}

func TestHandler_NonGateway_Write_Writable(t *testing.T) {
	h := makeNonGatewayHandler("one-based")

	req := &mblib.HoldingRegistersRequest{
		UnitId:  1,
		Addr:    1, // writable_reg
		IsWrite: true,
		Args:    []uint16{9999},
	}
	_, err := h.HandleHoldingRegisters(req)
	if err != nil {
		t.Fatalf("expected writable write to succeed, got: %v", err)
	}

	// Confirm value persisted.
	readReq := &mblib.HoldingRegistersRequest{
		UnitId:   1,
		Addr:     1,
		Quantity: 1,
	}
	vals, err := h.HandleHoldingRegisters(readReq)
	if err != nil {
		t.Fatalf("ReadHolding after write: %v", err)
	}
	if vals[0] != 9999 {
		t.Errorf("expected 9999, got %d", vals[0])
	}
}

func TestHandler_NonGateway_Write_ReadOnly_ReturnsErrIllegalFunction(t *testing.T) {
	h := makeNonGatewayHandler("one-based")

	req := &mblib.HoldingRegistersRequest{
		UnitId:  1,
		Addr:    2, // readonly_reg
		IsWrite: true,
		Args:    []uint16{100},
	}
	_, err := h.HandleHoldingRegisters(req)
	if err == nil {
		t.Fatal("expected error for read-only write, got nil")
	}
	if err != mblib.ErrIllegalFunction {
		t.Errorf("expected ErrIllegalFunction, got: %v", err)
	}
}

func TestHandler_NonGateway_Coil_Write_Writable(t *testing.T) {
	h := makeNonGatewayHandler("one-based")

	req := &mblib.CoilsRequest{
		UnitId:  1,
		Addr:    1, // run_cmd -- writable
		IsWrite: true,
		Args:    []bool{true},
	}
	_, err := h.HandleCoils(req)
	if err != nil {
		t.Fatalf("expected writable coil write to succeed, got: %v", err)
	}

	readReq := &mblib.CoilsRequest{UnitId: 1, Addr: 1, Quantity: 1}
	vals, err := h.HandleCoils(readReq)
	if err != nil {
		t.Fatalf("ReadCoils after write: %v", err)
	}
	if !vals[0] {
		t.Error("expected coil to be true after write")
	}
}

func TestHandler_NonGateway_Coil_Write_ReadOnly_ReturnsErrIllegalFunction(t *testing.T) {
	h := makeNonGatewayHandler("one-based")

	req := &mblib.CoilsRequest{
		UnitId:  1,
		Addr:    2, // alarm -- read-only
		IsWrite: true,
		Args:    []bool{true},
	}
	_, err := h.HandleCoils(req)
	if err == nil {
		t.Fatal("expected error for read-only coil write, got nil")
	}
	if err != mblib.ErrIllegalFunction {
		t.Errorf("expected ErrIllegalFunction, got: %v", err)
	}
}

// --- Gateway handler tests ---

func TestHandler_Gateway_UnitID_Routing(t *testing.T) {
	h, store1, store2 := makeGatewayHandler()

	// Write distinct values to each store via the handler to distinguish them.
	writeReq1 := &mblib.HoldingRegistersRequest{
		UnitId: 1, Addr: 1, IsWrite: true, Args: []uint16{111},
	}
	writeReq2 := &mblib.HoldingRegistersRequest{
		UnitId: 2, Addr: 1, IsWrite: true, Args: []uint16{222},
	}

	if _, err := h.HandleHoldingRegisters(writeReq1); err != nil {
		t.Fatalf("write to unit 1: %v", err)
	}
	if _, err := h.HandleHoldingRegisters(writeReq2); err != nil {
		t.Fatalf("write to unit 2: %v", err)
	}

	// Read back through stores directly to confirm routing independence.
	v1, err := store1.ReadHolding(1, 1)
	if err != nil {
		t.Fatalf("store1 read: %v", err)
	}
	v2, err := store2.ReadHolding(1, 1)
	if err != nil {
		t.Fatalf("store2 read: %v", err)
	}

	if v1[0] != 111 {
		t.Errorf("unit 1 store: expected 111, got %d", v1[0])
	}
	if v2[0] != 222 {
		t.Errorf("unit 2 store: expected 222, got %d", v2[0])
	}
}

func TestHandler_Gateway_UnknownUnitID_ReturnsErrGWPathUnavailable(t *testing.T) {
	h, _, _ := makeGatewayHandler()

	req := &mblib.HoldingRegistersRequest{
		UnitId:   99, // not configured
		Addr:     1,
		Quantity: 1,
	}
	_, err := h.HandleHoldingRegisters(req)
	if err == nil {
		t.Fatal("expected ErrGWPathUnavailable for unknown unit ID, got nil")
	}
	if err != mblib.ErrGWPathUnavailable {
		t.Errorf("expected ErrGWPathUnavailable (0x0A), got: %v", err)
	}
}

func TestHandler_Gateway_CoilRouting(t *testing.T) {
	h, _, _ := makeGatewayHandler()

	req := &mblib.CoilsRequest{
		UnitId:  1,
		Addr:    1,
		IsWrite: true,
		Args:    []bool{true},
	}
	if _, err := h.HandleCoils(req); err != nil {
		t.Fatalf("coil write to unit 1: %v", err)
	}

	readReq := &mblib.CoilsRequest{UnitId: 1, Addr: 1, Quantity: 1}
	vals, err := h.HandleCoils(readReq)
	if err != nil {
		t.Fatalf("coil read from unit 1: %v", err)
	}
	if !vals[0] {
		t.Error("expected coil to be true after write")
	}
}

func TestHandler_Gateway_UnknownUnitID_Coils_ReturnsErrGWPathUnavailable(t *testing.T) {
	h, _, _ := makeGatewayHandler()

	req := &mblib.CoilsRequest{
		UnitId:   55,
		Addr:     1,
		Quantity: 1,
	}
	_, err := h.HandleCoils(req)
	if err != mblib.ErrGWPathUnavailable {
		t.Errorf("expected ErrGWPathUnavailable, got: %v", err)
	}
}

func TestHandler_Delay_AppliedForConfiguredUnit(t *testing.T) {
	profile := makeTestProfile("one-based")
	store := mbpkg.NewRegisterStore(profile)

	const expectedDelay = 20 * time.Millisecond
	stores := map[uint8]*mbpkg.RegisterStore{0: store}
	delays := map[uint8]time.Duration{0: expectedDelay}
	jitters := map[uint8]time.Duration{0: 0}

	h := mbpkg.NewHandler(stores, delays, jitters, false, logging.NewTestLogger())

	req := &mblib.HoldingRegistersRequest{UnitId: 1, Addr: 1, Quantity: 1}

	start := time.Now()
	_, err := h.HandleHoldingRegisters(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed < expectedDelay {
		t.Errorf("expected elapsed >= %v, got %v", expectedDelay, elapsed)
	}
}
