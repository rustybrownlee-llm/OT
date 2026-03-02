package process

import (
	"math/rand"
	"strings"
)

// ProcessModel updates the registers for one device placement on each simulation tick.
// Each model holds a reference to its RegisterStore and maintains internal state such
// as ramp positions and decay accumulators.
type ProcessModel interface {
	// Name returns a human-readable label for logging and diagnostics.
	Name() string
	// Tick advances the model by one simulation step (called every 1 second).
	Tick()
}

// noisyUnits lists unit strings for which addNoise applies random fluctuation.
// All other unit types (enum, bitmask, counters, accumulators) are excluded per FR-4.
// [OT-REVIEW] Noise MUST NOT be applied to enum, bitmask, count, msgs, hours, or units registers.
// [OT-REVIEW] MCF (volume accumulator) excluded -- volume is counter-like; noise is applied
// indirectly via AGA-3 input variation (DP, pressure, temperature). NFR-4.
var noisyUnits = map[string]bool{
	// Water treatment / manufacturing units (SOW-003.0)
	"l/s":    true,
	"%":      true,
	"ph":     true,
	"ntu":    true,
	"degc":   true,
	"degf":   true,
	"gpm":    true,
	"psi":    true,
	"ft/min": true,
	"a":      true,
	"s":      true,
	"mw/cm2": true,
	"ml/min": true,
	"mg/l":   true,
	"kpa":    true,
	// Pipeline units (SOW-009.0)
	"mscfh":    true, // thousand standard cubic feet per hour (flow rate)
	"psig":     true, // pounds per square inch gauge (pressure)
	"inh2o":    true, // inches of water column (differential pressure)
	"rpm":      true, // revolutions per minute (compressor speed)
	"mils":     true, // thousandths of an inch (vibration amplitude)
	"btu/scf":  true, // British thermal units per standard cubic foot (heating value)
	"sg":       true, // specific gravity (dimensionless ratio)
	"lb/mmscf": true, // pounds per million standard cubic feet (moisture content)
}

// isNoisyUnit reports whether the register unit type should receive sensor noise.
// Returns false for enum, bitmask, count, msgs, units, and hours registers.
func isNoisyUnit(unit string) bool {
	return noisyUnits[strings.ToLower(strings.TrimSpace(unit))]
}

// toRaw converts an engineering-unit value to a raw 16-bit Modbus register value.
// Scaling convention: raw 0 = scaleMin, raw 32767 = scaleMax (established in SOW-002.0).
// The result is clamped to [0, 32767].
func toRaw(value, scaleMin, scaleMax float64) uint16 {
	if scaleMax <= scaleMin {
		return 0
	}
	raw := (value - scaleMin) / (scaleMax - scaleMin) * 32767.0
	if raw < 0 {
		raw = 0
	}
	if raw > 32767 {
		raw = 32767
	}
	return uint16(raw)
}

// toEng converts a raw 16-bit Modbus register value to an engineering-unit value.
// Inverse of toRaw: raw 0 = scaleMin, raw 32767 = scaleMax.
func toEng(raw uint16, scaleMin, scaleMax float64) float64 {
	return scaleMin + float64(raw)/32767.0*(scaleMax-scaleMin)
}

// addNoise adds random sensor noise to value. The noise magnitude is ±0.5% of scaleRange,
// producing plausible fluctuation without obscuring the signal. For a 100-unit range,
// noise is ±0.5 units per tick.
func addNoise(value, scaleRange float64) float64 {
	magnitude := scaleRange * 0.005
	return value + (rand.Float64()*2-1)*magnitude //nolint:gosec -- deterministic security not required for simulation noise
}

// ramp moves current toward target by at most step per call.
// Returns the new value (at target if within one step).
func ramp(current, target, step float64) float64 {
	if current < target {
		current += step
		if current > target {
			current = target
		}
	} else if current > target {
		current -= step
		if current < target {
			current = target
		}
	}
	return current
}

// clamp constrains value to [min, max].
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// drift moves current toward center by rate per tick, modeling slow environmental drift.
// The value will converge to center over time without overshooting.
func drift(current, center, rate float64) float64 {
	return ramp(current, center, rate)
}
