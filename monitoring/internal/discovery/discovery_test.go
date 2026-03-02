// Package discovery provides unit tests for register enumeration and device discovery.
// Tests use a mock Modbus client to avoid requiring a live plant binary.
// These are white-box tests within the same package to access unexported functions.
package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/simonvetter/modbus"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/config"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/inventory"
)

func init() {
	// Disable the per-read delay during tests so the binary-search enumeration
	// completes in milliseconds rather than seconds.
	discoveryReadDelay = 0 * time.Millisecond
}

// mockClient is a configurable Modbus client mock for unit testing.
// It returns pre-configured responses for specific address ranges.
type mockClient struct {
	// holdingRegs maps address to value. Addresses not in map return ErrIllegalDataAddress.
	holdingRegs map[uint16]uint16
	// coils maps address to value. Addresses not in map return ErrIllegalDataAddress.
	coils map[uint16]bool
	// unitIDLog records all SetUnitId calls.
	unitIDLog []uint8
	// openErr is returned by Open if set.
	openErr error
}

func (m *mockClient) Open() error { return m.openErr }
func (m *mockClient) Close() error { return nil }

func (m *mockClient) SetUnitId(id uint8) error {
	m.unitIDLog = append(m.unitIDLog, id)
	return nil
}

func (m *mockClient) ReadRegisters(addr uint16, quantity uint16, regType modbus.RegType) ([]uint16, error) {
	vals := make([]uint16, 0, quantity)
	for i := uint16(0); i < quantity; i++ {
		v, ok := m.holdingRegs[addr+i]
		if !ok {
			return nil, modbus.ErrIllegalDataAddress
		}
		vals = append(vals, v)
	}
	return vals, nil
}

func (m *mockClient) ReadCoils(addr uint16, quantity uint16) ([]bool, error) {
	vals := make([]bool, 0, quantity)
	for i := uint16(0); i < quantity; i++ {
		v, ok := m.coils[addr+i]
		if !ok {
			return nil, modbus.ErrIllegalDataAddress
		}
		vals = append(vals, v)
	}
	return vals, nil
}

// buildHoldingMap creates a register map with values at consecutive addresses.
// zeroBased=true: addresses 0..count-1; zeroBased=false: addresses 1..count.
func buildHoldingMap(count int, zeroBased bool) map[uint16]uint16 {
	m := make(map[uint16]uint16, count)
	base := uint16(0)
	if !zeroBased {
		base = 1
	}
	for i := 0; i < count; i++ {
		m[base+uint16(i)] = uint16(i + 1)
	}
	return m
}

// buildCoilMap creates a coil map for addresses 0..count-1.
func buildCoilMap(count int) map[uint16]bool {
	m := make(map[uint16]bool, count)
	for i := 0; i < count; i++ {
		m[uint16(i)] = true
	}
	return m
}

func TestEnumerateHoldingRegisters_ZeroBased_5(t *testing.T) {
	client := &mockClient{holdingRegs: buildHoldingMap(5, true)}
	count, addressing, err := enumerateHoldingRegisters(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addressing != "zero-based" {
		t.Errorf("addressing: got %q, want %q", addressing, "zero-based")
	}
	if count != 5 {
		t.Errorf("count: got %d, want 5", count)
	}
}

func TestEnumerateHoldingRegisters_OneBased_2000(t *testing.T) {
	client := &mockClient{holdingRegs: buildHoldingMap(2000, false)}
	count, addressing, err := enumerateHoldingRegisters(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addressing != "one-based" {
		t.Errorf("addressing: got %q, want %q", addressing, "one-based")
	}
	if count != 2000 {
		t.Errorf("count: got %d, want 2000", count)
	}
}

func TestEnumerateHoldingRegisters_NoRegisters(t *testing.T) {
	client := &mockClient{holdingRegs: map[uint16]uint16{}}
	count, addressing, err := enumerateHoldingRegisters(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addressing != "unknown" {
		t.Errorf("addressing: got %q, want %q", addressing, "unknown")
	}
	if count != 0 {
		t.Errorf("count: got %d, want 0", count)
	}
}

func TestEnumerateHoldingRegisters_ZeroBased_1(t *testing.T) {
	client := &mockClient{holdingRegs: buildHoldingMap(1, true)}
	count, addressing, err := enumerateHoldingRegisters(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addressing != "zero-based" {
		t.Errorf("addressing: got %q, want %q", addressing, "zero-based")
	}
	if count != 1 {
		t.Errorf("count: got %d, want 1", count)
	}
}

func TestEnumerateCoils_4Coils(t *testing.T) {
	client := &mockClient{
		holdingRegs: buildHoldingMap(5, true), // prevent addressing detection failure
		coils:       buildCoilMap(4),
	}
	count, err := enumerateCoils(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 4 {
		t.Errorf("count: got %d, want 4", count)
	}
}

func TestEnumerateCoils_NoCoils(t *testing.T) {
	client := &mockClient{coils: map[uint16]bool{}}
	count, err := enumerateCoils(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("count: got %d, want 0", count)
	}
}

func TestMeasureResponseTime_Returns(t *testing.T) {
	client := &mockClient{holdingRegs: buildHoldingMap(5, true)}
	mean, jitter, err := measureResponseTime(client, 0, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mean < 0 {
		t.Errorf("mean: got negative value %f", mean)
	}
	if jitter < 0 {
		t.Errorf("jitter: got negative value %f", jitter)
	}
}

func TestMeasureResponseTime_ZeroRegisters(t *testing.T) {
	client := &mockClient{}
	mean, jitter, err := measureResponseTime(client, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error for 0 registers: %v", err)
	}
	if mean != 0 || jitter != 0 {
		t.Errorf("expected 0,0 for empty register set; got mean=%f jitter=%f", mean, jitter)
	}
}

func TestGatewayDiscovery_CallsSetUnitId(t *testing.T) {
	mc := &mockClient{
		holdingRegs: buildHoldingMap(5, true),
		coils:       buildCoilMap(2),
	}

	cfg := &config.Config{
		PollIntervalSeconds:   2,
		GatewayRequestDelayMs: 0, // no delay in tests
		APIAddr:               ":8091",
		Environments: []config.Environment{
			{
				Name:    "test",
				Address: "127.0.0.1",
				Endpoints: []config.Endpoint{
					{
						Port:        5030,
						Gateway:     true,
						ScanUnitIDs: []uint8{1, 2, 247},
						Description: "test gateway",
					},
				},
			},
		},
	}
	inv := inventory.NewInventory()

	d := &Discoverer{
		cfg:          cfg,
		inv:          inv,
		newClient:    func(url string) (ClientInterface, error) { return mc, nil },
		gatewayDelay: 0,
	}

	ctx := context.Background()
	env := cfg.Environments[0]
	ep := env.Endpoints[0]

	if err := d.discoverGateway(ctx, env, ep); err != nil {
		t.Fatalf("discoverGateway: %v", err)
	}

	// SetUnitId should have been called for each scan ID (1, 2, 247).
	if len(mc.unitIDLog) < 3 {
		t.Errorf("SetUnitId called %d times, want at least 3", len(mc.unitIDLog))
	}

	// Verify all scan IDs were attempted.
	called := make(map[uint8]bool)
	for _, id := range mc.unitIDLog {
		called[id] = true
	}
	for _, id := range []uint8{1, 2, 247} {
		if !called[id] {
			t.Errorf("SetUnitId(%d) was never called", id)
		}
	}
}

type openFailClient struct{ mockClient }

func (o *openFailClient) Open() error { return &connRefusedError{} }

type connRefusedError struct{}

func (e *connRefusedError) Error() string { return "connection refused" }

func TestDiscoverAll_OfflineEndpoint(t *testing.T) {
	cfg := &config.Config{
		PollIntervalSeconds: 2,
		APIAddr:             ":8091",
		Environments: []config.Environment{
			{
				Name:    "test",
				Address: "127.0.0.1",
				Endpoints: []config.Endpoint{
					{Port: 5020, UnitID: 1, Description: "test plc"},
				},
			},
		},
	}
	inv := inventory.NewInventory()

	d := &Discoverer{
		cfg: cfg,
		inv: inv,
		newClient: func(url string) (ClientInterface, error) {
			return &openFailClient{}, nil
		},
		gatewayDelay: 0,
	}

	ctx := context.Background()
	if err := d.DiscoverAll(ctx); err != nil {
		t.Fatalf("DiscoverAll should not return error for offline devices: %v", err)
	}

	assets := inv.List()
	if len(assets) != 1 {
		t.Fatalf("expected 1 offline asset, got %d", len(assets))
	}
	if assets[0].Status != inventory.StatusOffline {
		t.Errorf("status: got %q, want %q", assets[0].Status, inventory.StatusOffline)
	}
}
