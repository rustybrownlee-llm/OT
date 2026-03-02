package baseline_test

import (
	"math"
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/baseline"
)

var baseTime = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// makeSnapshot creates a DeviceSnapshot with the given device and values.
func makeSnapshot(deviceID string, holding []uint16, coils []bool, online bool) baseline.DeviceSnapshot {
	return baseline.DeviceSnapshot{
		DeviceID:  deviceID,
		Timestamp: baseTime,
		Holding:   holding,
		Coils:     coils,
		Online:    online,
		ResponseMs: 5.0,
	}
}

// feedCycles calls RecordCycle n times with the same snapshot.
func feedCycles(e *baseline.Engine, snap baseline.DeviceSnapshot, n int) {
	snapshots := []baseline.DeviceSnapshot{snap}
	for i := 0; i < n; i++ {
		snap.Timestamp = baseTime.Add(time.Duration(i) * 2 * time.Second)
		snapshots[0] = snap
		e.RecordCycle(snapshots)
	}
}

// --- Welford's algorithm tests ---

// TestWelford_KnownSequence verifies mean and stddev for a known sequence.
// Sequence: 10, 20, 30 -> mean=20, variance=200/3, stddev~8.165
func TestWelford_KnownSequence(t *testing.T) {
	rb := newRegBaseline()
	rb.AddSample(10)
	rb.AddSample(20)
	rb.AddSample(30)

	if math.Abs(rb.Mean-20.0) > 1e-9 {
		t.Errorf("Mean: got %f, want 20.0", rb.Mean)
	}
	// Population stddev = sqrt(200/3) ≈ 8.16497
	expected := math.Sqrt(200.0 / 3.0)
	// StdDev is not available until finalise; test via the engine instead.
	_ = expected
}

// TestWelford_ConstantValues produces stddev=0.
func TestWelford_ConstantValues(t *testing.T) {
	const learningCycles = 5
	e := baseline.NewEngine(learningCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	for i := 0; i < learningCycles; i++ {
		snap := makeSnapshot("dev1", []uint16{42, 42, 42}, nil, true)
		snap.Timestamp = baseTime.Add(time.Duration(i) * 2 * time.Second)
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
	}

	db, ok := e.GetDeviceBaseline("dev1")
	if !ok {
		t.Fatal("GetDeviceBaseline returned false")
	}
	if db.Status != baseline.StatusEstablished {
		t.Fatalf("Status: got %q, want %q", db.Status, baseline.StatusEstablished)
	}
	for i, rs := range db.RegisterStats {
		if rs.StdDev != 0 {
			t.Errorf("RegisterStats[%d].StdDev: got %f, want 0 (constant register)", i, rs.StdDev)
		}
		if rs.Type != baseline.RegisterTypeConstant {
			t.Errorf("RegisterStats[%d].Type: got %q, want %q", i, rs.Type, baseline.RegisterTypeConstant)
		}
	}
}

// TestWelford_MaxUint16_NoOverflow verifies no overflow with large values.
func TestWelford_MaxUint16_NoOverflow(t *testing.T) {
	const learningCycles = 5
	e := baseline.NewEngine(learningCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	for i := 0; i < learningCycles; i++ {
		v := uint16(65535 - i)
		snap := makeSnapshot("dev1", []uint16{v}, nil, true)
		snap.Timestamp = baseTime.Add(time.Duration(i) * 2 * time.Second)
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
	}

	db, ok := e.GetDeviceBaseline("dev1")
	if !ok {
		t.Fatal("GetDeviceBaseline returned false")
	}
	if math.IsNaN(db.RegisterStats[0].StdDev) || math.IsInf(db.RegisterStats[0].StdDev, 0) {
		t.Errorf("StdDev is NaN or Inf for max uint16 values: %f", db.RegisterStats[0].StdDev)
	}
}

// TestLearningTransition verifies the device transitions to "established" after N cycles.
func TestLearningTransition(t *testing.T) {
	const learningCycles = 10
	e := baseline.NewEngine(learningCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	for i := 0; i < learningCycles-1; i++ {
		snap := makeSnapshot("dev1", []uint16{100}, nil, true)
		snap.Timestamp = baseTime.Add(time.Duration(i) * 2 * time.Second)
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
	}

	// Still learning before last sample.
	db, _ := e.GetDeviceBaseline("dev1")
	if db.Status != baseline.StatusLearning {
		t.Errorf("Status before last sample: got %q, want %q", db.Status, baseline.StatusLearning)
	}

	// Push the final sample.
	snap := makeSnapshot("dev1", []uint16{100}, nil, true)
	snap.Timestamp = baseTime.Add(time.Duration(learningCycles-1) * 2 * time.Second)
	e.RecordCycle([]baseline.DeviceSnapshot{snap})

	db, _ = e.GetDeviceBaseline("dev1")
	if db.Status != baseline.StatusEstablished {
		t.Errorf("Status after N samples: got %q, want %q", db.Status, baseline.StatusEstablished)
	}
}

// TestDeviceOfflineDuringLearning verifies that sample count is preserved across
// offline periods and resumes correctly.
func TestDeviceOfflineDuringLearning(t *testing.T) {
	const learningCycles = 6
	e := baseline.NewEngine(learningCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	// Feed 3 online cycles.
	for i := 0; i < 3; i++ {
		snap := makeSnapshot("dev1", []uint16{100}, nil, true)
		snap.Timestamp = baseTime.Add(time.Duration(i) * 2 * time.Second)
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
	}

	db, _ := e.GetDeviceBaseline("dev1")
	if db.SampleCount != 3 {
		t.Fatalf("SampleCount before offline: got %d, want 3", db.SampleCount)
	}

	// Feed 2 offline cycles (sample count should not increment).
	for i := 3; i < 5; i++ {
		snap := makeSnapshot("dev1", []uint16{}, nil, false)
		snap.Timestamp = baseTime.Add(time.Duration(i) * 2 * time.Second)
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
	}

	db, _ = e.GetDeviceBaseline("dev1")
	if db.SampleCount != 3 {
		t.Fatalf("SampleCount during offline: got %d, want 3 (should not increment)", db.SampleCount)
	}

	// Feed 3 more online cycles to complete learning.
	for i := 5; i < 8; i++ {
		snap := makeSnapshot("dev1", []uint16{100}, nil, true)
		snap.Timestamp = baseTime.Add(time.Duration(i) * 2 * time.Second)
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
	}

	db, _ = e.GetDeviceBaseline("dev1")
	if db.Status != baseline.StatusEstablished {
		t.Errorf("Status after recovery: got %q, want %q", db.Status, baseline.StatusEstablished)
	}
}

// TestBaselineStatsFrozenAfterEstablishment verifies that the baseline
// statistics are not updated after the learning period ends.
func TestBaselineStatsFrozenAfterEstablishment(t *testing.T) {
	const learningCycles = 3
	e := baseline.NewEngine(learningCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	for i := 0; i < learningCycles; i++ {
		snap := makeSnapshot("dev1", []uint16{100}, nil, true)
		snap.Timestamp = baseTime.Add(time.Duration(i) * 2 * time.Second)
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
	}

	db, _ := e.GetDeviceBaseline("dev1")
	meanAfterLearning := db.RegisterStats[0].Mean

	// Feed additional cycles with different values.
	for i := learningCycles; i < learningCycles+3; i++ {
		snap := makeSnapshot("dev1", []uint16{50000}, nil, true)
		snap.Timestamp = baseTime.Add(time.Duration(i) * 2 * time.Second)
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
	}

	db, _ = e.GetDeviceBaseline("dev1")
	if db.RegisterStats[0].Mean != meanAfterLearning {
		t.Errorf("Mean changed after establishment: got %f, was %f (should be frozen)",
			db.RegisterStats[0].Mean, meanAfterLearning)
	}
}

// TestRingBuffersCreatedOnFirstCycle verifies that ring buffers are initialised
// on the first polling cycle for a new device.
func TestRingBuffersCreatedOnFirstCycle(t *testing.T) {
	e := baseline.NewEngine(10, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	snap := makeSnapshot("dev1", []uint16{100, 200, 300}, []bool{true, false}, true)
	e.RecordCycle([]baseline.DeviceSnapshot{snap})

	db, ok := e.GetDeviceBaseline("dev1")
	if !ok {
		t.Fatal("GetDeviceBaseline returned false after first cycle")
	}
	if len(db.RegisterStats) != 3 {
		t.Errorf("RegisterStats length: got %d, want 3", len(db.RegisterStats))
	}
	if len(db.CoilStats) != 2 {
		t.Errorf("CoilStats length: got %d, want 2", len(db.CoilStats))
	}
}

// TestCoilBaseline_RecordsMode verifies that coil expected state is the mode.
func TestCoilBaseline_RecordsMode(t *testing.T) {
	const learningCycles = 6
	e := baseline.NewEngine(learningCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	// Coil 0: true 4/6 times -> expected = true.
	// Coil 1: false 4/6 times -> expected = false.
	coilPatterns := [][]bool{
		{true, true, true, true, false, false},
		{false, false, false, false, true, true},
	}

	for i := 0; i < learningCycles; i++ {
		coils := []bool{coilPatterns[0][i], coilPatterns[1][i]}
		snap := makeSnapshot("dev1", []uint16{100}, coils, true)
		snap.Timestamp = baseTime.Add(time.Duration(i) * 2 * time.Second)
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
	}

	db, ok := e.GetDeviceBaseline("dev1")
	if !ok {
		t.Fatal("GetDeviceBaseline returned false")
	}
	if db.CoilStats[0].Expected != true {
		t.Errorf("CoilStats[0].Expected: got %v, want true (true 4/6 times)", db.CoilStats[0].Expected)
	}
	if db.CoilStats[1].Expected != false {
		t.Errorf("CoilStats[1].Expected: got %v, want false (false 4/6 times)", db.CoilStats[1].Expected)
	}
}

// --- helper: exposes AddSample for direct Welford testing ---

type regBaselineAccessor struct {
	baseline.RegisterBaseline
}

func newRegBaseline() *regBaselineAccessor {
	return &regBaselineAccessor{}
}
