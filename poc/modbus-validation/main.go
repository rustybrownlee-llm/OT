// POC-001.0: Modbus Library Validation
// Validates simonvetter/modbus as the Modbus TCP server foundation for the OT Simulator.
// Throwaway code. Do not import from production packages.
package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/simonvetter/modbus"
)

// ---------------------------------------------------------------------------
// Handler struct and initialization
// ---------------------------------------------------------------------------

// handler implements modbus.RequestHandler.
// In-memory register maps use zero-based internal indexing.
// For one-based devices (units 1, 2), the handler subtracts baseAddr during
// range checking and array access.
type handler struct {
	holding    map[uint8][]uint16
	coils      map[uint8][]bool
	dinputs    map[uint8][]bool
	iregs      map[uint8][]uint16
	writable   map[uint8]map[uint16]bool // holding register writable flags (one-based key)
	coilWrite  map[uint8]map[uint16]bool // coil writable flags (one-based key)
	baseAddr   map[uint8]uint16
	delays     map[uint8]time.Duration
	configured map[uint8]bool
	mu         sync.RWMutex
}

func newHandler() *handler {
	h := &handler{
		holding:    make(map[uint8][]uint16),
		coils:      make(map[uint8][]bool),
		dinputs:    make(map[uint8][]bool),
		iregs:      make(map[uint8][]uint16),
		writable:   make(map[uint8]map[uint16]bool),
		coilWrite:  make(map[uint8]map[uint16]bool),
		baseAddr:   make(map[uint8]uint16),
		delays:     make(map[uint8]time.Duration),
		configured: make(map[uint8]bool),
	}
	h.initGateway()
	h.initSLC500()
	h.initModicon984()
	h.initOfflineUnit()
	return h
}

// initGateway initializes unit ID 10: Moxa NPort 5150 serial-gateway variant.
// Zero-based addressing. All registers are read-only (gateway status is observed).
// NOTE: Real NPort 5150 devices have no Modbus registers. This is a simulated
// educational abstraction modeling gateway state for training purposes.
func (h *handler) initGateway() {
	const uid uint8 = 10
	h.configured[uid] = true
	h.baseAddr[uid] = 0
	h.delays[uid] = 15 * time.Millisecond

	// Addresses 0-8: serial_port_status through uptime_hours.
	// Enum values: serial_baud_rate=3 (9600), serial_data_format=0 (8N1),
	// serial_mode=2 (RS-485-2wire), serial_port_status=1 (online).
	h.holding[uid] = []uint16{
		1,    // addr 0: serial_port_status (1=online)
		3,    // addr 1: serial_baud_rate (3=9600 baud enum)
		0,    // addr 2: serial_data_format (0=8N1)
		2,    // addr 3: serial_mode (2=RS-485-2wire)
		0,    // addr 4: active_tcp_connections
		0,    // addr 5: serial_tx_count
		0,    // addr 6: serial_rx_count
		0,    // addr 7: serial_error_count
		4320, // addr 8: uptime_hours (180 days)
	}

	// All gateway holding registers are read-only.
	h.writable[uid] = map[uint16]bool{}
}

// initSLC500 initializes unit ID 1: Allen-Bradley SLC 500-05, mfg-line-a variant.
// One-based addressing per ProSoft MVI46-MCM Modbus module convention.
func (h *handler) initSLC500() {
	const uid uint8 = 1
	h.configured[uid] = true
	h.baseAddr[uid] = 1
	h.delays[uid] = 81 * time.Millisecond

	// Holding registers addr 1-7: conveyor_speed through status_word.
	h.holding[uid] = []uint16{
		80,  // addr 1: conveyor_speed (ft/min) -- writable setpoint
		12,  // addr 2: motor_current (A) -- read-only sensor
		247, // addr 3: product_count (units) -- read-only counter
		3,   // addr 4: reject_count (units) -- read-only counter
		72,  // addr 5: line_temperature (degF) -- read-only sensor
		45,  // addr 6: cycle_time (s) -- read-only measurement
		1,   // addr 7: status_word (bitmask, bit0=running) -- read-only
	}

	// Only conveyor_speed (addr 1) is writable.
	h.writable[uid] = map[uint16]bool{
		1: true,
	}

	// Coils addr 1-4: conveyor_run through jam_detected.
	h.coils[uid] = []bool{
		false, // addr 1: conveyor_run -- writable command
		false, // addr 2: conveyor_direction -- writable command
		false, // addr 3: e_stop_active -- read-only (hardwired safety circuit)
		false, // addr 4: jam_detected -- read-only status
	}

	// conveyor_run (addr 1) and conveyor_direction (addr 2) are writable.
	h.coilWrite[uid] = map[uint16]bool{
		1: true,
		2: true,
	}

	// Input registers addr 1-3: static sensor feedback values.
	h.iregs[uid] = []uint16{
		82,  // addr 1: line_voltage_pct (scaled 82% of nominal)
		415, // addr 2: vfd_dc_bus_voltage (V)
		28,  // addr 3: brake_temperature (degF above ambient)
	}

	// Discrete inputs addr 1-2: static status bits.
	h.dinputs[uid] = []bool{
		true,  // addr 1: power_ok
		false, // addr 2: maintenance_due
	}
}

// initModicon984 initializes unit ID 2: Schneider Electric Modicon 984, mfg-cooling variant.
// One-based addressing per the original Modbus specification authored by Modicon.
func (h *handler) initModicon984() {
	const uid uint8 = 2
	h.configured[uid] = true
	h.baseAddr[uid] = 1
	h.delays[uid] = 111 * time.Millisecond

	// Holding registers addr 1-7: supply_temp through pump_runtime_hours.
	h.holding[uid] = []uint16{
		62,   // addr 1: supply_temp (degF) -- read-only sensor
		74,   // addr 2: return_temp (degF) -- read-only sensor
		220,  // addr 3: flow_rate (GPM) -- read-only sensor
		38,   // addr 4: pump_pressure (PSI) -- read-only sensor
		85,   // addr 5: tank_level (%) -- read-only sensor
		65,   // addr 6: setpoint_temp (degF) -- writable setpoint
		1823, // addr 7: pump_runtime_hours (hours) -- read-only counter
	}

	// Only setpoint_temp (addr 6) is writable.
	h.writable[uid] = map[uint16]bool{
		6: true,
	}

	// Coils addr 1-4: pump_1_run through high_temp_alarm.
	h.coils[uid] = []bool{
		true,  // addr 1: pump_1_run -- writable (lead pump running)
		false, // addr 2: pump_2_run -- writable (lag pump stopped)
		false, // addr 3: low_coolant_alarm -- read-only alarm
		false, // addr 4: high_temp_alarm -- read-only alarm
	}

	// pump_1_run (addr 1) and pump_2_run (addr 2) are writable.
	h.coilWrite[uid] = map[uint16]bool{
		1: true,
		2: true,
	}

	// Input registers addr 1-3: static sensor feedback values.
	h.iregs[uid] = []uint16{
		42,  // addr 1: makeup_water_conductivity (uS/cm)
		5,   // addr 2: tower_fan_current (A)
		91,  // addr 3: chiller_outlet_temp_pct (scaled, 91% of range)
	}

	// Discrete inputs addr 1-2: static status bits.
	h.dinputs[uid] = []bool{
		true,  // addr 1: chiller_online
		false, // addr 2: bypass_valve_open
	}
}

// initOfflineUnit configures unit ID 3 as a device with a gateway route but
// no register data, simulating a device that is powered off or cable-disconnected.
func (h *handler) initOfflineUnit() {
	const uid uint8 = 3
	h.configured[uid] = true
	// All register maps remain nil -- online check returns ErrGWTargetFailedToRespond.
}

// ---------------------------------------------------------------------------
// RequestHandler implementation
// ---------------------------------------------------------------------------

// routeCheck validates the unit ID and returns whether the unit is online.
// Returns ErrGWPathUnavailable if the unit ID has no configured route.
// Returns ErrGWTargetFailedToRespond if the route exists but unit is offline.
func (h *handler) routeCheck(uid uint8) error {
	if !h.configured[uid] {
		return modbus.ErrGWPathUnavailable
	}
	if h.holding[uid] == nil && h.coils[uid] == nil &&
		h.dinputs[uid] == nil && h.iregs[uid] == nil {
		return modbus.ErrGWTargetFailedToRespond
	}
	return nil
}

// HandleHoldingRegisters serves FC03 (read), FC06 (write single), FC10 (write multiple).
func (h *handler) HandleHoldingRegisters(req *modbus.HoldingRegistersRequest) ([]uint16, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.routeCheck(req.UnitId); err != nil {
		return nil, err
	}

	time.Sleep(h.delays[req.UnitId])

	base := h.baseAddr[req.UnitId]
	regs := h.holding[req.UnitId]

	// Reject addresses below the base address (catches addr 0 on one-based devices).
	if req.Addr < base {
		return nil, modbus.ErrIllegalDataAddress
	}

	idx := req.Addr - base
	end := uint32(idx) + uint32(req.Quantity)
	if end > uint32(len(regs)) {
		return nil, modbus.ErrIllegalDataAddress
	}

	if req.IsWrite {
		for i, v := range req.Args {
			addr := req.Addr + uint16(i)
			if !h.writable[req.UnitId][addr] {
				return nil, modbus.ErrIllegalDataAddress
			}
			regs[idx+uint16(i)] = v
		}
		return nil, nil
	}

	result := make([]uint16, req.Quantity)
	copy(result, regs[idx:idx+req.Quantity])
	return result, nil
}

// HandleCoils serves FC01 (read), FC05 (write single), FC0F (write multiple).
func (h *handler) HandleCoils(req *modbus.CoilsRequest) ([]bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.routeCheck(req.UnitId); err != nil {
		return nil, err
	}

	time.Sleep(h.delays[req.UnitId])

	base := h.baseAddr[req.UnitId]
	coils := h.coils[req.UnitId]

	if req.Addr < base {
		return nil, modbus.ErrIllegalDataAddress
	}

	idx := req.Addr - base
	end := uint32(idx) + uint32(req.Quantity)
	if end > uint32(len(coils)) {
		return nil, modbus.ErrIllegalDataAddress
	}

	if req.IsWrite {
		for i, v := range req.Args {
			addr := req.Addr + uint16(i)
			if !h.coilWrite[req.UnitId][addr] {
				return nil, modbus.ErrIllegalDataAddress
			}
			coils[idx+uint16(i)] = v
		}
		return nil, nil
	}

	result := make([]bool, req.Quantity)
	copy(result, coils[idx:idx+req.Quantity])
	return result, nil
}

// HandleInputRegisters serves FC04 (read-only).
func (h *handler) HandleInputRegisters(req *modbus.InputRegistersRequest) ([]uint16, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if err := h.routeCheck(req.UnitId); err != nil {
		return nil, err
	}

	time.Sleep(h.delays[req.UnitId])

	base := h.baseAddr[req.UnitId]
	regs := h.iregs[req.UnitId]

	if req.Addr < base {
		return nil, modbus.ErrIllegalDataAddress
	}

	idx := req.Addr - base
	end := uint32(idx) + uint32(req.Quantity)
	if end > uint32(len(regs)) {
		return nil, modbus.ErrIllegalDataAddress
	}

	result := make([]uint16, req.Quantity)
	copy(result, regs[idx:idx+req.Quantity])
	return result, nil
}

// HandleDiscreteInputs serves FC02 (read-only).
func (h *handler) HandleDiscreteInputs(req *modbus.DiscreteInputsRequest) ([]bool, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if err := h.routeCheck(req.UnitId); err != nil {
		return nil, err
	}

	time.Sleep(h.delays[req.UnitId])

	base := h.baseAddr[req.UnitId]
	inputs := h.dinputs[req.UnitId]

	if req.Addr < base {
		return nil, modbus.ErrIllegalDataAddress
	}

	idx := req.Addr - base
	end := uint32(idx) + uint32(req.Quantity)
	if end > uint32(len(inputs)) {
		return nil, modbus.ErrIllegalDataAddress
	}

	result := make([]bool, req.Quantity)
	copy(result, inputs[idx:idx+req.Quantity])
	return result, nil
}

// ---------------------------------------------------------------------------
// Client test infrastructure
// ---------------------------------------------------------------------------

type testResult struct {
	id     int
	name   string
	passed bool
	detail string
}

func newClient() (*modbus.ModbusClient, error) {
	c, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:     "tcp://localhost:5020",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return c, c.Open()
}

func pass(id int, name string) testResult {
	return testResult{id: id, name: name, passed: true}
}

func fail(id int, name, detail string) testResult {
	return testResult{id: id, name: name, passed: false, detail: detail}
}

// ---------------------------------------------------------------------------
// Individual tests T1-T16
// ---------------------------------------------------------------------------

func testT1(c *modbus.ModbusClient) testResult {
	const name = "Read holding registers, SLC-500 (unit 1)"
	if err := c.SetUnitId(1); err != nil {
		return fail(1, name, err.Error())
	}
	vals, err := c.ReadRegisters(1, 7, modbus.HOLDING_REGISTER)
	if err != nil {
		return fail(1, name, err.Error())
	}
	expected := []uint16{80, 12, 247, 3, 72, 45, 1}
	for i, v := range expected {
		if vals[i] != v {
			return fail(1, name, fmt.Sprintf("addr %d: got %d, want %d", i+1, vals[i], v))
		}
	}
	return pass(1, name)
}

func testT2(c *modbus.ModbusClient) testResult {
	const name = "Read coils, SLC-500 (unit 1)"
	if err := c.SetUnitId(1); err != nil {
		return fail(2, name, err.Error())
	}
	vals, err := c.ReadCoils(1, 4)
	if err != nil {
		return fail(2, name, err.Error())
	}
	expected := []bool{false, false, false, false}
	for i, v := range expected {
		if vals[i] != v {
			return fail(2, name, fmt.Sprintf("coil %d: got %v, want %v", i+1, vals[i], v))
		}
	}
	return pass(2, name)
}

func testT3(c *modbus.ModbusClient) testResult {
	const name = "Write single coil (unit 1)"
	if err := c.SetUnitId(1); err != nil {
		return fail(3, name, err.Error())
	}
	if err := c.WriteCoil(1, true); err != nil {
		return fail(3, name, "write failed: "+err.Error())
	}
	vals, err := c.ReadCoils(1, 1)
	if err != nil {
		return fail(3, name, "re-read failed: "+err.Error())
	}
	if !vals[0] {
		return fail(3, name, "coil did not persist after write")
	}
	return pass(3, name)
}

func testT4(c *modbus.ModbusClient) testResult {
	const name = "Write single register (unit 1)"
	if err := c.SetUnitId(1); err != nil {
		return fail(4, name, err.Error())
	}
	if err := c.WriteRegister(1, 500); err != nil {
		return fail(4, name, "write failed: "+err.Error())
	}
	vals, err := c.ReadRegisters(1, 1, modbus.HOLDING_REGISTER)
	if err != nil {
		return fail(4, name, "re-read failed: "+err.Error())
	}
	if vals[0] != 500 {
		return fail(4, name, fmt.Sprintf("got %d, want 500", vals[0]))
	}
	return pass(4, name)
}

func testT5(c *modbus.ModbusClient) testResult {
	const name = "Read input registers, Modicon 984 (unit 2)"
	if err := c.SetUnitId(2); err != nil {
		return fail(5, name, err.Error())
	}
	vals, err := c.ReadRegisters(1, 3, modbus.INPUT_REGISTER)
	if err != nil {
		return fail(5, name, err.Error())
	}
	expected := []uint16{42, 5, 91}
	for i, v := range expected {
		if vals[i] != v {
			return fail(5, name, fmt.Sprintf("addr %d: got %d, want %d", i+1, vals[i], v))
		}
	}
	return pass(5, name)
}

func testT6(c *modbus.ModbusClient) testResult {
	const name = "Read discrete inputs, Modicon 984 (unit 2)"
	if err := c.SetUnitId(2); err != nil {
		return fail(6, name, err.Error())
	}
	vals, err := c.ReadDiscreteInputs(1, 2)
	if err != nil {
		return fail(6, name, err.Error())
	}
	expected := []bool{true, false}
	for i, v := range expected {
		if vals[i] != v {
			return fail(6, name, fmt.Sprintf("input %d: got %v, want %v", i+1, vals[i], v))
		}
	}
	return pass(6, name)
}

func testT7(c *modbus.ModbusClient) testResult {
	const name = "Unit ID routing (units 1, 2, 10)"

	read := func(uid uint8, addr uint16) (uint16, error) {
		if err := c.SetUnitId(uid); err != nil {
			return 0, err
		}
		vals, err := c.ReadRegisters(addr, 1, modbus.HOLDING_REGISTER)
		if err != nil {
			return 0, err
		}
		return vals[0], nil
	}

	// Unit 1 addr 1: conveyor_speed = 500 (was written in T4)
	v1, err := read(1, 1)
	if err != nil {
		return fail(7, name, "unit 1 read failed: "+err.Error())
	}

	// Unit 2 addr 1: supply_temp = 62
	v2, err := read(2, 1)
	if err != nil {
		return fail(7, name, "unit 2 read failed: "+err.Error())
	}

	// Unit 10 addr 0: serial_port_status = 1
	v10, err := read(10, 0)
	if err != nil {
		return fail(7, name, "unit 10 read failed: "+err.Error())
	}

	if v1 == v2 && v2 == v10 {
		return fail(7, name, fmt.Sprintf("all units returned same value %d -- routing broken", v1))
	}

	// Verify expected values.
	if v1 != 500 {
		return fail(7, name, fmt.Sprintf("unit 1 addr 1: got %d, want 500", v1))
	}
	if v2 != 62 {
		return fail(7, name, fmt.Sprintf("unit 2 addr 1: got %d, want 62", v2))
	}
	if v10 != 1 {
		return fail(7, name, fmt.Sprintf("unit 10 addr 0: got %d, want 1", v10))
	}

	return pass(7, name)
}

func testT8(c *modbus.ModbusClient) testResult {
	const name = "Unknown unit ID - path unavailable (0x0A)"
	if err := c.SetUnitId(99); err != nil {
		return fail(8, name, err.Error())
	}
	_, err := c.ReadRegisters(1, 1, modbus.HOLDING_REGISTER)
	if err == nil {
		return fail(8, name, "expected ErrGWPathUnavailable, got nil")
	}
	if err != modbus.ErrGWPathUnavailable {
		return fail(8, name, fmt.Sprintf("expected ErrGWPathUnavailable, got: %v", err))
	}
	return pass(8, name)
}

func testT9(c *modbus.ModbusClient) testResult {
	const name = "Configured offline unit - target timeout (0x0B)"
	if err := c.SetUnitId(3); err != nil {
		return fail(9, name, err.Error())
	}
	_, err := c.ReadRegisters(1, 1, modbus.HOLDING_REGISTER)
	if err == nil {
		return fail(9, name, "expected ErrGWTargetFailedToRespond, got nil")
	}
	if err != modbus.ErrGWTargetFailedToRespond {
		return fail(9, name, fmt.Sprintf("expected ErrGWTargetFailedToRespond, got: %v", err))
	}
	return pass(9, name)
}

func testT10(c *modbus.ModbusClient) testResult {
	const name = "Out-of-range address"
	if err := c.SetUnitId(1); err != nil {
		return fail(10, name, err.Error())
	}
	_, err := c.ReadRegisters(999, 1, modbus.HOLDING_REGISTER)
	if err == nil {
		return fail(10, name, "expected ErrIllegalDataAddress, got nil")
	}
	if err != modbus.ErrIllegalDataAddress {
		return fail(10, name, fmt.Sprintf("expected ErrIllegalDataAddress, got: %v", err))
	}
	return pass(10, name)
}

func testT11(c *modbus.ModbusClient) testResult {
	const name = "Below-base address (one-based device)"
	if err := c.SetUnitId(2); err != nil {
		return fail(11, name, err.Error())
	}
	_, err := c.ReadRegisters(0, 1, modbus.HOLDING_REGISTER)
	if err == nil {
		return fail(11, name, "expected ErrIllegalDataAddress, got nil")
	}
	if err != modbus.ErrIllegalDataAddress {
		return fail(11, name, fmt.Sprintf("expected ErrIllegalDataAddress, got: %v", err))
	}
	return pass(11, name)
}

type delayMeasurement struct {
	unitID     uint8
	label      string
	targetMs   int
	floorMs    int
	addr       uint16
	measuredMs float64
}

func measureDelay(c *modbus.ModbusClient, uid uint8, addr uint16) (float64, error) {
	const samples = 5
	if err := c.SetUnitId(uid); err != nil {
		return 0, err
	}
	var total time.Duration
	for i := 0; i < samples; i++ {
		start := time.Now()
		_, err := c.ReadRegisters(addr, 1, modbus.HOLDING_REGISTER)
		elapsed := time.Since(start)
		if err != nil {
			return 0, err
		}
		total += elapsed
	}
	return float64(total.Milliseconds()) / samples, nil
}

func testT12(c *modbus.ModbusClient) (testResult, []delayMeasurement) {
	const name = "Response delay measurement"
	measurements := []delayMeasurement{
		{unitID: 10, label: "gateway", targetMs: 15, floorMs: 12, addr: 0},
		{unitID: 1, label: "SLC-500 via gw", targetMs: 81, floorMs: 65, addr: 1},
		{unitID: 2, label: "Modicon via gw", targetMs: 111, floorMs: 89, addr: 1},
	}

	allPass := true
	for i := range measurements {
		m := &measurements[i]
		avg, err := measureDelay(c, m.unitID, m.addr)
		if err != nil {
			return fail(12, name, fmt.Sprintf("unit %d read failed: %v", m.unitID, err)), measurements
		}
		m.measuredMs = avg
		if avg < float64(m.floorMs) {
			allPass = false
		}
	}

	if !allPass {
		return fail(12, name, "one or more units did not meet delay floor"), measurements
	}
	return pass(12, name), measurements
}

func testT13() testResult {
	const name = "Concurrent clients"
	var wg sync.WaitGroup
	errs := make([]error, 2)

	runConcurrent := func(idx int, uid uint8, addr uint16, want uint16) {
		defer wg.Done()
		c, err := modbus.NewClient(&modbus.ClientConfiguration{
			URL:     "tcp://localhost:5020",
			Timeout: 5 * time.Second,
		})
		if err != nil {
			errs[idx] = err
			return
		}
		if err = c.Open(); err != nil {
			errs[idx] = err
			return
		}
		defer c.Close()

		if err = c.SetUnitId(uid); err != nil {
			errs[idx] = err
			return
		}
		vals, err := c.ReadRegisters(addr, 1, modbus.HOLDING_REGISTER)
		if err != nil {
			errs[idx] = err
			return
		}
		if vals[0] != want {
			errs[idx] = fmt.Errorf("unit %d addr %d: got %d, want %d", uid, addr, vals[0], want)
		}
	}

	wg.Add(2)
	// Unit 1 addr 2: motor_current = 12 (unchanged by prior tests)
	go runConcurrent(0, 1, 2, 12)
	// Unit 2 addr 2: return_temp = 74 (unchanged by prior tests)
	go runConcurrent(1, 2, 2, 74)
	wg.Wait()

	for _, e := range errs {
		if e != nil {
			return fail(13, name, e.Error())
		}
	}
	return pass(13, name)
}

func testT14(c *modbus.ModbusClient) testResult {
	const name = "Write multiple registers"
	if err := c.SetUnitId(2); err != nil {
		return fail(14, name, err.Error())
	}
	// Write setpoint_temp (addr 6) via FC10.
	if err := c.WriteRegisters(6, []uint16{70}); err != nil {
		return fail(14, name, "write failed: "+err.Error())
	}
	vals, err := c.ReadRegisters(6, 1, modbus.HOLDING_REGISTER)
	if err != nil {
		return fail(14, name, "re-read failed: "+err.Error())
	}
	if vals[0] != 70 {
		return fail(14, name, fmt.Sprintf("got %d, want 70", vals[0]))
	}
	return pass(14, name)
}

func testT15(c *modbus.ModbusClient) testResult {
	const name = "Write multiple coils"
	if err := c.SetUnitId(1); err != nil {
		return fail(15, name, err.Error())
	}
	// Write conveyor_run (addr 1) and conveyor_direction (addr 2) via FC0F.
	if err := c.WriteCoils(1, []bool{true, true}); err != nil {
		return fail(15, name, "write failed: "+err.Error())
	}
	vals, err := c.ReadCoils(1, 2)
	if err != nil {
		return fail(15, name, "re-read failed: "+err.Error())
	}
	if !vals[0] || !vals[1] {
		return fail(15, name, fmt.Sprintf("coils did not persist: %v", vals))
	}
	return pass(15, name)
}

func testT16(c *modbus.ModbusClient) testResult {
	const name = "Write to read-only register (security)"
	if err := c.SetUnitId(2); err != nil {
		return fail(16, name, err.Error())
	}
	// supply_temp (addr 1) is read-only on Modicon 984.
	err := c.WriteRegister(1, 999)
	if err == nil {
		return fail(16, name, "expected exception on write to read-only register, got nil")
	}
	// Any Modbus exception is acceptable; the write must not silently succeed.
	return pass(16, name)
}

// ---------------------------------------------------------------------------
// Main: start server, wait, run tests, print results
// ---------------------------------------------------------------------------

func main() {
	fmt.Println("=== POC-001: Modbus Library Validation ===")
	fmt.Println("Library: simonvetter/modbus")
	fmt.Println("Server:  localhost:5020")
	fmt.Println()

	h := newHandler()
	srv, err := modbus.NewServer(&modbus.ServerConfiguration{
		URL:        "tcp://localhost:5020",
		MaxClients: 10,
	}, h)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server create failed: %v\n", err)
		os.Exit(1)
	}
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "server start failed: %v\n", err)
		os.Exit(1)
	}
	defer srv.Stop()

	time.Sleep(100 * time.Millisecond)

	c, err := newClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "client connect failed: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	results := make([]testResult, 0, 16)
	var delayMeasurements []delayMeasurement

	results = append(results, testT1(c))
	results = append(results, testT2(c))
	results = append(results, testT3(c))
	results = append(results, testT4(c))
	results = append(results, testT5(c))
	results = append(results, testT6(c))
	results = append(results, testT7(c))
	results = append(results, testT8(c))
	results = append(results, testT9(c))
	results = append(results, testT10(c))
	results = append(results, testT11(c))

	t12result, delayMeasurements := testT12(c)
	results = append(results, t12result)

	results = append(results, testT13())
	results = append(results, testT14(c))
	results = append(results, testT15(c))
	results = append(results, testT16(c))

	printResults(results, delayMeasurements)
}

func printResults(results []testResult, delays []delayMeasurement) {
	labels := []string{
		"Read holding registers, SLC-500 (unit 1)      ",
		"Read coils, SLC-500 (unit 1)                  ",
		"Write single coil (unit 1)                    ",
		"Write single register (unit 1)                ",
		"Read input registers, Modicon 984 (unit 2)    ",
		"Read discrete inputs, Modicon 984 (unit 2)    ",
		"Unit ID routing (units 1, 2, 10)              ",
		"Unknown unit ID - path unavailable (0x0A)     ",
		"Configured offline unit - target timeout (0x0B)",
		"Out-of-range address                          ",
		"Below-base address (one-based device)         ",
		"Response delay measurement                    ",
		"Concurrent clients                            ",
		"Write multiple registers                      ",
		"Write multiple coils                          ",
		"Write to read-only register (security)        ",
	}

	passed := 0
	for i, r := range results {
		status := "PASS"
		if r.passed {
			passed++
		} else {
			status = "FAIL"
		}
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		fmt.Printf("T%-2d %s %s\n", r.id, label, status)
		if !r.passed && r.detail != "" {
			fmt.Printf("    DETAIL: %s\n", r.detail)
		}
	}

	fmt.Println()
	fmt.Println("Response delay measurements:")
	for _, m := range delays {
		floor := m.floorMs
		status := "PASS"
		if m.measuredMs < float64(floor) {
			status = "FAIL"
		}
		fmt.Printf("  Unit %-2d (%-18s target %3dms):  avg %5.1fms   %s (>= %dms)\n",
			m.unitID, m.label+",", m.targetMs, m.measuredMs, status, floor)
	}

	fmt.Println()
	total := len(results)
	if passed == total {
		fmt.Printf("=== VERDICT: PASS (%d/%d tests passed) ===\n", passed, total)
		os.Exit(0)
	} else {
		fmt.Printf("=== VERDICT: FAIL (%d/%d tests passed) ===\n", passed, total)
		os.Exit(1)
	}
}
