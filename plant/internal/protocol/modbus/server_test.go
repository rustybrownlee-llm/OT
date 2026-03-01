package modbus_test

import (
	"fmt"
	"testing"
	"time"

	mblib "github.com/simonvetter/modbus"

	"github.com/rustybrownlee/ot-simulator/plant/internal/config"
	"github.com/rustybrownlee/ot-simulator/plant/internal/logging"
	mbpkg "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

// makeMinimalResolvedEnv builds a minimal ResolvedEnvironment with one Ethernet PLC.
// Uses port 15999 to avoid conflicts with production ports.
func makeMinimalResolvedEnv(port int) *config.ResolvedEnvironment {
	env := config.Environment{
		SchemaVersion: "0.1",
		Environment: config.EnvMeta{
			ID:   "test-env",
			Name: "Test Environment",
		},
		Networks: []config.NetworkRef{
			{Ref: "test-net"},
		},
		Placements: []config.Placement{
			{
				ID:                 "test-plc-01",
				Device:             "test-plc",
				Network:            "test-net",
				IP:                 "10.0.0.1",
				ModbusPort:         port,
				Role:               "Test PLC",
				RegisterMapVariant: "test-variant",
			},
		},
	}

	devices := map[string]*config.Device{
		"test-plc": {
			Device: config.DeviceMeta{
				ID:       "test-plc",
				Category: "plc",
				Model:    "Test PLC Model",
			},
			Connectivity: config.Connectivity{
				ResponseDelayMS:  0,
				ResponseJitterMS: 0,
				ConcurrentConns:  5,
			},
			Registers: config.RegisterCapacity{
				MaxHolding:          10,
				MaxCoils:            10,
				MaxInputRegisters:   10,
				MaxDiscreteInputs:   10,
				Addressing:          "zero-based",
				FloatByteOrder:      "big-endian",
				MaxRegistersPerRead: 125,
			},
			RegisterMapVariants: map[string]config.RegisterMap{
				"test-variant": {
					Holding: []config.RegisterDef{
						{Address: 0, Name: "flow_rate", Unit: "L/s",
							ScaleMin: 0, ScaleMax: 100, Writable: false},
						{Address: 1, Name: "setpoint", Unit: "%",
							ScaleMin: 0, ScaleMax: 100, Writable: true},
					},
					Coils: []config.CoilDef{
						{Address: 0, Name: "pump_run", Writable: true},
					},
				},
			},
		},
	}

	networks := map[string]*config.Network{
		"test-net": {
			Network: config.NetworkMeta{ID: "test-net", Name: "Test Network", Type: "ethernet"},
		},
	}

	return &config.ResolvedEnvironment{Env: env, Devices: devices, Networks: networks}
}

// makeMissingVariantEnv returns an environment where the placement references a
// variant that does not exist in the device atom. This triggers FR-2 fail-loud behavior.
func makeMissingVariantEnv() *config.ResolvedEnvironment {
	env := config.Environment{
		SchemaVersion: "0.1",
		Environment:   config.EnvMeta{ID: "bad-env", Name: "Bad Environment"},
		Networks:      []config.NetworkRef{{Ref: "test-net"}},
		Placements: []config.Placement{
			{
				ID:                 "bad-plc",
				Device:             "test-plc",
				Network:            "test-net",
				IP:                 "10.0.0.1",
				ModbusPort:         15998,
				Role:               "Bad PLC",
				RegisterMapVariant: "nonexistent-variant",
			},
		},
	}

	devices := map[string]*config.Device{
		"test-plc": {
			Device: config.DeviceMeta{ID: "test-plc", Category: "plc"},
			Connectivity: config.Connectivity{
				ConcurrentConns: 5,
			},
			Registers: config.RegisterCapacity{
				MaxHolding: 10, MaxCoils: 10,
				MaxInputRegisters: 10, MaxDiscreteInputs: 10,
				Addressing: "zero-based", MaxRegistersPerRead: 125,
			},
			RegisterMapVariants: map[string]config.RegisterMap{
				"other-variant": {},
			},
		},
	}

	networks := map[string]*config.Network{
		"test-net": {
			Network: config.NetworkMeta{ID: "test-net", Name: "Test Network", Type: "ethernet"},
		},
	}

	return &config.ResolvedEnvironment{Env: env, Devices: devices, Networks: networks}
}

// makeGatewayEnv builds an environment with a gateway and two serial placements.
// Uses port 15997 for the gateway listener.
func makeGatewayEnv() *config.ResolvedEnvironment {
	env := config.Environment{
		SchemaVersion: "0.1",
		Environment:   config.EnvMeta{ID: "gw-env", Name: "Gateway Test Environment"},
		Networks: []config.NetworkRef{
			{Ref: "eth-net"},
			{Ref: "serial-bus"},
		},
		Placements: []config.Placement{
			{
				ID:                 "test-gateway",
				Device:             "test-gw",
				Network:            "eth-net",
				IP:                 "10.0.0.20",
				ModbusPort:         15997,
				Role:               "Serial Gateway",
				RegisterMapVariant: "gw-status",
			},
			{
				ID:            "serial-plc-01",
				Device:        "serial-plc",
				Network:       "serial-bus",
				SerialAddress: 1,
				Gateway:       "test-gateway",
				Role:          "Serial PLC 1",
				RegisterMapVariant: "plc-variant",
			},
		},
	}

	devices := map[string]*config.Device{
		"test-gw": {
			Device: config.DeviceMeta{ID: "test-gw", Category: "gateway", Model: "Test GW"},
			Connectivity: config.Connectivity{
				ResponseDelayMS: 0, ResponseJitterMS: 0, ConcurrentConns: 4,
			},
			Registers: config.RegisterCapacity{
				MaxHolding: 5, MaxCoils: 0,
				MaxInputRegisters: 0, MaxDiscreteInputs: 0,
				Addressing: "zero-based", MaxRegistersPerRead: 125,
			},
			RegisterMapVariants: map[string]config.RegisterMap{
				"gw-status": {
					Holding: []config.RegisterDef{
						{Address: 0, Name: "uptime_hours", Unit: "hours",
							ScaleMin: 0, ScaleMax: 65535, Writable: false},
					},
				},
			},
		},
		"serial-plc": {
			Device: config.DeviceMeta{ID: "serial-plc", Category: "plc", Model: "Serial PLC"},
			Connectivity: config.Connectivity{
				ResponseDelayMS: 0, ResponseJitterMS: 0, ConcurrentConns: 1,
			},
			Registers: config.RegisterCapacity{
				MaxHolding: 10, MaxCoils: 5,
				MaxInputRegisters: 5, MaxDiscreteInputs: 5,
				Addressing: "one-based", MaxRegistersPerRead: 125,
			},
			RegisterMapVariants: map[string]config.RegisterMap{
				"plc-variant": {
					Holding: []config.RegisterDef{
						{Address: 1, Name: "flow", Unit: "L/s",
							ScaleMin: 0, ScaleMax: 100, Writable: false},
					},
					Coils: []config.CoilDef{
						{Address: 1, Name: "run", Writable: true},
					},
				},
			},
		},
	}

	networks := map[string]*config.Network{
		"eth-net": {
			Network: config.NetworkMeta{ID: "eth-net", Name: "Ethernet", Type: "ethernet"},
		},
		"serial-bus": {
			Network: config.NetworkMeta{ID: "serial-bus", Name: "Serial Bus", Type: "serial"},
		},
	}

	return &config.ResolvedEnvironment{Env: env, Devices: devices, Networks: networks}
}

func TestNewServerManager_Creation(t *testing.T) {
	resolved := makeMinimalResolvedEnv(15999)
	sm, err := mbpkg.NewServerManager(resolved, logging.NewTestLogger())
	if err != nil {
		t.Fatalf("NewServerManager failed: %v", err)
	}
	if sm == nil {
		t.Fatal("expected non-nil ServerManager")
	}
}

func TestNewServerManager_MissingVariant_ReturnsError(t *testing.T) {
	resolved := makeMissingVariantEnv()
	_, err := mbpkg.NewServerManager(resolved, logging.NewTestLogger())
	if err == nil {
		t.Fatal("expected error for missing variant, got nil")
	}
}

func TestServerManager_StartStop_Lifecycle(t *testing.T) {
	resolved := makeMinimalResolvedEnv(15996)
	sm, err := mbpkg.NewServerManager(resolved, logging.NewTestLogger())
	if err != nil {
		t.Fatalf("NewServerManager: %v", err)
	}

	if err := sm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Allow the listener to settle.
	time.Sleep(50 * time.Millisecond)

	sm.Stop()
}

func TestServerManager_GatewayEnv_Creation(t *testing.T) {
	resolved := makeGatewayEnv()
	sm, err := mbpkg.NewServerManager(resolved, logging.NewTestLogger())
	if err != nil {
		t.Fatalf("NewServerManager with gateway env failed: %v", err)
	}
	if sm == nil {
		t.Fatal("expected non-nil ServerManager")
	}
}

// TestServerManager_Integration starts a real Modbus TCP server on an ephemeral test port,
// connects a client, reads holding registers, and verifies the midpoint initialization value.
func TestServerManager_Integration(t *testing.T) {
	const testPort = 15994
	resolved := makeMinimalResolvedEnv(testPort)

	sm, err := mbpkg.NewServerManager(resolved, logging.NewTestLogger())
	if err != nil {
		t.Fatalf("NewServerManager: %v", err)
	}

	if err := sm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sm.Stop()

	// Allow the listener to settle before connecting.
	time.Sleep(100 * time.Millisecond)

	client, err := mblib.NewClient(&mblib.ClientConfiguration{
		URL:     fmt.Sprintf("tcp://localhost:%d", testPort),
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Open(); err != nil {
		t.Fatalf("client.Open: %v", err)
	}
	defer client.Close()

	// Read 2 holding registers starting at address 0 (flow_rate and setpoint).
	vals, err := client.ReadRegisters(0, 2, mblib.HOLDING_REGISTER)
	if err != nil {
		t.Fatalf("ReadRegisters: %v", err)
	}

	if len(vals) != 2 {
		t.Fatalf("expected 2 values, got %d", len(vals))
	}

	// Both are analog ("L/s" and "%") -- tier 1 init = 16383.
	for i, v := range vals {
		if v != 16383 {
			t.Errorf("register %d: expected midpoint 16383, got %d", i, v)
		}
	}
}

// TestServerManager_Integration_WriteReadOnly verifies ErrIllegalFunction on write to read-only register.
func TestServerManager_Integration_WriteReadOnly(t *testing.T) {
	const testPort = 15993
	resolved := makeMinimalResolvedEnv(testPort)

	sm, err := mbpkg.NewServerManager(resolved, logging.NewTestLogger())
	if err != nil {
		t.Fatalf("NewServerManager: %v", err)
	}
	if err := sm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	client, err := mblib.NewClient(&mblib.ClientConfiguration{
		URL:     fmt.Sprintf("tcp://localhost:%d", testPort),
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Open(); err != nil {
		t.Fatalf("client.Open: %v", err)
	}
	defer client.Close()

	// Address 0 is flow_rate (writable: false) -- must return ErrIllegalFunction.
	err = client.WriteRegister(0, 9999)
	if err == nil {
		t.Fatal("expected error writing read-only register, got nil")
	}
	if err != mblib.ErrIllegalFunction {
		t.Errorf("expected ErrIllegalFunction (0x01), got: %v", err)
	}
}

// TestServerManager_Integration_GatewayUnknownUnitID verifies ErrGWPathUnavailable for unknown unit IDs.
func TestServerManager_Integration_GatewayUnknownUnitID(t *testing.T) {
	resolved := makeGatewayEnv()

	sm, err := mbpkg.NewServerManager(resolved, logging.NewTestLogger())
	if err != nil {
		t.Fatalf("NewServerManager: %v", err)
	}
	if err := sm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	client, err := mblib.NewClient(&mblib.ClientConfiguration{
		URL:     "tcp://localhost:15997",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Open(); err != nil {
		t.Fatalf("client.Open: %v", err)
	}
	defer client.Close()

	if err := client.SetUnitId(99); err != nil {
		t.Fatalf("SetUnitId: %v", err)
	}
	_, err = client.ReadRegisters(1, 1, mblib.HOLDING_REGISTER)
	if err == nil {
		t.Fatal("expected ErrGWPathUnavailable for unknown unit ID, got nil")
	}
	if err != mblib.ErrGWPathUnavailable {
		t.Errorf("expected ErrGWPathUnavailable (0x0A), got: %v", err)
	}
}
