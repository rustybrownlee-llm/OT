package eventstore_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
)

// openMemStore opens an in-memory store for tests. Fails the test on error.
func openMemStore(t *testing.T) *eventstore.Store {
	t.Helper()
	s, err := eventstore.New(":memory:")
	if err != nil {
		t.Fatalf("New(:memory:): %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// makeEvent creates a minimal TransactionEvent with sensible defaults.
// The caller may override individual fields after calling this function.
func makeEvent(deviceID string, fc uint8, isWrite bool) *eventstore.TransactionEvent {
	return eventstore.NewEvent(
		time.Now().UTC().Truncate(time.Microsecond),
		"10.10.30.100:51234",
		"10.10.10.10:5020",
		1,
		fc,
		100,
		10,
		isWrite,
		true,
		0,
		1500,
		deviceID,
		"Test Device",
		"water-treatment",
		nil,
	)
}

// TestNew_Memory verifies that a store can be opened with :memory: for testing.
func TestNew_Memory(t *testing.T) {
	s := openMemStore(t)
	if s == nil {
		t.Fatal("New(:memory:) returned nil store")
	}
}

// TestNew_FileCreatesDirectory verifies that New creates the parent directory
// when the database path is under a directory that does not yet exist.
func TestNew_FileCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir", "nested")
	dbPath := filepath.Join(dir, "events.db")

	s, err := eventstore.New(dbPath)
	if err != nil {
		t.Fatalf("New(%q): %v", dbPath, err)
	}
	defer s.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("database file not created at %q: %v", dbPath, err)
	}
}

// TestWALMode verifies that WAL journal mode is active after opening the store.
// WAL mode is confirmed by the presence of the -wal sidecar file on the first
// write to a file-backed store. In-memory stores do not produce WAL files.
func TestWALMode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wal-test.db")
	fs, err := eventstore.New(dbPath)
	if err != nil {
		t.Fatalf("New(%q): %v", dbPath, err)
	}
	defer fs.Close()

	// WAL mode creates a -wal file on the first write.
	e := makeEvent("wal-device", 3, false)
	if err := fs.Insert(context.Background(), e); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	walPath := dbPath + "-wal"
	if _, err := os.Stat(walPath); err != nil {
		t.Errorf("WAL file not found at %q; WAL mode may not be active: %v", walPath, err)
	}
}

// TestInsert_SingleEvent verifies that a single event can be inserted and
// retrieved with all fields correctly round-tripped.
func TestInsert_SingleEvent(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	e := makeEvent("device-001", 3, false)

	if err := s.Insert(ctx, e); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := s.Query(ctx, eventstore.FilterOptions{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Query returned %d events, want 1", len(results))
	}

	got := results[0]
	assertEventEqual(t, e, got)
}

// TestInsert_WithWriteDetail verifies that WriteDetail is correctly serialized
// to JSON and deserialized back into the event.
func TestInsert_WithWriteDetail(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	detail := &eventstore.WriteDetail{
		Values: []uint16{1234, 5678},
	}
	e := eventstore.NewEvent(
		time.Now().UTC().Truncate(time.Microsecond),
		"10.10.30.100:51234", "10.10.10.10:5020",
		1, 16, 100, 2, true, true, 0, 1800,
		"device-002", "PLC-001", "water-treatment",
		detail,
	)

	if err := s.Insert(ctx, e); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := s.Query(ctx, eventstore.FilterOptions{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Query returned %d events, want 1", len(results))
	}

	got := results[0]
	if got.WriteDetail == nil {
		t.Fatal("WriteDetail: got nil, want non-nil")
	}
	if len(got.WriteDetail.Values) != 2 {
		t.Fatalf("WriteDetail.Values: got len %d, want 2", len(got.WriteDetail.Values))
	}
	if got.WriteDetail.Values[0] != 1234 || got.WriteDetail.Values[1] != 5678 {
		t.Errorf("WriteDetail.Values: got %v, want [1234 5678]", got.WriteDetail.Values)
	}
}

// TestInsert_WithCoilWriteDetail verifies that coil write details are correctly
// round-tripped.
func TestInsert_WithCoilWriteDetail(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	detail := &eventstore.WriteDetail{
		CoilValues: []bool{true, false, true},
	}
	e := eventstore.NewEvent(
		time.Now().UTC().Truncate(time.Microsecond),
		"10.10.30.100:51234", "10.10.10.10:5020",
		1, 15, 0, 3, true, true, 0, 900,
		"device-003", "RTU-001", "manufacturing",
		detail,
	)

	if err := s.Insert(ctx, e); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := s.Query(ctx, eventstore.FilterOptions{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	got := results[0]
	if got.WriteDetail == nil {
		t.Fatal("WriteDetail: got nil, want non-nil")
	}
	if len(got.WriteDetail.CoilValues) != 3 {
		t.Fatalf("WriteDetail.CoilValues: got len %d, want 3", len(got.WriteDetail.CoilValues))
	}
}

// TestInsert_NilWriteDetail verifies that a nil WriteDetail is stored as SQL
// NULL and deserialized back as nil.
func TestInsert_NilWriteDetail(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	e := makeEvent("device-004", 3, false)
	if e.WriteDetail != nil {
		t.Fatal("test setup: makeEvent should return nil WriteDetail for read")
	}

	if err := s.Insert(ctx, e); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := s.Query(ctx, eventstore.FilterOptions{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if results[0].WriteDetail != nil {
		t.Error("WriteDetail: got non-nil, want nil for read operation")
	}
}

// TestInsertBatch_AllEventsStored verifies that InsertBatch inserts all events
// atomically and they are all retrievable.
func TestInsertBatch_AllEventsStored(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	const batchSize = 100
	events := make([]*eventstore.TransactionEvent, batchSize)
	for i := range events {
		e := makeEvent(fmt.Sprintf("batch-device-%03d", i), 3, false)
		events[i] = e
	}

	if err := s.InsertBatch(ctx, events); err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}

	count, err := s.Count(ctx, eventstore.FilterOptions{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != batchSize {
		t.Errorf("Count after batch insert: got %d, want %d", count, batchSize)
	}
}

// TestInsertBatch_Empty verifies that InsertBatch with an empty slice is a no-op.
func TestInsertBatch_Empty(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	if err := s.InsertBatch(ctx, nil); err != nil {
		t.Errorf("InsertBatch(nil): unexpected error: %v", err)
	}
	if err := s.InsertBatch(ctx, []*eventstore.TransactionEvent{}); err != nil {
		t.Errorf("InsertBatch(empty): unexpected error: %v", err)
	}
}

// TestQuery_NoFilters verifies that Query with default options returns all events
// in descending timestamp order (newest first).
func TestQuery_NoFilters(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	e1 := makeEventAt("dev", 3, false, base.Add(-2*time.Second))
	e2 := makeEventAt("dev", 3, false, base.Add(-1*time.Second))
	e3 := makeEventAt("dev", 3, false, base)

	for _, e := range []*eventstore.TransactionEvent{e1, e2, e3} {
		if err := s.Insert(ctx, e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	results, err := s.Query(ctx, eventstore.FilterOptions{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("Query returned %d events, want 3", len(results))
	}

	// Newest first.
	if !results[0].Timestamp.Equal(e3.Timestamp) {
		t.Errorf("results[0] should be newest event; got %v, want %v",
			results[0].Timestamp, e3.Timestamp)
	}
	if !results[2].Timestamp.Equal(e1.Timestamp) {
		t.Errorf("results[2] should be oldest event; got %v, want %v",
			results[2].Timestamp, e1.Timestamp)
	}
}

// TestQuery_AfterFilter verifies that the After filter returns only events at or
// after the specified time.
func TestQuery_AfterFilter(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	old := makeEventAt("dev", 3, false, base.Add(-10*time.Second))
	boundary := makeEventAt("dev", 3, false, base)
	newer := makeEventAt("dev", 3, false, base.Add(1*time.Second))

	for _, e := range []*eventstore.TransactionEvent{old, boundary, newer} {
		if err := s.Insert(ctx, e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	after := base
	results, err := s.Query(ctx, eventstore.FilterOptions{After: &after})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("After filter: got %d events, want 2", len(results))
	}
}

// TestQuery_BeforeFilter verifies that the Before filter returns only events
// strictly before the specified time.
func TestQuery_BeforeFilter(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	old := makeEventAt("dev", 3, false, base.Add(-10*time.Second))
	boundary := makeEventAt("dev", 3, false, base)
	newer := makeEventAt("dev", 3, false, base.Add(1*time.Second))

	for _, e := range []*eventstore.TransactionEvent{old, boundary, newer} {
		if err := s.Insert(ctx, e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	before := base
	results, err := s.Query(ctx, eventstore.FilterOptions{Before: &before})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Before filter: got %d events, want 1", len(results))
	}
}

// TestQuery_DeviceIDFilter verifies that only events for the specified device are
// returned.
func TestQuery_DeviceIDFilter(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := s.Insert(ctx, makeEvent("device-A", 3, false)); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := s.Insert(ctx, makeEvent("device-B", 3, false)); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	deviceID := "device-A"
	results, err := s.Query(ctx, eventstore.FilterOptions{DeviceID: &deviceID})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("DeviceID filter: got %d events, want 5", len(results))
	}
	for _, r := range results {
		if r.DeviceID != "device-A" {
			t.Errorf("DeviceID filter: unexpected device %q in results", r.DeviceID)
		}
	}
}

// TestQuery_FuncCodeFilter verifies that only events for the specified function
// code are returned.
func TestQuery_FuncCodeFilter(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	if err := s.Insert(ctx, makeEvent("dev", 1, false)); err != nil {
		t.Fatalf("Insert FC1: %v", err)
	}
	if err := s.Insert(ctx, makeEvent("dev", 3, false)); err != nil {
		t.Fatalf("Insert FC3: %v", err)
	}
	if err := s.Insert(ctx, makeEvent("dev", 3, false)); err != nil {
		t.Fatalf("Insert FC3 (2): %v", err)
	}

	fc := uint8(3)
	results, err := s.Query(ctx, eventstore.FilterOptions{FuncCode: &fc})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("FuncCode filter: got %d events, want 2", len(results))
	}
}

// TestQuery_IsWriteFilter verifies that only write events are returned when the
// IsWrite filter is set to true.
func TestQuery_IsWriteFilter(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	if err := s.Insert(ctx, makeEvent("dev", 3, false)); err != nil {
		t.Fatalf("Insert read: %v", err)
	}
	if err := s.Insert(ctx, makeEvent("dev", 16, true)); err != nil {
		t.Fatalf("Insert write: %v", err)
	}

	isWrite := true
	results, err := s.Query(ctx, eventstore.FilterOptions{IsWrite: &isWrite})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("IsWrite filter: got %d events, want 1", len(results))
	}
	if !results[0].IsWrite {
		t.Error("IsWrite filter: returned a read event")
	}
}

// TestQuery_SuccessFilter verifies that only failed events are returned when
// the Success filter is set to false.
func TestQuery_SuccessFilter(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	good := makeEvent("dev", 3, false)
	bad := makeEvent("dev", 3, false)
	bad.Success = false
	bad.ExceptionCode = 2

	for _, e := range []*eventstore.TransactionEvent{good, bad} {
		if err := s.Insert(ctx, e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	success := false
	results, err := s.Query(ctx, eventstore.FilterOptions{Success: &success})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Success=false filter: got %d events, want 1", len(results))
	}
	if results[0].Success {
		t.Error("Success=false filter: returned a successful event")
	}
}

// TestQuery_CombinedFilters verifies that multiple filters are applied with AND
// logic.
func TestQuery_CombinedFilters(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	// Three events: only one should match FC=3 AND device=target.
	if err := s.Insert(ctx, makeEvent("target", 3, false)); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := s.Insert(ctx, makeEvent("target", 1, false)); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := s.Insert(ctx, makeEvent("other", 3, false)); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	deviceID := "target"
	fc := uint8(3)
	results, err := s.Query(ctx, eventstore.FilterOptions{
		DeviceID: &deviceID,
		FuncCode: &fc,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("combined filter: got %d events, want 1", len(results))
	}
}

// TestQuery_Limit verifies that the Limit field caps the number of returned rows.
func TestQuery_Limit(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		if err := s.Insert(ctx, makeEvent("dev", 3, false)); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	results, err := s.Query(ctx, eventstore.FilterOptions{Limit: 5})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Limit=5: got %d events, want 5", len(results))
	}
}

// TestQuery_LimitAndOffset verifies pagination: Offset skips the first N results
// of the ordered set.
func TestQuery_LimitAndOffset(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	var inserted []*eventstore.TransactionEvent
	for i := 0; i < 10; i++ {
		e := makeEventAt("dev", 3, false, base.Add(time.Duration(i)*time.Second))
		if err := s.Insert(ctx, e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
		inserted = append(inserted, e)
	}

	// Page 2: skip first 5 (newest), get next 5.
	results, err := s.Query(ctx, eventstore.FilterOptions{Limit: 5, Offset: 5})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Limit=5,Offset=5: got %d events, want 5", len(results))
	}
	// Oldest of the 10 should appear at results[4] (offset puts us in the older half).
	if !results[4].Timestamp.Equal(inserted[0].Timestamp) {
		t.Errorf("Offset=5: results[4] timestamp %v, want %v",
			results[4].Timestamp, inserted[0].Timestamp)
	}
}

// TestCount_MatchesQueryLength verifies that Count returns the same number as
// the length of Query results for the same filter options.
func TestCount_MatchesQueryLength(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	for i := 0; i < 7; i++ {
		if err := s.Insert(ctx, makeEvent("dev", 3, false)); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	opts := eventstore.FilterOptions{}
	results, err := s.Query(ctx, opts)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	count, err := s.Count(ctx, opts)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != int64(len(results)) {
		t.Errorf("Count=%d, len(Query)=%d: should be equal", count, len(results))
	}
}

// TestPrune_DeletesOldEvents verifies that Prune deletes events older than the
// cutoff and keeps events newer than (or at) the cutoff.
func TestPrune_DeletesOldEvents(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	old1 := makeEventAt("dev", 3, false, now.Add(-3*24*time.Hour))
	old2 := makeEventAt("dev", 3, false, now.Add(-2*24*time.Hour))
	recent := makeEventAt("dev", 3, false, now.Add(-1*time.Hour))
	newest := makeEventAt("dev", 3, false, now)

	for _, e := range []*eventstore.TransactionEvent{old1, old2, recent, newest} {
		if err := s.Insert(ctx, e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	// Prune events older than 12 hours ago.
	cutoff := now.Add(-12 * time.Hour)
	deleted, err := s.Prune(ctx, cutoff)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 2 {
		t.Errorf("Prune deleted %d rows, want 2", deleted)
	}

	remaining, err := s.Count(ctx, eventstore.FilterOptions{})
	if err != nil {
		t.Fatalf("Count after prune: %v", err)
	}
	if remaining != 2 {
		t.Errorf("Count after prune: got %d, want 2", remaining)
	}
}

// TestPrune_ReturnsCorrectDeletedCount verifies that the returned count from
// Prune matches the actual number of rows removed.
func TestPrune_ReturnsCorrectDeletedCount(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		e := makeEventAt("dev", 3, false, now.Add(-time.Duration(i+1)*24*time.Hour))
		if err := s.Insert(ctx, e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	// All 5 events are older than 6 hours ago.
	deleted, err := s.Prune(ctx, now.Add(-6*time.Hour))
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 5 {
		t.Errorf("Prune deleted count: got %d, want 5", deleted)
	}
}

// TestPrune_KeepsNewEvents verifies that Prune does not delete events that are
// newer than the cutoff.
func TestPrune_KeepsNewEvents(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		e := makeEventAt("dev", 3, false, now.Add(-time.Duration(i)*time.Minute))
		if err := s.Insert(ctx, e); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	// Cutoff is 1 hour ago -- all events are newer.
	deleted, err := s.Prune(ctx, now.Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 0 {
		t.Errorf("Prune deleted %d rows, want 0 (all events are newer than cutoff)", deleted)
	}

	count, err := s.Count(ctx, eventstore.FilterOptions{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 3 {
		t.Errorf("Count after no-op prune: got %d, want 3", count)
	}
}

// TestConcurrentAccess verifies that concurrent inserts and queries do not
// produce data races or errors. The race detector (-race flag) will catch any
// unsynchronized access.
func TestConcurrentAccess(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	const writers = 5
	const readsPerWriter = 10
	const insertsPerWriter = 20

	var wg sync.WaitGroup

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < insertsPerWriter; i++ {
				e := makeEvent(fmt.Sprintf("concurrent-device-%d", id), 3, false)
				if err := s.Insert(ctx, e); err != nil {
					t.Errorf("concurrent Insert: %v", err)
					return
				}
			}
		}(w)

		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < readsPerWriter; i++ {
				if _, err := s.Query(ctx, eventstore.FilterOptions{Limit: 10}); err != nil {
					t.Errorf("concurrent Query: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()
}

// TestInsert_ExceptionCodeRoundTrip verifies that a failed transaction with an
// exception code is correctly stored and retrieved.
func TestInsert_ExceptionCodeRoundTrip(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	e := makeEvent("device-err", 3, false)
	e.Success = false
	e.ExceptionCode = 2 // Illegal Data Address

	if err := s.Insert(ctx, e); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := s.Query(ctx, eventstore.FilterOptions{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Query: got %d results, want 1", len(results))
	}

	got := results[0]
	if got.Success {
		t.Error("Success: got true, want false")
	}
	if got.ExceptionCode != 2 {
		t.Errorf("ExceptionCode: got %d, want 2", got.ExceptionCode)
	}
}

// makeEventAt creates an event with a specific timestamp, used for time-range
// filter tests.
func makeEventAt(deviceID string, fc uint8, isWrite bool, ts time.Time) *eventstore.TransactionEvent {
	return eventstore.NewEvent(
		ts.Truncate(time.Microsecond),
		"10.10.30.100:51234",
		"10.10.10.10:5020",
		1,
		fc,
		100,
		10,
		isWrite,
		true,
		0,
		1500,
		deviceID,
		"Test Device",
		"water-treatment",
		nil,
	)
}

// assertEventEqual compares two TransactionEvents field by field, reporting
// any mismatches via t.Errorf.
func assertEventEqual(t *testing.T, want, got *eventstore.TransactionEvent) {
	t.Helper()
	if got.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ID, want.ID)
	}
	if !got.Timestamp.Equal(want.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", got.Timestamp, want.Timestamp)
	}
	if got.SrcAddr != want.SrcAddr {
		t.Errorf("SrcAddr: got %q, want %q", got.SrcAddr, want.SrcAddr)
	}
	if got.DstAddr != want.DstAddr {
		t.Errorf("DstAddr: got %q, want %q", got.DstAddr, want.DstAddr)
	}
	if got.UnitID != want.UnitID {
		t.Errorf("UnitID: got %d, want %d", got.UnitID, want.UnitID)
	}
	if got.FunctionCode != want.FunctionCode {
		t.Errorf("FunctionCode: got %d, want %d", got.FunctionCode, want.FunctionCode)
	}
	if got.FunctionName != want.FunctionName {
		t.Errorf("FunctionName: got %q, want %q", got.FunctionName, want.FunctionName)
	}
	if got.AddressStart != want.AddressStart {
		t.Errorf("AddressStart: got %d, want %d", got.AddressStart, want.AddressStart)
	}
	if got.AddressCount != want.AddressCount {
		t.Errorf("AddressCount: got %d, want %d", got.AddressCount, want.AddressCount)
	}
	if got.IsWrite != want.IsWrite {
		t.Errorf("IsWrite: got %v, want %v", got.IsWrite, want.IsWrite)
	}
	if got.Success != want.Success {
		t.Errorf("Success: got %v, want %v", got.Success, want.Success)
	}
	if got.ResponseTimeUs != want.ResponseTimeUs {
		t.Errorf("ResponseTimeUs: got %d, want %d", got.ResponseTimeUs, want.ResponseTimeUs)
	}
	if got.DeviceID != want.DeviceID {
		t.Errorf("DeviceID: got %q, want %q", got.DeviceID, want.DeviceID)
	}
	if got.DeviceName != want.DeviceName {
		t.Errorf("DeviceName: got %q, want %q", got.DeviceName, want.DeviceName)
	}
	if got.EnvID != want.EnvID {
		t.Errorf("EnvID: got %q, want %q", got.EnvID, want.EnvID)
	}
}
