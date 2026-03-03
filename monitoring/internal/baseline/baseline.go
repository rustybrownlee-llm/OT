// Package baseline implements behavioral baseline learning and anomaly detection
// for the OT simulator monitoring module.
//
// The baseline engine tracks per-device, per-register statistical baselines
// (mean, standard deviation, min, max) using Welford's online algorithm.
// After a configurable learning period, the engine transitions each device's
// baseline from "learning" to "established" and begins evaluating anomaly rules.
//
// All state is in-memory. On restart, baselines re-enter the learning period.
// See PROTOTYPE-DEBT td-021 in the alert package.
package baseline

import (
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/alert"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/timeseries"
)

// StatusLearning and StatusEstablished are the two baseline lifecycle states.
const (
	StatusLearning    = "learning"
	StatusEstablished = "established"
)

// RegisterType classifies register behavior observed during the learning period.
const (
	RegisterTypeConstant = "constant"
	RegisterTypeAnalog   = "analog"
	RegisterTypeCounter  = "counter"
)

// DeviceSnapshot represents one polling cycle's results for one device.
// It includes the current register values, the previous cycle's values
// (for change detection), and the response time for the cycle.
type DeviceSnapshot struct {
	DeviceID    string
	Timestamp   time.Time
	Holding     []uint16
	Coils       []bool
	ResponseMs  float64
	Online      bool
	PrevHolding []uint16 // previous cycle's holding register values
	PrevCoils   []bool   // previous cycle's coil states
}

// RegisterBaseline holds running statistics for one holding register.
// Statistics are computed via Welford's online algorithm and frozen at
// the end of the learning period.
//
// PROTOTYPE-DEBT: [td-baseline-029] Fixed baseline after learning period.
// In real OT monitoring tools baselines are periodically refreshed or use
// sliding windows. See SOW-013.0 Section 11.
type RegisterBaseline struct {
	Mean   float64
	StdDev float64
	Min    uint16
	Max    uint16
	Type   string // RegisterTypeConstant, RegisterTypeAnalog, or RegisterTypeCounter

	// Welford's algorithm internal state (active during learning only).
	m2    float64
	count int
	first uint16 // first observed value (for counter direction check)
}

// AddSample updates the running statistics with a new uint16 sample.
// Should only be called during the learning period.
func (rb *RegisterBaseline) AddSample(v uint16) {
	rb.count++
	fv := float64(v)

	if rb.count == 1 {
		rb.Mean = fv
		rb.Min = v
		rb.Max = v
		rb.first = v
		return
	}

	delta := fv - rb.Mean
	rb.Mean += delta / float64(rb.count)
	delta2 := fv - rb.Mean
	rb.m2 += delta * delta2

	if v < rb.Min {
		rb.Min = v
	}
	if v > rb.Max {
		rb.Max = v
	}
}

// finalise computes StdDev from accumulated M2 and classifies the register type.
// Must be called exactly once at the end of the learning period.
func (rb *RegisterBaseline) finalise(samples []uint16) {
	if rb.count > 1 {
		rb.StdDev = math.Sqrt(rb.m2 / float64(rb.count))
	}
	rb.Type = classifyRegister(samples, rb.StdDev)
}

// classifyRegister determines whether a register behaves as constant, analog,
// or counter based on the full sample sequence observed during learning.
//
// Classification rules (FR-13):
//   - Constant: StdDev == 0 (all values identical).
//   - Counter: >=90% of consecutive deltas are non-negative AND the final
//     value exceeds the first value.
//   - Analog: everything else.
//
// PROTOTYPE-DEBT: [td-baseline-030] Heuristic may misclassify registers that
// happen to be monotonically increasing during the learning window (e.g., a
// temperature rising during startup). See SOW-013.0 Section 11.
func classifyRegister(samples []uint16, stddev float64) string {
	if stddev == 0 {
		return RegisterTypeConstant
	}
	if len(samples) < 2 {
		return RegisterTypeAnalog
	}

	nonNegCount := 0
	total := len(samples) - 1

	for i := 1; i < len(samples); i++ {
		// Unsigned subtraction: cast to int to detect wrapping.
		delta := int(samples[i]) - int(samples[i-1])
		if delta >= 0 {
			nonNegCount++
		}
	}

	pct := float64(nonNegCount) / float64(total)
	last := samples[len(samples)-1]
	first := samples[0]

	if pct >= 0.90 && int(last) > int(first) {
		return RegisterTypeCounter
	}
	return RegisterTypeAnalog
}

// CoilLearning tracks coil state statistics during the learning period.
type CoilLearning struct {
	TrueCount   int
	FalseCount  int
	ToggleCount int     // number of state transitions during learning
	Expected    bool    // mode: true if TrueCount >= FalseCount
	MaxToggleFrequency float64 // max toggles per polling window during learning
}

// EventBaseline holds protocol-metadata baselines learned from TransactionEvents.
// Populated during the learning period, frozen at transition to established.
type EventBaseline struct {
	ObservedFCs        map[uint8]bool   // function codes seen during learning
	ObservedSrcs       map[string]bool  // source addresses seen during learning
	IsWriteTarget      bool             // true if any write FC was observed during learning
	PollIntervals      []time.Duration  // intervals between consecutive events (nil after finalization)
	PollIntervalMean   time.Duration
	PollIntervalStdDev time.Duration
	LastEventTime      time.Time        // timestamp of most recent event (for interval calc)
}

// DeviceBaseline holds the baseline state for one device.
type DeviceBaseline struct {
	DeviceID        string
	Status          string
	SampleCount     int
	RequiredSamples int
	RegisterStats   []RegisterBaseline
	CoilStats       []CoilLearning
	// Response time statistics.
	ResponseTimeMean    float64
	ResponseTimeStdDev  float64
	responseTimeM2      float64
	responseTimeSamples int
	// Event-driven protocol metadata baseline (SOW-032.0).
	Event EventBaseline
}

// Engine orchestrates baseline learning and anomaly detection across all devices.
//
// RecordCycle is the primary entry point, called synchronously by the poller
// after each polling cycle. It stores ring buffer samples, updates running
// statistics, and evaluates anomaly rules once baselines are established.
type Engine struct {
	mu              sync.RWMutex
	baselines       map[string]*DeviceBaseline
	alertStore      *alert.Store
	learningCycles  int
	bufferSize      int
	registerBuffers map[string][]*timeseries.RingBuffer
	coilBuffers     map[string][]*timeseries.CoilRingBuffer
	// sampleAccum tracks raw samples during learning for register classification.
	sampleAccum     map[string][][]uint16 // deviceID -> register index -> samples
	knownDeviceIDs  map[string]bool
}

// NewEngine creates a baseline Engine with the given learning period and buffer size.
// The engine creates its own alert store with the given capacity.
func NewEngine(learningCycles, bufferSize, maxAlerts int) *Engine {
	return &Engine{
		baselines:       make(map[string]*DeviceBaseline),
		alertStore:      alert.NewStore(maxAlerts),
		learningCycles:  learningCycles,
		bufferSize:      bufferSize,
		registerBuffers: make(map[string][]*timeseries.RingBuffer),
		coilBuffers:     make(map[string][]*timeseries.CoilRingBuffer),
		sampleAccum:     make(map[string][][]uint16),
		knownDeviceIDs:  make(map[string]bool),
	}
}

// AlertStore returns the alert store for use by the API handlers.
func (e *Engine) AlertStore() *alert.Store {
	return e.alertStore
}

// SetKnownDevices records the set of device IDs present at the end of initial
// discovery. The "new device" rule fires for any device not in this set.
// Must be called after discovery completes and before the polling loop starts.
func (e *Engine) SetKnownDevices(deviceIDs []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, id := range deviceIDs {
		e.knownDeviceIDs[id] = true
	}
}

// RecordCycle processes one polling cycle. It is called synchronously by the
// poller after each cycle completes. Processing order per device:
//  1. Initialise ring buffers on first observation.
//  2. Push values into ring buffers.
//  3. If learning: update Welford accumulators.
//  4. If sample threshold reached: transition to "established".
//  5. If established: evaluate anomaly rules.
//  6. Always: evaluate device_offline and new_device rules.
func (e *Engine) RecordCycle(snapshots []DeviceSnapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range snapshots {
		snap := &snapshots[i]
		e.processDevice(snap)
	}
}

// processDevice handles one device for one polling cycle.
// Caller must hold the write lock.
func (e *Engine) processDevice(snap *DeviceSnapshot) {
	// Always evaluate device-level rules (offline, new device).
	evaluateDeviceOffline(snap, e.getOrCreateBaseline(snap), e.alertStore)
	evaluateNewDevice(snap, e.knownDeviceIDs, e.alertStore)

	if !snap.Online {
		return
	}

	db := e.getOrCreateBaseline(snap)
	e.initBuffers(snap, db)
	e.pushToBuffers(snap)

	if db.Status == StatusLearning {
		e.updateLearning(snap, db)
		if db.SampleCount >= db.RequiredSamples {
			e.finaliseBaseline(snap.DeviceID, db)
		}
		return
	}

	// Baseline established: evaluate anomaly rules.
	evaluateValueOutOfRange(snap, db, e.alertStore)
	evaluateUnexpectedWrite(snap, db, e.alertStore)
	evaluateResponseTimeAnomaly(snap, db, e.alertStore)
}

// getOrCreateBaseline returns the DeviceBaseline for the given device,
// creating it if it does not exist. Caller must hold the write lock.
func (e *Engine) getOrCreateBaseline(snap *DeviceSnapshot) *DeviceBaseline {
	db, ok := e.baselines[snap.DeviceID]
	if !ok {
		db = &DeviceBaseline{
			DeviceID:        snap.DeviceID,
			Status:          StatusLearning,
			RequiredSamples: e.learningCycles,
			Event: EventBaseline{
				ObservedFCs:  make(map[uint8]bool),
				ObservedSrcs: make(map[string]bool),
			},
		}
		e.baselines[snap.DeviceID] = db
	}
	return db
}

// initBuffers creates ring buffers for a device on first observation.
// Caller must hold the write lock.
func (e *Engine) initBuffers(snap *DeviceSnapshot, db *DeviceBaseline) {
	if _, ok := e.registerBuffers[snap.DeviceID]; ok {
		return // already initialised
	}

	regBufs := make([]*timeseries.RingBuffer, len(snap.Holding))
	for i := range regBufs {
		regBufs[i] = timeseries.NewRingBuffer(e.bufferSize)
	}
	e.registerBuffers[snap.DeviceID] = regBufs

	coilBufs := make([]*timeseries.CoilRingBuffer, len(snap.Coils))
	for i := range coilBufs {
		coilBufs[i] = timeseries.NewCoilRingBuffer(e.bufferSize)
	}
	e.coilBuffers[snap.DeviceID] = coilBufs

	// Initialise baseline statistics slices.
	db.RegisterStats = make([]RegisterBaseline, len(snap.Holding))
	db.CoilStats = make([]CoilLearning, len(snap.Coils))

	// Initialise sample accumulator for register classification.
	e.sampleAccum[snap.DeviceID] = make([][]uint16, len(snap.Holding))
}

// pushToBuffers stores the current snapshot values in the ring buffers.
// Caller must hold the write lock.
func (e *Engine) pushToBuffers(snap *DeviceSnapshot) {
	regBufs := e.registerBuffers[snap.DeviceID]
	for i, v := range snap.Holding {
		if i < len(regBufs) {
			regBufs[i].Push(timeseries.RegisterSample{
				Timestamp: snap.Timestamp,
				Value:     v,
			})
		}
	}

	coilBufs := e.coilBuffers[snap.DeviceID]
	for i, v := range snap.Coils {
		if i < len(coilBufs) {
			coilBufs[i].Push(timeseries.CoilSample{
				Timestamp: snap.Timestamp,
				Value:     v,
			})
		}
	}
}

// updateLearning adds sample data to the Welford accumulators for each register
// and coil. Also updates the response time running statistics.
// Caller must hold the write lock.
func (e *Engine) updateLearning(snap *DeviceSnapshot, db *DeviceBaseline) {
	db.SampleCount++

	for i, v := range snap.Holding {
		if i >= len(db.RegisterStats) {
			break
		}
		db.RegisterStats[i].AddSample(v)
		e.sampleAccum[snap.DeviceID][i] = append(e.sampleAccum[snap.DeviceID][i], v)
	}

	for i, v := range snap.Coils {
		if i >= len(db.CoilStats) {
			break
		}
		cs := &db.CoilStats[i]
		if v {
			cs.TrueCount++
		} else {
			cs.FalseCount++
		}
		// Detect toggle: compare with previous coil state.
		if len(snap.PrevCoils) > i && snap.PrevCoils[i] != v {
			cs.ToggleCount++
		}
	}

	// Update response time statistics using Welford's algorithm.
	db.responseTimeSamples++
	delta := snap.ResponseMs - db.ResponseTimeMean
	db.ResponseTimeMean += delta / float64(db.responseTimeSamples)
	delta2 := snap.ResponseMs - db.ResponseTimeMean
	db.responseTimeM2 += delta * delta2
	if db.responseTimeSamples > 1 {
		db.ResponseTimeStdDev = math.Sqrt(db.responseTimeM2 / float64(db.responseTimeSamples))
	}
}

// finaliseBaseline transitions a device's baseline from learning to established.
// It computes final statistics, classifies register types, and logs the transition.
// Caller must hold the write lock.
func (e *Engine) finaliseBaseline(deviceID string, db *DeviceBaseline) {
	accum := e.sampleAccum[deviceID]
	for i := range db.RegisterStats {
		var samples []uint16
		if i < len(accum) {
			samples = accum[i]
		}
		db.RegisterStats[i].finalise(samples)
	}

	// Compute coil expected state (mode) and max toggle frequency.
	for i := range db.CoilStats {
		cs := &db.CoilStats[i]
		cs.Expected = cs.TrueCount >= cs.FalseCount
		// MaxToggleFrequency: toggles per learning window. We record the raw
		// toggle count per learning window; per-window rate = toggles / cycles.
		if db.SampleCount > 0 {
			cs.MaxToggleFrequency = float64(cs.ToggleCount) / float64(db.SampleCount)
		}
	}

	db.Status = StatusEstablished
	delete(e.sampleAccum, deviceID)

	// Finalise the event baseline: compute interval statistics and release memory.
	finaliseEventBaseline(&db.Event)

	slog.Info("baseline established",
		"device", deviceID,
		"samples", db.SampleCount,
		"registers", len(db.RegisterStats),
	)
}

// GetBaselines returns a copy of all device baselines keyed by device ID.
// Safe to call from any goroutine.
func (e *Engine) GetBaselines() map[string]*DeviceBaseline {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make(map[string]*DeviceBaseline, len(e.baselines))
	for id, db := range e.baselines {
		cp := *db
		regCopy := make([]RegisterBaseline, len(db.RegisterStats))
		copy(regCopy, db.RegisterStats)
		cp.RegisterStats = regCopy
		coilCopy := make([]CoilLearning, len(db.CoilStats))
		copy(coilCopy, db.CoilStats)
		cp.CoilStats = coilCopy
		cp.Event = copyEventBaseline(&db.Event)
		out[id] = &cp
	}
	return out
}

// GetDeviceBaseline returns the baseline for a single device.
// Returns nil, false if the device has not been observed.
func (e *Engine) GetDeviceBaseline(deviceID string) (*DeviceBaseline, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	db, ok := e.baselines[deviceID]
	if !ok {
		return nil, false
	}
	cp := *db
	regCopy := make([]RegisterBaseline, len(db.RegisterStats))
	copy(regCopy, db.RegisterStats)
	cp.RegisterStats = regCopy
	coilCopy := make([]CoilLearning, len(db.CoilStats))
	copy(coilCopy, db.CoilStats)
	cp.CoilStats = coilCopy
	cp.Event = copyEventBaseline(&db.Event)
	return &cp, true
}

// copyEventBaseline returns a deep copy of an EventBaseline, duplicating both
// the ObservedFCs and ObservedSrcs maps so the caller cannot mutate engine state.
func copyEventBaseline(eb *EventBaseline) EventBaseline {
	cp := EventBaseline{
		IsWriteTarget:      eb.IsWriteTarget,
		PollIntervalMean:   eb.PollIntervalMean,
		PollIntervalStdDev: eb.PollIntervalStdDev,
		LastEventTime:      eb.LastEventTime,
	}
	if eb.ObservedFCs != nil {
		cp.ObservedFCs = make(map[uint8]bool, len(eb.ObservedFCs))
		for k, v := range eb.ObservedFCs {
			cp.ObservedFCs[k] = v
		}
	}
	if eb.ObservedSrcs != nil {
		cp.ObservedSrcs = make(map[string]bool, len(eb.ObservedSrcs))
		for k, v := range eb.ObservedSrcs {
			cp.ObservedSrcs[k] = v
		}
	}
	return cp
}
