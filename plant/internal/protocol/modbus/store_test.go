package modbus_test

import (
	"testing"

	mblib "github.com/simonvetter/modbus"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
	mbpkg "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// makeZeroBasedProfile returns a profile with zero-based addressing for testing.
func makeZeroBasedProfile() *device.DeviceProfile {
	return &device.DeviceProfile{
		PlacementID:       "test-plc",
		DeviceID:          "test-device",
		Addressing:        "zero-based",
		MaxHolding:        256,
		MaxCoils:          256,
		MaxInputRegisters: 10,
		MaxDiscreteInputs: 10,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 0, Name: "flow_rate", Unit: "L/s", Writable: false, InitValue: 16383},
			{Address: 1, Name: "speed_setpoint", Unit: "%", Writable: true, InitValue: 16383},
		},
		Coils: []device.CoilEntry{
			{Address: 0, Name: "pump_run", Writable: true, InitValue: false},
			{Address: 1, Name: "alarm", Writable: false, InitValue: false},
		},
	}
}

// makeOneBasedProfile returns a profile with one-based addressing for testing.
func makeOneBasedProfile() *device.DeviceProfile {
	return &device.DeviceProfile{
		PlacementID:       "test-slc",
		DeviceID:          "slc-500-05",
		Addressing:        "one-based",
		MaxHolding:        256,
		MaxCoils:          256,
		MaxInputRegisters: 10,
		MaxDiscreteInputs: 10,
		HoldingRegisters: []device.RegisterEntry{
			{Address: 1, Name: "conveyor_speed", Unit: "ft/min", Writable: true, InitValue: 16383},
			{Address: 2, Name: "motor_current", Unit: "A", Writable: false, InitValue: 16383},
		},
		Coils: []device.CoilEntry{
			{Address: 1, Name: "conveyor_run", Writable: true, InitValue: false},
			{Address: 2, Name: "e_stop", Writable: false, InitValue: false},
		},
	}
}

func TestNewRegisterStore_InitValues(t *testing.T) {
	profile := makeZeroBasedProfile()
	store := mbpkg.NewRegisterStore(profile)

	vals, err := store.ReadHolding(0, 2)
	if err != nil {
		t.Fatalf("ReadHolding: %v", err)
	}

	if vals[0] != 16383 {
		t.Errorf("addr 0: expected init 16383, got %d", vals[0])
	}
	if vals[1] != 16383 {
		t.Errorf("addr 1: expected init 16383, got %d", vals[1])
	}
}

func TestReadWriteHolding_RoundTrip(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	if err := store.WriteHolding(1, []uint16{9999}); err != nil {
		t.Fatalf("WriteHolding: %v", err)
	}

	vals, err := store.ReadHolding(1, 1)
	if err != nil {
		t.Fatalf("ReadHolding: %v", err)
	}

	if vals[0] != 9999 {
		t.Errorf("expected 9999, got %d", vals[0])
	}
}

func TestWriteHolding_ReadOnly_ReturnsErrIllegalFunction(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	err := store.WriteHolding(0, []uint16{100}) // addr 0 is read-only
	if err == nil {
		t.Fatal("expected error for read-only write, got nil")
	}
	if err != mblib.ErrIllegalFunction {
		t.Errorf("expected ErrIllegalFunction (0x01), got: %v", err)
	}
}

func TestWriteHolding_OutOfRange_ReturnsErrIllegalDataAddress(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	err := store.WriteHolding(999, []uint16{1})
	if err == nil {
		t.Fatal("expected error for out-of-range write, got nil")
	}
	if err != mblib.ErrIllegalDataAddress {
		t.Errorf("expected ErrIllegalDataAddress (0x02), got: %v", err)
	}
}

func TestReadHolding_OutOfRange_ReturnsErrIllegalDataAddress(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	_, err := store.ReadHolding(999, 1)
	if err == nil {
		t.Fatal("expected error for out-of-range read, got nil")
	}
	if err != mblib.ErrIllegalDataAddress {
		t.Errorf("expected ErrIllegalDataAddress, got: %v", err)
	}
}

func TestReadWriteCoils_RoundTrip(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	if err := store.WriteCoils(0, []bool{true}); err != nil {
		t.Fatalf("WriteCoils: %v", err)
	}

	vals, err := store.ReadCoils(0, 1)
	if err != nil {
		t.Fatalf("ReadCoils: %v", err)
	}

	if !vals[0] {
		t.Error("expected coil to be true after write")
	}
}

func TestWriteCoils_ReadOnly_ReturnsErrIllegalFunction(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	err := store.WriteCoils(1, []bool{true}) // addr 1 is alarm (read-only)
	if err == nil {
		t.Fatal("expected error for read-only coil write, got nil")
	}
	if err != mblib.ErrIllegalFunction {
		t.Errorf("expected ErrIllegalFunction (0x01), got: %v", err)
	}
}

func TestReadCoils_OutOfRange_ReturnsErrIllegalDataAddress(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	_, err := store.ReadCoils(999, 1)
	if err == nil {
		t.Fatal("expected error for out-of-range coil read, got nil")
	}
	if err != mblib.ErrIllegalDataAddress {
		t.Errorf("expected ErrIllegalDataAddress, got: %v", err)
	}
}

func TestOneBased_AddressZero_ReturnsErrIllegalDataAddress(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeOneBasedProfile())

	// Address 0 is invalid on one-based devices.
	_, err := store.ReadHolding(0, 1)
	if err == nil {
		t.Fatal("expected error reading address 0 on one-based device, got nil")
	}
	if err != mblib.ErrIllegalDataAddress {
		t.Errorf("expected ErrIllegalDataAddress, got: %v", err)
	}
}

func TestOneBased_AddressOne_Accepted(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeOneBasedProfile())

	vals, err := store.ReadHolding(1, 1)
	if err != nil {
		t.Fatalf("expected address 1 to be valid on one-based device, got: %v", err)
	}
	if vals[0] != 16383 {
		t.Errorf("expected init value 16383, got %d", vals[0])
	}
}

func TestOneBased_WriteReadRoundTrip(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeOneBasedProfile())

	if err := store.WriteHolding(1, []uint16{500}); err != nil {
		t.Fatalf("WriteHolding addr 1: %v", err)
	}

	vals, err := store.ReadHolding(1, 1)
	if err != nil {
		t.Fatalf("ReadHolding addr 1: %v", err)
	}

	if vals[0] != 500 {
		t.Errorf("expected 500, got %d", vals[0])
	}
}

func TestOneBased_CoilAddressZero_ReturnsErrIllegalDataAddress(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeOneBasedProfile())

	_, err := store.ReadCoils(0, 1)
	if err == nil {
		t.Fatal("expected error reading coil 0 on one-based device, got nil")
	}
	if err != mblib.ErrIllegalDataAddress {
		t.Errorf("expected ErrIllegalDataAddress, got: %v", err)
	}
}

func TestReadInputRegisters_ReturnsZeros(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	vals, err := store.ReadInputRegisters(0, 5)
	if err != nil {
		t.Fatalf("ReadInputRegisters: %v", err)
	}

	for i, v := range vals {
		if v != 0 {
			t.Errorf("input register %d: expected 0, got %d", i, v)
		}
	}
}

func TestReadDiscreteInputs_ReturnsZeros(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	vals, err := store.ReadDiscreteInputs(0, 5)
	if err != nil {
		t.Fatalf("ReadDiscreteInputs: %v", err)
	}

	for i, v := range vals {
		if v {
			t.Errorf("discrete input %d: expected false, got true", i)
		}
	}
}

func TestWriteHolding_PartialRangeReadOnly_ReturnsErrIllegalFunction(t *testing.T) {
	// Write a range where one register is read-only: addr 0 (read-only) and addr 1 (writable).
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	err := store.WriteHolding(0, []uint16{1, 2}) // starts at addr 0 which is read-only
	if err == nil {
		t.Fatal("expected error for partial range with read-only register, got nil")
	}
	if err != mblib.ErrIllegalFunction {
		t.Errorf("expected ErrIllegalFunction, got: %v", err)
	}
}

func TestWriteHoldingInternal_BypassesWritableCheck(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	// addr 0 is read-only -- WriteHolding returns ErrIllegalFunction.
	if err := store.WriteHolding(0, []uint16{42}); err != mblib.ErrIllegalFunction {
		t.Fatalf("expected ErrIllegalFunction from WriteHolding on read-only addr, got: %v", err)
	}

	// WriteHoldingInternal must succeed on the same address.
	if err := store.WriteHoldingInternal(0, []uint16{42}); err != nil {
		t.Fatalf("WriteHoldingInternal on read-only addr: %v", err)
	}

	vals, err := store.ReadHolding(0, 1)
	if err != nil {
		t.Fatalf("ReadHolding: %v", err)
	}
	if vals[0] != 42 {
		t.Errorf("expected 42, got %d", vals[0])
	}
}

func TestWriteHoldingInternal_OutOfRange_ReturnsErrIllegalDataAddress(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	err := store.WriteHoldingInternal(999, []uint16{1})
	if err == nil {
		t.Fatal("expected error for out-of-range internal write, got nil")
	}
	if err != mblib.ErrIllegalDataAddress {
		t.Errorf("expected ErrIllegalDataAddress, got: %v", err)
	}
}

func TestWriteCoilsInternal_BypassesWritableCheck(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	// addr 1 is a read-only alarm coil.
	if err := store.WriteCoils(1, []bool{true}); err != mblib.ErrIllegalFunction {
		t.Fatalf("expected ErrIllegalFunction from WriteCoils on read-only coil, got: %v", err)
	}

	// WriteCoilsInternal must succeed on the same address.
	if err := store.WriteCoilsInternal(1, []bool{true}); err != nil {
		t.Fatalf("WriteCoilsInternal on read-only coil: %v", err)
	}

	vals, err := store.ReadCoils(1, 1)
	if err != nil {
		t.Fatalf("ReadCoils: %v", err)
	}
	if !vals[0] {
		t.Error("expected coil to be true after internal write")
	}
}

func TestWriteCoilsInternal_OutOfRange_ReturnsErrIllegalDataAddress(t *testing.T) {
	store := mbpkg.NewRegisterStore(makeZeroBasedProfile())

	err := store.WriteCoilsInternal(999, []bool{true})
	if err == nil {
		t.Fatal("expected error for out-of-range internal coil write, got nil")
	}
	if err != mblib.ErrIllegalDataAddress {
		t.Errorf("expected ErrIllegalDataAddress, got: %v", err)
	}
}
