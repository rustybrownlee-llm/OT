package device_test

import (
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/plant/internal/config"
	"github.com/rustybrownlee/ot-simulator/plant/internal/device"
)

// makeDevice constructs a minimal config.Device for testing.
func makeDevice(addressing string, delayMS, jitterMS int, variants map[string]config.RegisterMap) *config.Device {
	return &config.Device{
		SchemaVersion: "0.1",
		Device: config.DeviceMeta{
			ID:     "test-device",
			Vendor: "Test Vendor",
			Model:  "Test Model",
		},
		Connectivity: config.Connectivity{
			ResponseDelayMS:  delayMS,
			ResponseJitterMS: jitterMS,
		},
		Registers: config.RegisterCapacity{
			MaxHolding:          256,
			MaxCoils:            256,
			MaxInputRegisters:   256,
			MaxDiscreteInputs:   256,
			Addressing:          addressing,
			FloatByteOrder:      "big-endian",
			MaxRegistersPerRead: 125,
		},
		RegisterMapVariants: variants,
	}
}

// makePlacement constructs a minimal config.Placement for testing.
func makePlacement(id, variant string) config.Placement {
	return config.Placement{
		ID:                 id,
		Device:             "test-device",
		Network:            "test-net",
		Role:               "Test Role",
		RegisterMapVariant: variant,
	}
}

func TestBuildProfile_MissingVariant(t *testing.T) {
	dev := makeDevice("zero-based", 5, 3, map[string]config.RegisterMap{
		"existing-variant": {},
	})
	p := makePlacement("plc-01", "nonexistent-variant")

	_, err := device.BuildProfile(p, dev)
	if err == nil {
		t.Fatal("expected error for missing variant, got nil")
	}
}

func TestBuildProfile_EmptyVariant(t *testing.T) {
	dev := makeDevice("zero-based", 5, 3, map[string]config.RegisterMap{})
	p := makePlacement("plc-01", "")

	profile, err := device.BuildProfile(p, dev)
	if err != nil {
		t.Fatalf("unexpected error for empty variant: %v", err)
	}
	if len(profile.HoldingRegisters) != 0 {
		t.Errorf("expected 0 holding registers, got %d", len(profile.HoldingRegisters))
	}
	if len(profile.Coils) != 0 {
		t.Errorf("expected 0 coils, got %d", len(profile.Coils))
	}
}

func TestBuildProfile_TimingAndAddressing(t *testing.T) {
	dev := makeDevice("one-based", 50, 20, map[string]config.RegisterMap{
		"test-variant": {},
	})
	p := makePlacement("plc-01", "test-variant")

	profile, err := device.BuildProfile(p, dev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Addressing != "one-based" {
		t.Errorf("addressing: got %q, want %q", profile.Addressing, "one-based")
	}
	if profile.ResponseDelay != 50*time.Millisecond {
		t.Errorf("delay: got %v, want 50ms", profile.ResponseDelay)
	}
	if profile.ResponseJitter != 20*time.Millisecond {
		t.Errorf("jitter: got %v, want 20ms", profile.ResponseJitter)
	}
}

func TestBuildProfile_InitValue_Tier1_Analog(t *testing.T) {
	analogUnits := []string{"L/s", "%", "pH", "NTU", "degC", "degF", "GPM", "PSI", "ft/min", "A", "s", "kPa", "mW/cm2", "mg/L", "mL/min"}

	for _, unit := range analogUnits {
		t.Run(unit, func(t *testing.T) {
			dev := makeDevice("zero-based", 5, 3, map[string]config.RegisterMap{
				"v": {
					Holding: []config.RegisterDef{
						{Address: 0, Name: "test_reg", Unit: unit, ScaleMin: 0, ScaleMax: 100},
					},
				},
			})
			p := makePlacement("plc-01", "v")
			profile, err := device.BuildProfile(p, dev)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if profile.HoldingRegisters[0].InitValue != 16383 {
				t.Errorf("unit %q: expected init 16383, got %d", unit, profile.HoldingRegisters[0].InitValue)
			}
		})
	}
}

func TestBuildProfile_InitValue_Tier2_EnumBitmask(t *testing.T) {
	cases := []struct {
		unit string
	}{
		{"enum"},
		{"bitmask"},
	}

	for _, tc := range cases {
		t.Run(tc.unit, func(t *testing.T) {
			dev := makeDevice("zero-based", 5, 3, map[string]config.RegisterMap{
				"v": {
					Holding: []config.RegisterDef{
						{Address: 0, Name: "status", Unit: tc.unit, ScaleMin: 0, ScaleMax: 3},
					},
				},
			})
			p := makePlacement("plc-01", "v")
			profile, err := device.BuildProfile(p, dev)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if profile.HoldingRegisters[0].InitValue != 0 {
				t.Errorf("unit %q: expected init 0, got %d", tc.unit, profile.HoldingRegisters[0].InitValue)
			}
		})
	}
}

func TestBuildProfile_InitValue_Tier3_Counters(t *testing.T) {
	cases := []struct {
		unit string
	}{
		{"count"},
		{"msgs"},
		{"units"},
	}

	for _, tc := range cases {
		t.Run(tc.unit, func(t *testing.T) {
			dev := makeDevice("zero-based", 5, 3, map[string]config.RegisterMap{
				"v": {
					Holding: []config.RegisterDef{
						{Address: 0, Name: "counter", Unit: tc.unit, ScaleMin: 0, ScaleMax: 65535},
					},
				},
			})
			p := makePlacement("plc-01", "v")
			profile, err := device.BuildProfile(p, dev)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if profile.HoldingRegisters[0].InitValue != 0 {
				t.Errorf("unit %q: expected init 0, got %d", tc.unit, profile.HoldingRegisters[0].InitValue)
			}
		})
	}
}

func TestBuildProfile_InitValue_Tier4_UptimeHours(t *testing.T) {
	dev := makeDevice("zero-based", 15, 10, map[string]config.RegisterMap{
		"v": {
			Holding: []config.RegisterDef{
				{Address: 0, Name: "uptime_hours", Unit: "hours", ScaleMin: 0, ScaleMax: 65535},
			},
		},
	})
	p := makePlacement("gw-01", "v")
	profile, err := device.BuildProfile(p, dev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.HoldingRegisters[0].InitValue != 8760 {
		t.Errorf("uptime_hours: expected init 8760, got %d", profile.HoldingRegisters[0].InitValue)
	}
}

func TestBuildProfile_InitValue_Tier4_PumpRuntimeHours(t *testing.T) {
	dev := makeDevice("one-based", 80, 50, map[string]config.RegisterMap{
		"v": {
			Holding: []config.RegisterDef{
				{Address: 7, Name: "pump_runtime_hours", Unit: "hours", ScaleMin: 0, ScaleMax: 65535},
			},
		},
	})
	p := makePlacement("plc-02", "v")
	profile, err := device.BuildProfile(p, dev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.HoldingRegisters[0].InitValue != 2400 {
		t.Errorf("pump_runtime_hours: expected init 2400, got %d", profile.HoldingRegisters[0].InitValue)
	}
}

func TestBuildProfile_InitValue_Tier4_GenericHours(t *testing.T) {
	dev := makeDevice("zero-based", 5, 3, map[string]config.RegisterMap{
		"v": {
			Holding: []config.RegisterDef{
				{Address: 0, Name: "run_time_hours", Unit: "hours", ScaleMin: 0, ScaleMax: 65535},
			},
		},
	})
	p := makePlacement("plc-01", "v")
	profile, err := device.BuildProfile(p, dev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.HoldingRegisters[0].InitValue != 0 {
		t.Errorf("generic hours: expected init 0, got %d", profile.HoldingRegisters[0].InitValue)
	}
}

func TestBuildProfile_Coils_AlwaysFalse(t *testing.T) {
	dev := makeDevice("zero-based", 5, 3, map[string]config.RegisterMap{
		"v": {
			Coils: []config.CoilDef{
				{Address: 0, Name: "pump_run", Writable: true},
				{Address: 1, Name: "alarm_active", Writable: false},
			},
		},
	})
	p := makePlacement("plc-01", "v")
	profile, err := device.BuildProfile(p, dev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range profile.Coils {
		if c.InitValue {
			t.Errorf("coil %q: expected false init value, got true", c.Name)
		}
	}
}

func TestBuildProfile_RegisterFields(t *testing.T) {
	dev := makeDevice("zero-based", 5, 3, map[string]config.RegisterMap{
		"v": {
			Holding: []config.RegisterDef{
				{
					Address:  0,
					Name:     "flow_rate",
					Unit:     "L/s",
					ScaleMin: 0,
					ScaleMax: 100,
					Writable: false,
				},
				{
					Address:  1,
					Name:     "speed_setpoint",
					Unit:     "%",
					ScaleMin: 0,
					ScaleMax: 100,
					Writable: true,
				},
			},
		},
	})
	p := makePlacement("plc-01", "v")
	profile, err := device.BuildProfile(p, dev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(profile.HoldingRegisters) != 2 {
		t.Fatalf("expected 2 holding registers, got %d", len(profile.HoldingRegisters))
	}

	r0 := profile.HoldingRegisters[0]
	if r0.Address != 0 || r0.Name != "flow_rate" || r0.Writable {
		t.Errorf("register 0 fields mismatch: %+v", r0)
	}

	r1 := profile.HoldingRegisters[1]
	if r1.Address != 1 || r1.Name != "speed_setpoint" || !r1.Writable {
		t.Errorf("register 1 fields mismatch: %+v", r1)
	}
}
