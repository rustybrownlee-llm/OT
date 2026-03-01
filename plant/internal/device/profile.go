// Package device loads and validates device profiles from YAML configuration.
// A device profile defines the observable behavior of a specific PLC model:
// communication parameters, register map limits, response timing, diagnostic
// capabilities, and network discoverability.
//
// Per ADR-004, vendor-specific behavior is applied as a configuration overlay
// rather than code. This allows adding new device types by creating YAML
// profile files under plant/config/devices/ without modifying the PLC engine.
//
// The four initial profiles (CompactLogix L33ER, SLC-500/05, Modicon 984,
// Moxa NPort 5150) are defined in SOW-001.0 and SOW-004.0.
//
// Implemented in SOW-001.0.
package device

import (
	"fmt"
	"strings"
	"time"

	"github.com/rustybrownlee/ot-simulator/plant/internal/config"
)

// DeviceProfile extracts the runtime characteristics needed by the Modbus server
// from a resolved device atom and placement.
type DeviceProfile struct {
	PlacementID         string
	DeviceID            string
	Role                string
	ResponseDelay       time.Duration
	ResponseJitter      time.Duration
	Addressing          string // "zero-based" or "one-based"
	FloatByteOrder      string // "big-endian" or "little-endian"
	MaxRegistersPerRead int
	HoldingRegisters    []RegisterEntry
	Coils               []CoilEntry
	MaxHolding          int
	MaxCoils            int
	MaxInputRegisters   int
	MaxDiscreteInputs   int
}

// RegisterEntry is a runtime-ready register definition.
type RegisterEntry struct {
	Address   int
	Name      string
	Unit      string
	ScaleMin  float64
	ScaleMax  float64
	Writable  bool
	InitValue uint16 // computed per tiered InitValue rules
}

// CoilEntry is a runtime-ready coil definition.
type CoilEntry struct {
	Address   int
	Name      string
	Writable  bool
	InitValue bool // false for both command and alarm coils (safe startup state)
}

// BuildProfile constructs a DeviceProfile from a placement and its resolved device atom.
// It selects the named variant from the device atom's RegisterMapVariants.
// Returns an error if the variant does not exist in the device atom.
func BuildProfile(placement config.Placement, device *config.Device) (*DeviceProfile, error) {
	variant, err := selectVariant(placement, device)
	if err != nil {
		return nil, err
	}

	holding := buildHoldingEntries(variant.Holding)
	coils := buildCoilEntries(variant.Coils)

	return &DeviceProfile{
		PlacementID:         placement.ID,
		DeviceID:            device.Device.ID,
		Role:                placement.Role,
		ResponseDelay:       time.Duration(device.Connectivity.ResponseDelayMS) * time.Millisecond,
		ResponseJitter:      time.Duration(device.Connectivity.ResponseJitterMS) * time.Millisecond,
		Addressing:          device.Registers.Addressing,
		FloatByteOrder:      device.Registers.FloatByteOrder,
		MaxRegistersPerRead: device.Registers.MaxRegistersPerRead,
		HoldingRegisters:    holding,
		Coils:               coils,
		MaxHolding:          device.Registers.MaxHolding,
		MaxCoils:            device.Registers.MaxCoils,
		MaxInputRegisters:   device.Registers.MaxInputRegisters,
		MaxDiscreteInputs:   device.Registers.MaxDiscreteInputs,
	}, nil
}

// selectVariant finds the named variant in the device atom.
// An empty variant name is valid -- it results in an empty RegisterMap.
// A non-empty variant name that does not exist in the device atom returns an error.
func selectVariant(placement config.Placement, device *config.Device) (config.RegisterMap, error) {
	if placement.RegisterMapVariant == "" {
		return config.RegisterMap{}, nil
	}

	variant, ok := device.RegisterMapVariants[placement.RegisterMapVariant]
	if !ok {
		return config.RegisterMap{}, fmt.Errorf(
			"placement %q: variant %q not found in device %q",
			placement.ID, placement.RegisterMapVariant, device.Device.ID,
		)
	}

	return variant, nil
}

// buildHoldingEntries converts RegisterDef slices to RegisterEntry slices with computed InitValues.
func buildHoldingEntries(defs []config.RegisterDef) []RegisterEntry {
	entries := make([]RegisterEntry, len(defs))
	for i, d := range defs {
		entries[i] = RegisterEntry{
			Address:   d.Address,
			Name:      d.Name,
			Unit:      d.Unit,
			ScaleMin:  d.ScaleMin,
			ScaleMax:  d.ScaleMax,
			Writable:  d.Writable,
			InitValue: computeInitValue(d),
		}
	}
	return entries
}

// buildCoilEntries converts CoilDef slices to CoilEntry slices.
// All coils initialize to false: command coils for safe startup, alarm coils for no-alarm idle.
func buildCoilEntries(defs []config.CoilDef) []CoilEntry {
	entries := make([]CoilEntry, len(defs))
	for i, d := range defs {
		entries[i] = CoilEntry{
			Address:   d.Address,
			Name:      d.Name,
			Writable:  d.Writable,
			InitValue: false,
		}
	}
	return entries
}

// computeInitValue applies the four-tier initialization rule from SOW-002.0 Section 5.
//
// Tier 1 -- Analog process values: initValue = 16383 (midpoint of 0-32767)
// Tier 2 -- Enum and bitmask registers: initValue = 0
// Tier 3 -- Event counters: initValue = 0
// Tier 4 -- Runtime/uptime registers: initValue depends on name (see below)
func computeInitValue(d config.RegisterDef) uint16 {
	unit := strings.ToLower(strings.TrimSpace(d.Unit))

	switch unit {
	case "enum", "bitmask":
		// Tier 2: first enum value or no bits set.
		return 0

	case "count", "msgs", "units":
		// Tier 3: no events since simulation start.
		return 0

	case "hours":
		// Tier 4: runtime/uptime registers.
		return computeHoursInitValue(d.Name)
	}

	// Tier 1: analog process value -- midpoint of 16-bit scaled range.
	return 16383
}

// computeHoursInitValue determines the initial value for "hours" unit registers.
// Registers with "uptime" in the name initialize to 8760 (one year of operation).
// Registers whose description indicates power-cycle reset (pump_runtime_hours) initialize to 2400.
// All other "hours" registers initialize to 0.
func computeHoursInitValue(name string) uint16 {
	lower := strings.ToLower(name)

	if strings.Contains(lower, "uptime") {
		// Device has been running for one year continuously.
		return 8760
	}

	if lower == "pump_runtime_hours" {
		// PROTOTYPE-DEBT: [td-profile-001] Plausible non-zero value since last power cycle (~100 days).
		// TODO-FUTURE: Drive this from device atom description field when power-cycle semantics are formalized.
		return 2400
	}

	return 0
}
