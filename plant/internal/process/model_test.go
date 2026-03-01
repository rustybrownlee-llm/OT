package process

import (
	"math"
	"testing"
)

// TestToRaw_ScalingBoundaries verifies raw conversion at scale endpoints.
func TestToRaw_ScalingBoundaries(t *testing.T) {
	cases := []struct {
		name     string
		value    float64
		min, max float64
		want     uint16
	}{
		{"at min gives 0", 0, 0, 100, 0},
		{"at max gives 32767", 100, 0, 100, 32767},
		{"midpoint", 50, 0, 100, 16383},
		{"below min clamps to 0", -5, 0, 100, 0},
		{"above max clamps to 32767", 105, 0, 100, 32767},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toRaw(tc.value, tc.min, tc.max)
			if got != tc.want {
				t.Errorf("toRaw(%v, %v, %v) = %d, want %d", tc.value, tc.min, tc.max, got, tc.want)
			}
		})
	}
}

// TestToEng_ScalingBoundaries verifies engineering-unit conversion at raw endpoints.
func TestToEng_ScalingBoundaries(t *testing.T) {
	const epsilon = 1e-6
	const scaleMin = 0.0
	const scaleMax = 100.0

	cases := []struct {
		name string
		raw  uint16
		want float64
	}{
		{"raw 0 gives scaleMin", 0, scaleMin},
		{"raw 32767 gives scaleMax", 32767, scaleMax},
		{"raw midpoint", 16383, scaleMin + float64(16383)/32767.0*(scaleMax-scaleMin)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toEng(tc.raw, scaleMin, scaleMax)
			if math.Abs(got-tc.want) > epsilon {
				t.Errorf("toEng(%d, %v, %v) = %f, want %f", tc.raw, scaleMin, scaleMax, got, tc.want)
			}
		})
	}
}

// TestToRaw_ToEng_RoundTrip verifies that converting to raw and back recovers the value
// within the quantization error of a 15-bit scale (max error = range/32767).
func TestToRaw_ToEng_RoundTrip(t *testing.T) {
	cases := []struct {
		value, min, max float64
	}{
		{7.2, 0, 14},    // pH
		{35, 0, 100},    // NTU turbidity
		{18, 0, 40},     // degC
		{65.0, 40, 120}, // degF setpoint
		{150, 0, 500},   // GPM flow
	}

	for _, tc := range cases {
		raw := toRaw(tc.value, tc.min, tc.max)
		got := toEng(raw, tc.min, tc.max)
		maxErr := (tc.max - tc.min) / 32767.0
		if diff := math.Abs(got - tc.value); diff > maxErr {
			t.Errorf("round-trip %v [%v,%v]: got %v, diff %v > maxErr %v",
				tc.value, tc.min, tc.max, got, diff, maxErr)
		}
	}
}

// TestAddNoise_StaysWithinBounds verifies noise stays within ±0.5% of range over many samples.
func TestAddNoise_StaysWithinBounds(t *testing.T) {
	const scaleRange = 100.0
	const center = 50.0
	const tolerance = scaleRange * 0.005 * 1.001 // allow tiny floating-point margin

	for i := 0; i < 10000; i++ {
		got := addNoise(center, scaleRange)
		if math.Abs(got-center) > tolerance {
			t.Errorf("addNoise deviation %v exceeds ±0.5%% at iteration %d", got-center, i)
			break
		}
	}
}

// TestRamp_ConvergesInExpectedTicks verifies pump ramp reaches target within 5 ticks.
func TestRamp_ConvergesInExpectedTicks(t *testing.T) {
	const step = 0.2 // 20% per tick = 5 ticks to full ramp
	current := 0.0
	target := 1.0

	for tick := 1; tick <= 5; tick++ {
		current = ramp(current, target, step)
	}

	if math.Abs(current-target) > 1e-9 {
		t.Errorf("ramp did not converge to %v in 5 ticks: got %v", target, current)
	}
}

// TestRamp_DecelerationConverges verifies ramp decreases toward 0 in 5 ticks.
func TestRamp_DecelerationConverges(t *testing.T) {
	const step = 0.2
	current := 1.0
	target := 0.0

	for tick := 1; tick <= 5; tick++ {
		current = ramp(current, target, step)
	}

	if math.Abs(current-target) > 1e-9 {
		t.Errorf("ramp did not decelerate to %v in 5 ticks: got %v", target, current)
	}
}

// TestRamp_DoesNotOvershoot verifies ramp never exceeds its target.
func TestRamp_DoesNotOvershoot(t *testing.T) {
	current := 0.0
	target := 0.5

	for tick := 1; tick <= 10; tick++ {
		current = ramp(current, target, 0.2)
		if current > target+1e-9 {
			t.Errorf("tick %d: ramp overshot target %v, got %v", tick, target, current)
			break
		}
	}
}

// TestClamp_ConstrainsToRange verifies clamping at boundaries and within range.
func TestClamp_ConstrainsToRange(t *testing.T) {
	cases := []struct {
		value, min, max, want float64
	}{
		{5.0, 0, 10, 5.0},
		{-1.0, 0, 10, 0.0},
		{11.0, 0, 10, 10.0},
		{0.0, 0, 10, 0.0},
		{10.0, 0, 10, 10.0},
	}
	for _, tc := range cases {
		got := clamp(tc.value, tc.min, tc.max)
		if got != tc.want {
			t.Errorf("clamp(%v, %v, %v) = %v, want %v", tc.value, tc.min, tc.max, got, tc.want)
		}
	}
}

// TestIsNoisyUnit_AnalogUnitsReceiveNoise verifies analog process units are eligible for noise.
func TestIsNoisyUnit_AnalogUnitsReceiveNoise(t *testing.T) {
	noisyOnes := []string{"L/s", "%", "pH", "NTU", "degC", "degF", "GPM", "PSI",
		"ft/min", "A", "s", "mW/cm2", "mL/min", "mg/L", "kPa"}
	for _, u := range noisyOnes {
		if !isNoisyUnit(u) {
			t.Errorf("unit %q should be noisy but isNoisyUnit returned false", u)
		}
	}
}

// TestIsNoisyUnit_CounterAndEnumUnitsExcluded verifies non-analog units are excluded.
func TestIsNoisyUnit_CounterAndEnumUnitsExcluded(t *testing.T) {
	excluded := []string{"enum", "bitmask", "count", "msgs", "units", "hours"}
	for _, u := range excluded {
		if isNoisyUnit(u) {
			t.Errorf("unit %q should not receive noise but isNoisyUnit returned true", u)
		}
	}
}
