package alert_test

import (
	"sync"
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/alert"
)

// base is a fixed reference time used to create deterministic alert timestamps.
var base = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func makeAlert(id, ruleID, deviceID, severity string, ts time.Time) *alert.Alert {
	return &alert.Alert{
		ID:              id,
		Timestamp:       ts,
		Severity:        severity,
		RuleID:          ruleID,
		DeviceID:        deviceID,
		Description:     "test alert",
		RegisterAddress: -1,
		ExpectedValue:   "N/A",
		ActualValue:     "N/A",
		Active:          true,
	}
}

// TestAlertID_Deterministic verifies that identical inputs produce the same ID.
func TestAlertID_Deterministic(t *testing.T) {
	id1 := alert.AlertID("value_out_of_range", "10.10.30.10:5020:1", 3)
	id2 := alert.AlertID("value_out_of_range", "10.10.30.10:5020:1", 3)
	if id1 != id2 {
		t.Errorf("AlertID not deterministic: %q != %q", id1, id2)
	}
}

// TestAlertID_DiffersOnChange verifies each field change produces a different ID.
func TestAlertID_DiffersOnChange(t *testing.T) {
	base := alert.AlertID("rule", "device", 0)

	cases := []struct {
		name   string
		ruleID string
		device string
		addr   int
	}{
		{"different rule", "other_rule", "device", 0},
		{"different device", "rule", "other_device", 0},
		{"different address", "rule", "device", 1},
	}

	for _, tc := range cases {
		got := alert.AlertID(tc.ruleID, tc.device, tc.addr)
		if got == base {
			t.Errorf("%s: expected different ID, got same %q", tc.name, got)
		}
	}
}

// TestAlertID_Length verifies the ID is exactly 16 hex characters.
func TestAlertID_Length(t *testing.T) {
	id := alert.AlertID("value_out_of_range", "10.10.30.10:5020:1", 0)
	if len(id) != 16 {
		t.Errorf("AlertID length: got %d, want 16", len(id))
	}
}

// TestGetOrCreate_CreatesNewAlert verifies a new alert is created when ID is absent.
func TestGetOrCreate_CreatesNewAlert(t *testing.T) {
	s := alert.NewStore(10)
	id := alert.AlertID("rule", "device", 0)
	created := false

	a := s.GetOrCreate(id, func() *alert.Alert {
		created = true
		return makeAlert(id, "rule", "device", "warning", base)
	})

	if !created {
		t.Error("createFn was not called for new alert")
	}
	if a.ID != id {
		t.Errorf("ID: got %q, want %q", a.ID, id)
	}
	if s.Count() != 1 {
		t.Errorf("Count: got %d, want 1", s.Count())
	}
}

// TestGetOrCreate_ReturnsExistingAlert verifies createFn is not called on repeat.
func TestGetOrCreate_ReturnsExistingAlert(t *testing.T) {
	s := alert.NewStore(10)
	id := alert.AlertID("rule", "device", 0)

	s.GetOrCreate(id, func() *alert.Alert {
		return makeAlert(id, "rule", "device", "warning", base)
	})

	callCount := 0
	a := s.GetOrCreate(id, func() *alert.Alert {
		callCount++
		return makeAlert(id, "rule", "device", "high", base.Add(time.Minute))
	})

	if callCount != 0 {
		t.Errorf("createFn called %d times on repeat GetOrCreate; expected 0", callCount)
	}
	if a.Severity != "warning" {
		t.Errorf("Severity: got %q, want %q (should return original)", a.Severity, "warning")
	}
}

// TestGet_ReturnsAlertByID verifies direct lookup.
func TestGet_ReturnsAlertByID(t *testing.T) {
	s := alert.NewStore(10)
	id := alert.AlertID("rule", "device", 0)
	s.GetOrCreate(id, func() *alert.Alert {
		return makeAlert(id, "rule", "device", "warning", base)
	})

	a, ok := s.Get(id)
	if !ok {
		t.Fatal("Get returned false for existing alert")
	}
	if a.ID != id {
		t.Errorf("Get ID: got %q, want %q", a.ID, id)
	}
}

// TestGet_UnknownIDReturnsFalse verifies missing alert returns false.
func TestGet_UnknownIDReturnsFalse(t *testing.T) {
	s := alert.NewStore(10)
	_, ok := s.Get("nonexistent")
	if ok {
		t.Error("Get returned true for nonexistent ID")
	}
}

// TestList_NoFilters_ReturnsAll verifies List returns all alerts.
func TestList_NoFilters_ReturnsAll(t *testing.T) {
	s := alert.NewStore(10)
	for i := 0; i < 3; i++ {
		id := alert.AlertID("rule", "device", i)
		s.GetOrCreate(id, func() *alert.Alert {
			return makeAlert(id, "rule", "device", "warning", base.Add(time.Duration(i)*time.Minute))
		})
	}

	alerts := s.List()
	if len(alerts) != 3 {
		t.Errorf("List count: got %d, want 3", len(alerts))
	}
}

// TestList_FilterBySeverity verifies severity filtering.
func TestList_FilterBySeverity(t *testing.T) {
	s := alert.NewStore(10)

	idW := alert.AlertID("rule", "d1", 0)
	s.GetOrCreate(idW, func() *alert.Alert { return makeAlert(idW, "rule", "d1", "warning", base) })
	idH := alert.AlertID("rule", "d2", 0)
	s.GetOrCreate(idH, func() *alert.Alert { return makeAlert(idH, "rule", "d2", "high", base) })
	idC := alert.AlertID("rule", "d3", 0)
	s.GetOrCreate(idC, func() *alert.Alert { return makeAlert(idC, "rule", "d3", "critical", base) })

	got := s.List(alert.BySeverity("high"))
	if len(got) != 1 {
		t.Fatalf("BySeverity(high): got %d alerts, want 1", len(got))
	}
	if got[0].Severity != "high" {
		t.Errorf("BySeverity: severity got %q, want %q", got[0].Severity, "high")
	}
}

// TestList_FilterByDevice verifies device filtering.
func TestList_FilterByDevice(t *testing.T) {
	s := alert.NewStore(10)

	id1 := alert.AlertID("rule", "device-A", 0)
	s.GetOrCreate(id1, func() *alert.Alert { return makeAlert(id1, "rule", "device-A", "warning", base) })
	id2 := alert.AlertID("rule", "device-B", 0)
	s.GetOrCreate(id2, func() *alert.Alert { return makeAlert(id2, "rule", "device-B", "warning", base) })

	got := s.List(alert.ByDevice("device-A"))
	if len(got) != 1 {
		t.Fatalf("ByDevice: got %d, want 1", len(got))
	}
	if got[0].DeviceID != "device-A" {
		t.Errorf("ByDevice: got %q, want %q", got[0].DeviceID, "device-A")
	}
}

// TestList_FilterByActive verifies active/inactive filtering.
func TestList_FilterByActive(t *testing.T) {
	s := alert.NewStore(10)

	id1 := alert.AlertID("rule", "d1", 0)
	s.GetOrCreate(id1, func() *alert.Alert { return makeAlert(id1, "rule", "d1", "warning", base) })
	id2 := alert.AlertID("rule", "d2", 0)
	s.GetOrCreate(id2, func() *alert.Alert { return makeAlert(id2, "rule", "d2", "warning", base) })
	s.Clear(id2, base.Add(time.Minute))

	activeAlerts := s.List(alert.ByActive(true))
	if len(activeAlerts) != 1 {
		t.Errorf("ByActive(true): got %d, want 1", len(activeAlerts))
	}

	inactiveAlerts := s.List(alert.ByActive(false))
	if len(inactiveAlerts) != 1 {
		t.Errorf("ByActive(false): got %d, want 1", len(inactiveAlerts))
	}
}

// TestList_MultipleFilters_ANDLogic verifies multiple filters are ANDed.
func TestList_MultipleFilters_ANDLogic(t *testing.T) {
	s := alert.NewStore(10)

	id1 := alert.AlertID("r", "d1", 0)
	s.GetOrCreate(id1, func() *alert.Alert { return makeAlert(id1, "r", "d1", "high", base) })
	id2 := alert.AlertID("r", "d2", 0)
	s.GetOrCreate(id2, func() *alert.Alert { return makeAlert(id2, "r", "d2", "warning", base) })
	id3 := alert.AlertID("r", "d1", 1)
	s.GetOrCreate(id3, func() *alert.Alert { return makeAlert(id3, "r", "d1", "warning", base) })

	// Only d1 + warning should match.
	got := s.List(alert.BySeverity("warning"), alert.ByDevice("d1"))
	if len(got) != 1 {
		t.Fatalf("combined filter: got %d, want 1", len(got))
	}
	if got[0].ID != id3 {
		t.Errorf("combined filter: got ID %q, want %q", got[0].ID, id3)
	}
}

// TestList_SortedByTimestampDescending verifies ordering.
func TestList_SortedByTimestampDescending(t *testing.T) {
	s := alert.NewStore(10)

	id1 := alert.AlertID("r", "d", 0)
	s.GetOrCreate(id1, func() *alert.Alert { return makeAlert(id1, "r", "d", "warning", base) })
	id2 := alert.AlertID("r", "d", 1)
	s.GetOrCreate(id2, func() *alert.Alert {
		return makeAlert(id2, "r", "d", "warning", base.Add(2*time.Minute))
	})
	id3 := alert.AlertID("r", "d", 2)
	s.GetOrCreate(id3, func() *alert.Alert {
		return makeAlert(id3, "r", "d", "warning", base.Add(time.Minute))
	})

	got := s.List()
	if len(got) != 3 {
		t.Fatalf("List count: got %d, want 3", len(got))
	}
	// Newest first: id2 (base+2min), id3 (base+1min), id1 (base).
	if got[0].ID != id2 {
		t.Errorf("got[0].ID: %q, want %q", got[0].ID, id2)
	}
	if got[1].ID != id3 {
		t.Errorf("got[1].ID: %q, want %q", got[1].ID, id3)
	}
	if got[2].ID != id1 {
		t.Errorf("got[2].ID: %q, want %q", got[2].ID, id1)
	}
}

// TestAcknowledge_SetsFlag verifies acknowledgment sets the flag.
func TestAcknowledge_SetsFlag(t *testing.T) {
	s := alert.NewStore(10)
	id := alert.AlertID("rule", "device", 0)
	s.GetOrCreate(id, func() *alert.Alert { return makeAlert(id, "rule", "device", "warning", base) })

	a, ok := s.Acknowledge(id)
	if !ok {
		t.Fatal("Acknowledge returned false for existing ID")
	}
	if !a.Acknowledged {
		t.Error("Acknowledged: expected true after Acknowledge")
	}
}

// TestAcknowledge_UnknownIDReturnsFalse verifies missing alert returns false.
func TestAcknowledge_UnknownIDReturnsFalse(t *testing.T) {
	s := alert.NewStore(10)
	_, ok := s.Acknowledge("nonexistent")
	if ok {
		t.Error("Acknowledge returned true for nonexistent ID")
	}
}

// TestClear_SetsActiveAndClearedAt verifies clear marks alert inactive.
func TestClear_SetsActiveAndClearedAt(t *testing.T) {
	s := alert.NewStore(10)
	id := alert.AlertID("rule", "device", 0)
	s.GetOrCreate(id, func() *alert.Alert { return makeAlert(id, "rule", "device", "warning", base) })

	clearedAt := base.Add(5 * time.Minute)
	s.Clear(id, clearedAt)

	a, ok := s.Get(id)
	if !ok {
		t.Fatal("Get returned false after Clear")
	}
	if a.Active {
		t.Error("Active: expected false after Clear")
	}
	if a.ClearedAt == nil {
		t.Fatal("ClearedAt: expected non-nil after Clear")
	}
	if !a.ClearedAt.Equal(clearedAt) {
		t.Errorf("ClearedAt: got %v, want %v", a.ClearedAt, clearedAt)
	}
}

// TestEviction_OldestClearedEvicted verifies the ring buffer evicts properly.
func TestEviction_OldestClearedEvicted(t *testing.T) {
	s := alert.NewStore(3)

	ids := make([]string, 3)
	for i := 0; i < 3; i++ {
		ids[i] = alert.AlertID("rule", "device", i)
		s.GetOrCreate(ids[i], func() *alert.Alert {
			return makeAlert(ids[i], "rule", "device", "warning", base.Add(time.Duration(i)*time.Minute))
		})
	}
	// Clear the oldest alert (ids[0]).
	s.Clear(ids[0], base.Add(time.Minute))

	// Add a 4th alert, which should evict the oldest cleared (ids[0]).
	id4 := alert.AlertID("rule", "device", 99)
	s.GetOrCreate(id4, func() *alert.Alert {
		return makeAlert(id4, "rule", "device", "warning", base.Add(10*time.Minute))
	})

	// ids[0] should be gone.
	if _, ok := s.Get(ids[0]); ok {
		t.Error("oldest cleared alert should have been evicted but was still found")
	}

	// The new alert and the two remaining active alerts should all be present.
	if _, ok := s.Get(ids[1]); !ok {
		t.Error("ids[1] should still be present")
	}
	if _, ok := s.Get(ids[2]); !ok {
		t.Error("ids[2] should still be present")
	}
	if _, ok := s.Get(id4); !ok {
		t.Error("id4 should be present")
	}
}

// TestConcurrentGetAndGetOrCreate exercises the race detector.
func TestConcurrentGetAndGetOrCreate(t *testing.T) {
	s := alert.NewStore(100)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		idx := i

		// Writer goroutine.
		go func() {
			defer wg.Done()
			id := alert.AlertID("rule", "device", idx)
			s.GetOrCreate(id, func() *alert.Alert {
				return makeAlert(id, "rule", "device", "warning", base.Add(time.Duration(idx)*time.Second))
			})
		}()

		// Reader goroutine.
		go func() {
			defer wg.Done()
			id := alert.AlertID("rule", "device", idx)
			s.Get(id)
		}()
	}

	wg.Wait()
}
