// event_rules.go implements event-driven baseline learning and anomaly detection
// for the four protocol-metadata rules introduced in SOW-032.0:
//
//   - write_to_readonly: Write FC targets a device that only received reads during learning.
//   - new_source: Transaction originated from a source address not seen during learning.
//   - fc_anomaly: Function code not observed during the learning period.
//   - poll_gap: No transaction seen within 3x the learned polling interval mean.
//
// These rules complement the existing register-value rules by operating on
// TransactionEvent metadata (function codes, source addresses, polling intervals)
// rather than register values. In production OT monitoring, protocol-metadata
// detection catches attacker reconnaissance and unauthorized commands before
// process values change.
//
// PROTOTYPE-DEBT: [td-baseline-040] new_source is ineffective in Beta 0.6
// (single Modbus client). Becomes valuable with passive capture in Beta 0.7.
// PROTOTYPE-DEBT: [td-baseline-041] poll_gap uses a fixed 3x threshold.
// Production tools allow configurable thresholds per device.
// PROTOTYPE-DEBT: [td-baseline-042] EventBaseline uses the same SampleCount
// threshold as register learning. Event counts may differ from snapshot counts.
// PROTOTYPE-DEBT: [td-baseline-043] fc_anomaly and new_source add to observed
// sets permanently, preventing re-alerting. A sliding window would allow
// re-alerting after a quiet period.
package baseline

import (
	"fmt"
	"math"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/alert"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
)

// maxLearningInterval is the maximum polling gap that counts as a legitimate
// polling interval during the learning period. Gaps above this threshold are
// treated as restarts and excluded from mean/stddev computation (FR-7).
const maxLearningInterval = 60 * time.Second

// RecordEvents processes a batch of TransactionEvents for baseline learning
// and anomaly detection. It is the event-driven counterpart to RecordCycle.
//
// During learning: updates the EventBaseline (FC set, source set, intervals).
// After establishment: evaluates the four event-driven anomaly rules.
// Handles nil and empty slices without error (FR-11 in success criteria).
func (e *Engine) RecordEvents(events []*eventstore.TransactionEvent) {
	if len(events) == 0 {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for _, ev := range events {
		if ev == nil {
			continue
		}
		db := e.ensureEventBaseline(ev.DeviceID)
		if db.Status == StatusLearning {
			updateEventLearning(ev, &db.Event)
		} else {
			evaluateEventRules(ev, db, e.alertStore)
		}
	}
}

// ensureEventBaseline returns the DeviceBaseline for the given device ID,
// creating a minimal baseline if none exists. Unlike getOrCreateBaseline,
// this method does not require a DeviceSnapshot and initializes only the
// EventBaseline maps. Caller must hold the write lock.
func (e *Engine) ensureEventBaseline(deviceID string) *DeviceBaseline {
	db, ok := e.baselines[deviceID]
	if !ok {
		db = &DeviceBaseline{
			DeviceID:        deviceID,
			Status:          StatusLearning,
			RequiredSamples: e.learningCycles,
			Event: EventBaseline{
				ObservedFCs:  make(map[uint8]bool),
				ObservedSrcs: make(map[string]bool),
			},
		}
		e.baselines[deviceID] = db
		return db
	}

	// Ensure EventBaseline maps are initialized on an existing baseline
	// (handles baselines created via getOrCreateBaseline before RecordEvents).
	if db.Event.ObservedFCs == nil {
		db.Event.ObservedFCs = make(map[uint8]bool)
	}
	if db.Event.ObservedSrcs == nil {
		db.Event.ObservedSrcs = make(map[string]bool)
	}
	return db
}

// updateEventLearning updates the EventBaseline with data from one event.
// Called during the learning period only. Caller must hold the write lock.
func updateEventLearning(ev *eventstore.TransactionEvent, eb *EventBaseline) {
	eb.ObservedFCs[ev.FunctionCode] = true
	eb.ObservedSrcs[ev.SrcAddr] = true

	if ev.IsWrite {
		eb.IsWriteTarget = true
	}

	// Compute polling interval from the previous event for this device.
	// Skip interval computation on the first event (no prior LastEventTime).
	if !eb.LastEventTime.IsZero() {
		interval := ev.Timestamp.Sub(eb.LastEventTime)
		// Skip intervals that exceed the restart threshold (FR-7, FR-8 note 8).
		if interval <= maxLearningInterval && interval > 0 {
			eb.PollIntervals = append(eb.PollIntervals, interval)
		}
	}
	eb.LastEventTime = ev.Timestamp
}

// finaliseEventBaseline computes PollIntervalMean and PollIntervalStdDev from
// the accumulated intervals using Welford's online algorithm, then releases
// the PollIntervals slice to free memory. Called from finaliseBaseline.
func finaliseEventBaseline(eb *EventBaseline) {
	intervals := eb.PollIntervals

	if len(intervals) > 0 {
		// Welford's online algorithm for mean and variance.
		var mean, m2 float64
		count := 0

		for _, d := range intervals {
			count++
			x := float64(d)
			delta := x - mean
			mean += delta / float64(count)
			delta2 := x - mean
			m2 += delta * delta2
		}

		eb.PollIntervalMean = time.Duration(mean)
		if count > 1 {
			eb.PollIntervalStdDev = time.Duration(math.Sqrt(m2 / float64(count)))
		}
	}

	// Release the accumulator slice (same pattern as sampleAccum cleanup).
	eb.PollIntervals = nil
}

// evaluateEventRules evaluates all four event-driven rules for one event.
// Called after baseline establishment only. Caller must hold the write lock.
func evaluateEventRules(ev *eventstore.TransactionEvent, db *DeviceBaseline, store *alert.Store) {
	// Compute elapsed time since last event and update LastEventTime.
	var interval time.Duration
	if !db.Event.LastEventTime.IsZero() {
		interval = ev.Timestamp.Sub(db.Event.LastEventTime)
	}
	db.Event.LastEventTime = ev.Timestamp

	evaluateWriteToReadonly(ev, db, store)
	evaluateFCAnomaly(ev, db, store)
	evaluateNewSource(ev, db, store)
	evaluatePollGap(ev, db, store, interval)
}

// evaluateWriteToReadonly fires a Critical alert when a write FC targets a
// device that only received read function codes during the entire learning period.
// Fires regardless of Success value -- a failed write attempt still indicates
// someone tried to modify a process control register. (FR-3)
func evaluateWriteToReadonly(ev *eventstore.TransactionEvent, db *DeviceBaseline, store *alert.Store) {
	if !ev.IsWrite || db.Event.IsWriteTarget {
		return
	}

	alertID := alert.AlertID("write_to_readonly", db.DeviceID, -1)
	store.GetOrCreate(alertID, func() *alert.Alert {
		writeStatus := "succeeded"
		if !ev.Success {
			writeStatus = "failed"
		}
		return &alert.Alert{
			ID:              alertID,
			Timestamp:       ev.Timestamp,
			Severity:        "critical",
			RuleID:          "write_to_readonly",
			DeviceID:        db.DeviceID,
			Description:     fmt.Sprintf("Write FC %d to read-only device %s (write %s)", ev.FunctionCode, db.DeviceID, writeStatus),
			RegisterAddress: -1,
			ExpectedValue:   "no write operations (device was read-only during learning)",
			ActualValue:     fmt.Sprintf("FC %d (%s)", ev.FunctionCode, ev.FunctionName),
			Active:          true,
		}
	})
}

// evaluateFCAnomaly fires a High alert when a function code not observed during
// the learning period is seen. The new FC is added to the observed set after
// the first alert to prevent re-alerting on the same FC. (FR-5)
func evaluateFCAnomaly(ev *eventstore.TransactionEvent, db *DeviceBaseline, store *alert.Store) {
	if db.Event.ObservedFCs[ev.FunctionCode] {
		return
	}

	alertID := alert.AlertID("fc_anomaly", db.DeviceID, -1)
	store.GetOrCreate(alertID, func() *alert.Alert {
		return &alert.Alert{
			ID:              alertID,
			Timestamp:       ev.Timestamp,
			Severity:        "high",
			RuleID:          "fc_anomaly",
			DeviceID:        db.DeviceID,
			Description:     fmt.Sprintf("Unexpected function code FC %d (%s) on device %s", ev.FunctionCode, ev.FunctionName, db.DeviceID),
			RegisterAddress: -1,
			ExpectedValue:   "function codes observed during learning period",
			ActualValue:     fmt.Sprintf("FC %d (%s)", ev.FunctionCode, ev.FunctionName),
			Active:          true,
		}
	})

	// Add to observed set to prevent re-alerting on the same FC.
	db.Event.ObservedFCs[ev.FunctionCode] = true
}

// evaluateNewSource fires a High alert when a transaction originates from a
// source address not seen during the learning period. The new source is added
// to the observed set after the first alert to prevent re-alerting. (FR-4)
func evaluateNewSource(ev *eventstore.TransactionEvent, db *DeviceBaseline, store *alert.Store) {
	if db.Event.ObservedSrcs[ev.SrcAddr] {
		return
	}

	alertID := alert.AlertID("new_source", db.DeviceID, -1)
	store.GetOrCreate(alertID, func() *alert.Alert {
		return &alert.Alert{
			ID:              alertID,
			Timestamp:       ev.Timestamp,
			Severity:        "high",
			RuleID:          "new_source",
			DeviceID:        db.DeviceID,
			Description:     fmt.Sprintf("Unexpected source address %s communicating with device %s", ev.SrcAddr, db.DeviceID),
			RegisterAddress: -1,
			ExpectedValue:   "source addresses observed during learning period",
			ActualValue:     ev.SrcAddr,
			Active:          true,
		}
	})

	// Add to observed set to prevent re-alerting on the same source.
	db.Event.ObservedSrcs[ev.SrcAddr] = true
}

// evaluatePollGap fires a Warning alert when no transaction was seen for a
// device within 3x the learned polling interval mean. Clears the alert when
// normal polling resumes. Skips evaluation entirely when PollIntervalMean is
// zero (device had too few valid intervals during learning). (FR-6)
func evaluatePollGap(ev *eventstore.TransactionEvent, db *DeviceBaseline, store *alert.Store, interval time.Duration) {
	if db.Event.PollIntervalMean == 0 {
		return
	}

	alertID := alert.AlertID("poll_gap", db.DeviceID, -1)
	threshold := 3 * db.Event.PollIntervalMean

	if interval > threshold {
		store.GetOrCreate(alertID, func() *alert.Alert {
			return &alert.Alert{
				ID:              alertID,
				Timestamp:       ev.Timestamp,
				Severity:        "warning",
				RuleID:          "poll_gap",
				DeviceID:        db.DeviceID,
				Description:     fmt.Sprintf("Polling gap %v exceeds 3x baseline mean %v for device %s", interval.Round(time.Millisecond), db.Event.PollIntervalMean.Round(time.Millisecond), db.DeviceID),
				RegisterAddress: -1,
				ExpectedValue:   fmt.Sprintf("<= %v (3x baseline mean)", threshold.Round(time.Millisecond)),
				ActualValue:     interval.Round(time.Millisecond).String(),
				Active:          true,
			}
		})
	} else {
		store.Clear(alertID, ev.Timestamp)
	}
}
