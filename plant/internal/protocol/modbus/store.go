// Package modbus implements the Modbus TCP protocol adapter for the plant
// simulator. It wraps the simonvetter/modbus library to provide a server that
// exposes each virtual PLC's register map over a dedicated TCP port in the
// range 5020-5039.
//
// The adapter handles MBAP framing, function code dispatch, and exception
// response generation. Per ADR-002, Modbus TCP is the Phase 1 protocol.
// Modbus RTU over simulated serial is Phase 2 (see protocol/serial).
//
// Wire-level fidelity is the primary goal: function codes, register addresses,
// byte order, and exception codes must match the Modbus Application Protocol
// Specification V1.1b3.
//
// Implemented in SOW-001.0.
package modbus

import (
	"sync"

	"github.com/simonvetter/modbus"

	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
)

// RegisterStore provides thread-safe read/write access to a device's register state.
// It is the single source of truth for register values during simulation.
// SOW-003.0 will add methods for process simulation to update values.
//
// Scaling convention: raw value 0 = scale_min, raw value 32767 = scale_max.
// This is the standard Modbus holding register convention (4xxxx address space).
// SOW-003.0 depends on this convention for process value computation.
//
// PROTOTYPE-DEBT: [td-store-008] Input registers (FC04) and discrete inputs (FC02) return zeros.
// No register entries populate them. Resolution: TD-005 -- add input register entries to device atoms.
type RegisterStore struct {
	holding   []uint16       // indexed by (address - baseAddr)
	coils     []bool         // indexed by (address - baseAddr)
	dinputs   []bool         // discrete inputs -- static zeros (TD-008)
	iregs     []uint16       // input registers -- static zeros (TD-008)
	writable  map[int]bool   // holding register address -> writable flag
	coilWrite map[int]bool   // coil address -> writable flag
	baseAddr  int            // 0 for zero-based, 1 for one-based
	mu        sync.RWMutex
}

// NewRegisterStore creates a store initialized from the given DeviceProfile.
// The store's internal arrays are zero-indexed; baseAddr is subtracted from
// all incoming addresses before indexing.
func NewRegisterStore(profile *device.DeviceProfile) *RegisterStore {
	baseAddr := baseAddress(profile.Addressing)

	maxH := maxIndex(profile.HoldingRegisters, baseAddr, profile.MaxHolding)
	holding := make([]uint16, maxH)
	writable := make(map[int]bool, len(profile.HoldingRegisters))

	for _, r := range profile.HoldingRegisters {
		idx := r.Address - baseAddr
		if idx >= 0 && idx < maxH {
			holding[idx] = r.InitValue
		}
		writable[r.Address] = r.Writable
	}

	maxC := maxCoilIndex(profile.Coils, baseAddr, profile.MaxCoils)
	coils := make([]bool, maxC)
	coilWrite := make(map[int]bool, len(profile.Coils))

	for _, c := range profile.Coils {
		// InitValue is always false per SOW-002.0 spec.
		coilWrite[c.Address] = c.Writable
	}

	dinputs := make([]bool, profile.MaxDiscreteInputs)
	iregs := make([]uint16, profile.MaxInputRegisters)

	return &RegisterStore{
		holding:   holding,
		coils:     coils,
		dinputs:   dinputs,
		iregs:     iregs,
		writable:  writable,
		coilWrite: coilWrite,
		baseAddr:  baseAddr,
	}
}

// ReadHolding returns qty register values starting at addr.
// Returns ErrIllegalDataAddress if addr is below baseAddr or the range is out of bounds.
func (s *RegisterStore) ReadHolding(addr, qty uint16) ([]uint16, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, end, err := s.holdingBounds(addr, qty)
	if err != nil {
		return nil, err
	}

	result := make([]uint16, qty)
	copy(result, s.holding[idx:end])
	return result, nil
}

// WriteHolding writes values to holding registers starting at addr.
// Returns ErrIllegalFunction if any register in the range is not writable.
// Returns ErrIllegalDataAddress if addr is below baseAddr or the range is out of bounds.
func (s *RegisterStore) WriteHolding(addr uint16, values []uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, end, err := s.holdingBounds(addr, uint16(len(values)))
	if err != nil {
		return err
	}

	for i := range values {
		regAddr := int(addr) + i
		if !s.writable[regAddr] {
			// PROTOTYPE-DEBT: [td-store-006] ErrIllegalFunction returned for read-only writes.
			// Teaches learners: register exists and is readable, but write is not permitted.
			// This is distinct from ErrIllegalDataAddress (address does not exist).
			return modbus.ErrIllegalFunction
		}
	}

	copy(s.holding[idx:end], values)
	return nil
}

// ReadCoils returns qty coil values starting at addr.
// Returns ErrIllegalDataAddress if addr is below baseAddr or the range is out of bounds.
func (s *RegisterStore) ReadCoils(addr, qty uint16) ([]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, end, err := s.coilBounds(addr, qty)
	if err != nil {
		return nil, err
	}

	result := make([]bool, qty)
	copy(result, s.coils[idx:end])
	return result, nil
}

// WriteCoils writes values to coils starting at addr.
// Returns ErrIllegalFunction if any coil in the range is not writable.
// Returns ErrIllegalDataAddress if addr is below baseAddr or the range is out of bounds.
func (s *RegisterStore) WriteCoils(addr uint16, values []bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, end, err := s.coilBounds(addr, uint16(len(values)))
	if err != nil {
		return err
	}

	for i := range values {
		coilAddr := int(addr) + i
		if !s.coilWrite[coilAddr] {
			return modbus.ErrIllegalFunction
		}
	}

	copy(s.coils[idx:end], values)
	return nil
}

// ReadInputRegisters returns qty input register values starting at addr.
// Returns zeros within device capacity (TD-008). Returns ErrIllegalDataAddress if out of bounds.
func (s *RegisterStore) ReadInputRegisters(addr, qty uint16) ([]uint16, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, end, err := s.inputRegBounds(addr, qty)
	if err != nil {
		return nil, err
	}

	result := make([]uint16, qty)
	copy(result, s.iregs[idx:end])
	return result, nil
}

// ReadDiscreteInputs returns qty discrete input values starting at addr.
// Returns zeros within device capacity (TD-008). Returns ErrIllegalDataAddress if out of bounds.
func (s *RegisterStore) ReadDiscreteInputs(addr, qty uint16) ([]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, end, err := s.discreteInputBounds(addr, qty)
	if err != nil {
		return nil, err
	}

	result := make([]bool, qty)
	copy(result, s.dinputs[idx:end])
	return result, nil
}

// WriteHoldingInternal writes values to holding registers starting at addr, bypassing the
// writable flag check. This method is used exclusively by the simulation engine to update
// read-only sensor registers (e.g., flow_rate, temperature) on each tick. External Modbus
// client writes continue to use WriteHolding, which enforces writable flags.
//
// Returns ErrIllegalDataAddress if addr is below baseAddr or the range is out of bounds.
func (s *RegisterStore) WriteHoldingInternal(addr uint16, values []uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, end, err := s.holdingBounds(addr, uint16(len(values)))
	if err != nil {
		return err
	}

	copy(s.holding[idx:end], values)
	return nil
}

// WriteCoilsInternal writes coil values starting at addr, bypassing the writable flag check.
// This method is used exclusively by the simulation engine to update read-only alarm coils
// (e.g., high_dp_alarm, low_coolant_alarm) on each tick. External Modbus client writes
// continue to use WriteCoils, which enforces writable flags.
//
// Returns ErrIllegalDataAddress if addr is below baseAddr or the range is out of bounds.
func (s *RegisterStore) WriteCoilsInternal(addr uint16, values []bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, end, err := s.coilBounds(addr, uint16(len(values)))
	if err != nil {
		return err
	}

	copy(s.coils[idx:end], values)
	return nil
}

// holdingBounds validates a holding register range and returns zero-based start/end indices.
func (s *RegisterStore) holdingBounds(addr, qty uint16) (int, int, error) {
	if int(addr) < s.baseAddr {
		return 0, 0, modbus.ErrIllegalDataAddress
	}
	idx := int(addr) - s.baseAddr
	end := idx + int(qty)
	if end > len(s.holding) {
		return 0, 0, modbus.ErrIllegalDataAddress
	}
	return idx, end, nil
}

// coilBounds validates a coil range and returns zero-based start/end indices.
func (s *RegisterStore) coilBounds(addr, qty uint16) (int, int, error) {
	if int(addr) < s.baseAddr {
		return 0, 0, modbus.ErrIllegalDataAddress
	}
	idx := int(addr) - s.baseAddr
	end := idx + int(qty)
	if end > len(s.coils) {
		return 0, 0, modbus.ErrIllegalDataAddress
	}
	return idx, end, nil
}

// inputRegBounds validates an input register range. Input registers use zero-based addressing.
func (s *RegisterStore) inputRegBounds(addr, qty uint16) (int, int, error) {
	idx := int(addr)
	end := idx + int(qty)
	if end > len(s.iregs) {
		return 0, 0, modbus.ErrIllegalDataAddress
	}
	return idx, end, nil
}

// discreteInputBounds validates a discrete input range. Discrete inputs use zero-based addressing.
func (s *RegisterStore) discreteInputBounds(addr, qty uint16) (int, int, error) {
	idx := int(addr)
	end := idx + int(qty)
	if end > len(s.dinputs) {
		return 0, 0, modbus.ErrIllegalDataAddress
	}
	return idx, end, nil
}

// baseAddress returns 0 for zero-based addressing, 1 for one-based addressing.
func baseAddress(addressing string) int {
	if addressing == "one-based" {
		return 1
	}
	return 0
}

// maxIndex computes the required holding array length from the register entries and device capacity.
func maxIndex(entries []device.RegisterEntry, baseAddr, deviceMax int) int {
	if len(entries) == 0 {
		return 0
	}
	highest := 0
	for _, e := range entries {
		idx := e.Address - baseAddr
		if idx > highest {
			highest = idx
		}
	}
	size := highest + 1
	if size < deviceMax {
		return size
	}
	return deviceMax
}

// maxCoilIndex computes the required coil array length from the coil entries and device capacity.
func maxCoilIndex(entries []device.CoilEntry, baseAddr, deviceMax int) int {
	if len(entries) == 0 {
		return 0
	}
	highest := 0
	for _, e := range entries {
		idx := e.Address - baseAddr
		if idx > highest {
			highest = idx
		}
	}
	size := highest + 1
	if size < deviceMax {
		return size
	}
	return deviceMax
}
