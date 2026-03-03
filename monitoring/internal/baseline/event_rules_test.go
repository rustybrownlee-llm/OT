package baseline_test

import (
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/baseline"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
)

// eventTestTime is the fixed base timestamp used in event rule tests.
var eventTestTime = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

// makeEvent constructs a minimal TransactionEvent for testing.
func makeEvent(deviceID string, fc uint8, isWrite bool, success bool, srcAddr string, ts time.Time) *eventstore.TransactionEvent {
	return &eventstore.TransactionEvent{
		ID:           "test-event-id",
		Timestamp:    ts,
		SrcAddr:      srcAddr,
		DstAddr:      "192.168.1.10:502",
		UnitID:       1,
		FunctionCode: fc,
		FunctionName: eventstore.LookupFuncCode(fc).Name,
		IsWrite:      isWrite,
		Success:      success,
		DeviceID:     deviceID,
	}
}

// buildEstablishedEventEngine creates an Engine with an established baseline
// for deviceID by feeding nCycles snapshot cycles and nCycles events. Events
// use fc 3 (Read Holding Registers), srcAddr, and 1-second intervals.
// The device is treated as read-only unless isWriteTarget is true.
func buildEstablishedEventEngine(t *testing.T, deviceID string, nCycles int, srcAddr string, isWriteTarget bool) *baseline.Engine {
	t.Helper()
	e := baseline.NewEngine(nCycles, 300, 1000)
	e.SetKnownDevices([]string{deviceID})

	for i := 0; i < nCycles; i++ {
		ts := eventTestTime.Add(time.Duration(i) * time.Second)

		// Feed snapshot cycle for register-based baseline.
		snap := baseline.DeviceSnapshot{
			DeviceID:   deviceID,
			Timestamp:  ts,
			Holding:    []uint16{100},
			Online:     true,
			ResponseMs: 5.0,
		}
		e.RecordCycle([]baseline.DeviceSnapshot{snap})

		// Feed event for protocol-metadata baseline.
		fc := uint8(3) // Read Holding Registers
		isWrite := false
		if isWriteTarget && i == 0 {
			fc = 6 // Write Single Register -- marks device as write target
			isWrite = true
		}
		ev := makeEvent(deviceID, fc, isWrite, true, srcAddr, ts)
		e.RecordEvents([]*eventstore.TransactionEvent{ev})
	}
	return e
}

// --- Event learning tests ---

func TestEventLearning_AccumulatesFCs(t *testing.T) {
	const nCycles = 5
	e := baseline.NewEngine(nCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	// Feed events with two different FCs during learning.
	events := []*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "127.0.0.1:5020", eventTestTime),
		makeEvent("dev1", 4, false, true, "127.0.0.1:5020", eventTestTime.Add(time.Second)),
	}
	e.RecordEvents(events)

	db, ok := e.GetDeviceBaseline("dev1")
	if !ok {
		t.Fatal("GetDeviceBaseline returned false")
	}
	if !db.Event.ObservedFCs[3] {
		t.Error("expected FC 3 in ObservedFCs after learning event")
	}
	if !db.Event.ObservedFCs[4] {
		t.Error("expected FC 4 in ObservedFCs after learning event")
	}
}

func TestEventLearning_AccumulatesSourceAddresses(t *testing.T) {
	const nCycles = 5
	e := baseline.NewEngine(nCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	events := []*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "192.168.1.100:5020", eventTestTime),
		makeEvent("dev1", 3, false, true, "10.0.0.1:5021", eventTestTime.Add(time.Second)),
	}
	e.RecordEvents(events)

	db, ok := e.GetDeviceBaseline("dev1")
	if !ok {
		t.Fatal("GetDeviceBaseline returned false")
	}
	if !db.Event.ObservedSrcs["192.168.1.100:5020"] {
		t.Error("expected 192.168.1.100:5020 in ObservedSrcs")
	}
	if !db.Event.ObservedSrcs["10.0.0.1:5021"] {
		t.Error("expected 10.0.0.1:5021 in ObservedSrcs")
	}
}

func TestEventLearning_SetsIsWriteTarget(t *testing.T) {
	const nCycles = 5
	e := baseline.NewEngine(nCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	// Initially not a write target.
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "127.0.0.1:5020", eventTestTime),
	})
	db, _ := e.GetDeviceBaseline("dev1")
	if db.Event.IsWriteTarget {
		t.Error("IsWriteTarget should be false before any write event")
	}

	// Now send a write event.
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 6, true, true, "127.0.0.1:5020", eventTestTime.Add(time.Second)),
	})
	db, _ = e.GetDeviceBaseline("dev1")
	if !db.Event.IsWriteTarget {
		t.Error("IsWriteTarget should be true after write FC event")
	}
}

func TestEventLearning_ComputesPollingIntervals(t *testing.T) {
	const nCycles = 10
	e := baseline.NewEngine(nCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	// Feed nCycles snapshot cycles and events at 1-second intervals.
	for i := 0; i < nCycles; i++ {
		ts := eventTestTime.Add(time.Duration(i) * time.Second)
		snap := baseline.DeviceSnapshot{
			DeviceID: "dev1", Timestamp: ts,
			Holding: []uint16{100}, Online: true, ResponseMs: 5.0,
		}
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
		e.RecordEvents([]*eventstore.TransactionEvent{
			makeEvent("dev1", 3, false, true, "127.0.0.1:5020", ts),
		})
	}

	db, ok := e.GetDeviceBaseline("dev1")
	if !ok {
		t.Fatal("GetDeviceBaseline returned false")
	}
	if db.Status != baseline.StatusEstablished {
		t.Fatalf("expected established status, got %q", db.Status)
	}
	// Mean should be approximately 1 second.
	if db.Event.PollIntervalMean == 0 {
		t.Error("PollIntervalMean should be non-zero after learning with 1s intervals")
	}
	// PollIntervals slice should be nil after finalization.
	if db.Event.PollIntervals != nil {
		t.Error("PollIntervals slice should be nil after finalization")
	}
}

// --- No alerts during learning tests ---

func TestEventRules_SilentDuringLearning(t *testing.T) {
	const nCycles = 10
	e := baseline.NewEngine(nCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	// Send a write to a device that started as read-only -- still in learning.
	for i := 0; i < nCycles-1; i++ {
		ts := eventTestTime.Add(time.Duration(i) * time.Second)
		snap := baseline.DeviceSnapshot{
			DeviceID: "dev1", Timestamp: ts,
			Holding: []uint16{100}, Online: true, ResponseMs: 5.0,
		}
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
		// Write event during learning -- should NOT fire write_to_readonly.
		e.RecordEvents([]*eventstore.TransactionEvent{
			makeEvent("dev1", 6, true, true, "127.0.0.1:5020", ts),
		})
	}

	for _, a := range e.AlertStore().List() {
		if a.RuleID == "write_to_readonly" || a.RuleID == "fc_anomaly" ||
			a.RuleID == "new_source" || a.RuleID == "poll_gap" {
			t.Errorf("event rule %q fired during learning period (should be silent)", a.RuleID)
		}
	}
}

// --- write_to_readonly tests ---

func TestWriteToReadonly_FiresForReadOnlyDevice(t *testing.T) {
	const nCycles = 5
	e := buildEstablishedEventEngine(t, "dev1", nCycles, "127.0.0.1:5020", false)

	db, _ := e.GetDeviceBaseline("dev1")
	if db.Status != baseline.StatusEstablished {
		t.Fatalf("expected established status, got %q", db.Status)
	}
	if db.Event.IsWriteTarget {
		t.Fatal("device should not be a write target after read-only learning")
	}

	// Now send a write event to a device that was read-only during learning.
	ts := eventTestTime.Add(time.Duration(nCycles) * time.Second)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 6, true, true, "127.0.0.1:5020", ts),
	})

	found := false
	for _, a := range e.AlertStore().List() {
		if a.RuleID == "write_to_readonly" && a.Active && a.DeviceID == "dev1" {
			found = true
			if a.Severity != "critical" {
				t.Errorf("write_to_readonly: expected critical severity, got %q", a.Severity)
			}
		}
	}
	if !found {
		t.Error("write_to_readonly: expected active critical alert for write to read-only device, none found")
	}
}

func TestWriteToReadonly_SilentForWriteTargetDevice(t *testing.T) {
	const nCycles = 5
	// Build engine where device received writes during learning.
	e := buildEstablishedEventEngine(t, "dev1", nCycles, "127.0.0.1:5020", true)

	db, _ := e.GetDeviceBaseline("dev1")
	if !db.Event.IsWriteTarget {
		t.Fatal("device should be a write target after write-inclusive learning")
	}

	// Another write after establishment should NOT trigger write_to_readonly.
	ts := eventTestTime.Add(time.Duration(nCycles) * time.Second)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 6, true, true, "127.0.0.1:5020", ts),
	})

	for _, a := range e.AlertStore().List() {
		if a.RuleID == "write_to_readonly" && a.Active {
			t.Error("write_to_readonly: unexpected alert for device that received writes during learning")
		}
	}
}

func TestWriteToReadonly_FiresOnFailedWrite(t *testing.T) {
	const nCycles = 5
	e := buildEstablishedEventEngine(t, "dev1", nCycles, "127.0.0.1:5020", false)

	// Failed write (Success=false) -- should still fire Critical.
	ts := eventTestTime.Add(time.Duration(nCycles) * time.Second)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 6, true, false, "127.0.0.1:5020", ts),
	})

	found := false
	for _, a := range e.AlertStore().List() {
		if a.RuleID == "write_to_readonly" && a.Active && a.Severity == "critical" {
			found = true
		}
	}
	if !found {
		t.Error("write_to_readonly: expected critical alert for failed write to read-only device, none found")
	}
}

// --- fc_anomaly tests ---

func TestFCAnomaly_FiresForUnseenFC(t *testing.T) {
	const nCycles = 5
	e := buildEstablishedEventEngine(t, "dev1", nCycles, "127.0.0.1:5020", false)

	// FC 43 (Read Device Identification) was never seen during learning.
	ts := eventTestTime.Add(time.Duration(nCycles) * time.Second)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 43, false, true, "127.0.0.1:5020", ts),
	})

	found := false
	for _, a := range e.AlertStore().List() {
		if a.RuleID == "fc_anomaly" && a.Active && a.DeviceID == "dev1" {
			found = true
			if a.Severity != "high" {
				t.Errorf("fc_anomaly: expected high severity, got %q", a.Severity)
			}
		}
	}
	if !found {
		t.Error("fc_anomaly: expected High alert for unseen function code, none found")
	}
}

func TestFCAnomaly_FiresOnlyOnce(t *testing.T) {
	const nCycles = 5
	e := buildEstablishedEventEngine(t, "dev1", nCycles, "127.0.0.1:5020", false)

	// Send FC 43 twice -- should only fire once (FC added to set after first alert).
	ts1 := eventTestTime.Add(time.Duration(nCycles) * time.Second)
	ts2 := ts1.Add(time.Second)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 43, false, true, "127.0.0.1:5020", ts1),
		makeEvent("dev1", 43, false, true, "127.0.0.1:5020", ts2),
	})

	count := 0
	for _, a := range e.AlertStore().List() {
		if a.RuleID == "fc_anomaly" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("fc_anomaly: expected 1 alert (idempotent), got %d", count)
	}
}

func TestFCAnomaly_SilentForSeenFC(t *testing.T) {
	const nCycles = 5
	// Learning uses FC 3; FC 3 should not trigger fc_anomaly.
	e := buildEstablishedEventEngine(t, "dev1", nCycles, "127.0.0.1:5020", false)

	ts := eventTestTime.Add(time.Duration(nCycles) * time.Second)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "127.0.0.1:5020", ts),
	})

	for _, a := range e.AlertStore().List() {
		if a.RuleID == "fc_anomaly" && a.Active {
			t.Error("fc_anomaly: unexpected alert for function code seen during learning")
		}
	}
}

// --- new_source tests ---

func TestNewSource_FiresForUnseenSource(t *testing.T) {
	const nCycles = 5
	e := buildEstablishedEventEngine(t, "dev1", nCycles, "127.0.0.1:5020", false)

	// New source address not seen during learning.
	ts := eventTestTime.Add(time.Duration(nCycles) * time.Second)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "10.0.0.99:9999", ts),
	})

	found := false
	for _, a := range e.AlertStore().List() {
		if a.RuleID == "new_source" && a.Active && a.DeviceID == "dev1" {
			found = true
			if a.Severity != "high" {
				t.Errorf("new_source: expected high severity, got %q", a.Severity)
			}
		}
	}
	if !found {
		t.Error("new_source: expected High alert for unseen source address, none found")
	}
}

func TestNewSource_FiresOnlyOnce(t *testing.T) {
	const nCycles = 5
	e := buildEstablishedEventEngine(t, "dev1", nCycles, "127.0.0.1:5020", false)

	// Send same new source twice -- should only fire once (source added to set).
	ts1 := eventTestTime.Add(time.Duration(nCycles) * time.Second)
	ts2 := ts1.Add(time.Second)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "10.0.0.99:9999", ts1),
		makeEvent("dev1", 3, false, true, "10.0.0.99:9999", ts2),
	})

	count := 0
	for _, a := range e.AlertStore().List() {
		if a.RuleID == "new_source" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("new_source: expected 1 alert (idempotent), got %d", count)
	}
}

func TestNewSource_SilentForKnownSource(t *testing.T) {
	const nCycles = 5
	// Learning uses "127.0.0.1:5020"; same source after establishment is fine.
	e := buildEstablishedEventEngine(t, "dev1", nCycles, "127.0.0.1:5020", false)

	ts := eventTestTime.Add(time.Duration(nCycles) * time.Second)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "127.0.0.1:5020", ts),
	})

	for _, a := range e.AlertStore().List() {
		if a.RuleID == "new_source" && a.Active {
			t.Error("new_source: unexpected alert for source address seen during learning")
		}
	}
}

// --- poll_gap tests ---

// buildEstablishedEventEngineWithIntervals creates an established engine for
// deviceID with consistent polling at interval spacing during learning.
func buildEstablishedEventEngineWithIntervals(t *testing.T, deviceID string, nCycles int, interval time.Duration) *baseline.Engine {
	t.Helper()
	e := baseline.NewEngine(nCycles, 300, 1000)
	e.SetKnownDevices([]string{deviceID})

	for i := 0; i < nCycles; i++ {
		ts := eventTestTime.Add(time.Duration(i) * interval)
		snap := baseline.DeviceSnapshot{
			DeviceID: deviceID, Timestamp: ts,
			Holding: []uint16{100}, Online: true, ResponseMs: 5.0,
		}
		e.RecordCycle([]baseline.DeviceSnapshot{snap})
		e.RecordEvents([]*eventstore.TransactionEvent{
			makeEvent(deviceID, 3, false, true, "127.0.0.1:5020", ts),
		})
	}
	return e
}

func TestPollGap_FiresWhenIntervalExceeds3xMean(t *testing.T) {
	const nCycles = 10
	const pollInterval = time.Second

	e := buildEstablishedEventEngineWithIntervals(t, "dev1", nCycles, pollInterval)

	db, _ := e.GetDeviceBaseline("dev1")
	if db.Status != baseline.StatusEstablished {
		t.Fatalf("expected established status, got %q", db.Status)
	}
	if db.Event.PollIntervalMean == 0 {
		t.Fatal("PollIntervalMean should be non-zero")
	}

	// Send event with gap > 3x mean. Mean ~= 1s, so gap of 5s should trigger.
	lastEventTime := eventTestTime.Add(time.Duration(nCycles-1) * pollInterval)
	bigGap := lastEventTime.Add(5 * time.Second)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "127.0.0.1:5020", bigGap),
	})

	found := false
	for _, a := range e.AlertStore().List() {
		if a.RuleID == "poll_gap" && a.Active && a.DeviceID == "dev1" {
			found = true
			if a.Severity != "warning" {
				t.Errorf("poll_gap: expected warning severity, got %q", a.Severity)
			}
		}
	}
	if !found {
		t.Errorf("poll_gap: expected warning alert for gap > 3x mean (mean=%v), none found",
			db.Event.PollIntervalMean)
	}
}

func TestPollGap_ClearsWhenNormalPollingResumes(t *testing.T) {
	const nCycles = 10
	const pollInterval = time.Second

	e := buildEstablishedEventEngineWithIntervals(t, "dev1", nCycles, pollInterval)

	// Trigger poll_gap with a large gap.
	lastEventTime := eventTestTime.Add(time.Duration(nCycles-1) * pollInterval)
	bigGapTime := lastEventTime.Add(10 * time.Second)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "127.0.0.1:5020", bigGapTime),
	})

	// Verify alert is active.
	alertActive := false
	for _, a := range e.AlertStore().List() {
		if a.RuleID == "poll_gap" && a.Active {
			alertActive = true
		}
	}
	if !alertActive {
		t.Fatal("poll_gap: expected active alert after large gap; cannot test clear behavior")
	}

	// Normal event arrives shortly after (interval <= 3x mean).
	normalTime := bigGapTime.Add(pollInterval)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "127.0.0.1:5020", normalTime),
	})

	for _, a := range e.AlertStore().List() {
		if a.RuleID == "poll_gap" && a.Active {
			t.Error("poll_gap: expected alert to clear when normal polling resumes")
		}
	}
}

func TestPollGap_SkipsWhenMeanIsZero(t *testing.T) {
	// Engine with only 1 event during learning produces no valid intervals
	// (interval requires at least 2 events), so PollIntervalMean == 0.
	const nCycles = 1
	e := baseline.NewEngine(nCycles, 300, 100)
	e.SetKnownDevices([]string{"dev1"})

	ts := eventTestTime
	snap := baseline.DeviceSnapshot{
		DeviceID: "dev1", Timestamp: ts,
		Holding: []uint16{100}, Online: true, ResponseMs: 5.0,
	}
	e.RecordCycle([]baseline.DeviceSnapshot{snap})
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "127.0.0.1:5020", ts),
	})

	db, _ := e.GetDeviceBaseline("dev1")
	if db.Status != baseline.StatusEstablished {
		t.Fatalf("expected established, got %q", db.Status)
	}
	if db.Event.PollIntervalMean != 0 {
		t.Fatalf("expected PollIntervalMean=0 with 1 event, got %v", db.Event.PollIntervalMean)
	}

	// Even with a huge gap, poll_gap should NOT fire when mean is zero.
	bigGapTime := ts.Add(10 * time.Minute)
	e.RecordEvents([]*eventstore.TransactionEvent{
		makeEvent("dev1", 3, false, true, "127.0.0.1:5020", bigGapTime),
	})

	for _, a := range e.AlertStore().List() {
		if a.RuleID == "poll_gap" && a.Active {
			t.Error("poll_gap: unexpected alert when PollIntervalMean is zero")
		}
	}
}

// --- nil and empty slice handling ---

func TestRecordEvents_HandlesNilSlice(t *testing.T) {
	e := baseline.NewEngine(5, 300, 100)
	// Must not panic.
	e.RecordEvents(nil)
}

func TestRecordEvents_HandlesEmptySlice(t *testing.T) {
	e := baseline.NewEngine(5, 300, 100)
	// Must not panic.
	e.RecordEvents([]*eventstore.TransactionEvent{})
}

func TestRecordEvents_HandlesNilEventInSlice(t *testing.T) {
	e := baseline.NewEngine(5, 300, 100)
	e.SetKnownDevices([]string{"dev1"})
	// Nil entry in slice must not panic.
	e.RecordEvents([]*eventstore.TransactionEvent{nil})
}
