package eventstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
)

// TestDeviceStats_EmptyStore verifies that DeviceStats returns an empty
// non-nil slice when the event store has no events.
func TestDeviceStats_EmptyStore(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	stats, err := s.DeviceStats(ctx)
	if err != nil {
		t.Fatalf("DeviceStats on empty store: %v", err)
	}
	if stats == nil {
		t.Fatal("DeviceStats: got nil, want empty slice")
	}
	if len(stats) != 0 {
		t.Errorf("DeviceStats: got %d entries, want 0", len(stats))
	}
}

// TestDeviceStats_TotalAndWriteCounts verifies that DeviceStats correctly
// separates total event counts from write-only counts per device.
func TestDeviceStats_TotalAndWriteCounts(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	// device-A: 3 reads, 2 writes
	for i := 0; i < 3; i++ {
		mustInsert(t, s, makeEvent("device-A", 3, false))
	}
	for i := 0; i < 2; i++ {
		mustInsert(t, s, makeEvent("device-A", 16, true))
	}
	// device-B: 4 reads only
	for i := 0; i < 4; i++ {
		mustInsert(t, s, makeEvent("device-B", 3, false))
	}

	stats, err := s.DeviceStats(ctx)
	if err != nil {
		t.Fatalf("DeviceStats: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("DeviceStats: got %d entries, want 2", len(stats))
	}

	byID := indexStatsByID(stats)

	a := byID["device-A"]
	if a.TotalEvents != 5 {
		t.Errorf("device-A TotalEvents: got %d, want 5", a.TotalEvents)
	}
	if a.WriteEvents != 2 {
		t.Errorf("device-A WriteEvents: got %d, want 2", a.WriteEvents)
	}

	b := byID["device-B"]
	if b.TotalEvents != 4 {
		t.Errorf("device-B TotalEvents: got %d, want 4", b.TotalEvents)
	}
	if b.WriteEvents != 0 {
		t.Errorf("device-B WriteEvents: got %d, want 0", b.WriteEvents)
	}
}

// TestDeviceStats_DeviceNameFromData verifies that DeviceName is populated
// from the MAX(device_name) aggregate of the underlying event rows.
func TestDeviceStats_DeviceNameFromData(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	e := makeEvent("dev-001", 3, false)
	e.DeviceName = "Water Pump PLC"
	mustInsert(t, s, e)

	stats, err := s.DeviceStats(ctx)
	if err != nil {
		t.Fatalf("DeviceStats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("DeviceStats: got %d entries, want 1", len(stats))
	}
	if stats[0].DeviceName != "Water Pump PLC" {
		t.Errorf("DeviceName: got %q, want %q", stats[0].DeviceName, "Water Pump PLC")
	}
}

// TestDeviceStats_DistinctFCs verifies that DistinctFCs contains the sorted
// unique set of function codes observed for each device.
func TestDeviceStats_DistinctFCs(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	// device-X: FC1 and FC3 reads (FC3 twice -- should appear once in DistinctFCs)
	mustInsert(t, s, makeEvent("device-X", 1, false))
	mustInsert(t, s, makeEvent("device-X", 3, false))
	mustInsert(t, s, makeEvent("device-X", 3, false))

	stats, err := s.DeviceStats(ctx)
	if err != nil {
		t.Fatalf("DeviceStats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("DeviceStats: got %d entries, want 1", len(stats))
	}

	fcs := stats[0].DistinctFCs
	if len(fcs) != 2 {
		t.Fatalf("DistinctFCs: got %d, want 2 (FC1, FC3)", len(fcs))
	}
	if fcs[0] != 1 {
		t.Errorf("DistinctFCs[0]: got FC%d, want FC1", fcs[0])
	}
	if fcs[1] != 3 {
		t.Errorf("DistinctFCs[1]: got FC%d, want FC3", fcs[1])
	}
}

// TestDeviceStats_LastSeenIsMaxTimestamp verifies that LastSeen reflects the
// most recent event timestamp for the device.
func TestDeviceStats_LastSeenIsMaxTimestamp(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	mustInsert(t, s, makeEventAt("dev-ts", 3, false, base.Add(-1*time.Minute)))
	mustInsert(t, s, makeEventAt("dev-ts", 3, false, base))

	stats, err := s.DeviceStats(ctx)
	if err != nil {
		t.Fatalf("DeviceStats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("DeviceStats: got %d entries, want 1", len(stats))
	}
	if !stats[0].LastSeen.Equal(base) {
		t.Errorf("LastSeen: got %v, want %v", stats[0].LastSeen, base)
	}
}

// TestCommEdges_EmptyStore verifies that CommEdges returns an empty non-nil
// slice when the event store has no events.
func TestCommEdges_EmptyStore(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	edges, err := s.CommEdges(ctx)
	if err != nil {
		t.Fatalf("CommEdges on empty store: %v", err)
	}
	if edges == nil {
		t.Fatal("CommEdges: got nil, want empty slice")
	}
	if len(edges) != 0 {
		t.Errorf("CommEdges: got %d entries, want 0", len(edges))
	}
}

// TestCommEdges_SourceDestPairs verifies that CommEdges groups events by
// (src_addr, device_id) and reports correct event counts.
func TestCommEdges_SourceDestPairs(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	// Same src, two different devices.
	mustInsert(t, s, makeEventWithSrc("10.10.30.100:51234", "dev-A", 3, false))
	mustInsert(t, s, makeEventWithSrc("10.10.30.100:51234", "dev-A", 3, false))
	mustInsert(t, s, makeEventWithSrc("10.10.30.100:51235", "dev-B", 16, true))

	edges, err := s.CommEdges(ctx)
	if err != nil {
		t.Fatalf("CommEdges: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("CommEdges: got %d edges, want 2", len(edges))
	}

	// Highest event count first (dev-A with 2 events).
	if edges[0].DstDeviceID != "dev-A" {
		t.Errorf("edges[0].DstDeviceID: got %q, want %q", edges[0].DstDeviceID, "dev-A")
	}
	if edges[0].EventCount != 2 {
		t.Errorf("edges[0].EventCount: got %d, want 2", edges[0].EventCount)
	}
	if edges[0].WriteCount != 0 {
		t.Errorf("edges[0].WriteCount: got %d, want 0", edges[0].WriteCount)
	}
	if edges[1].WriteCount != 1 {
		t.Errorf("edges[1].WriteCount: got %d, want 1", edges[1].WriteCount)
	}
}

// TestFCDistribution_EmptyStore verifies that FCDistribution returns an empty
// non-nil slice when the device has no events.
func TestFCDistribution_EmptyStore(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	counts, err := s.FCDistribution(ctx, "nonexistent-device")
	if err != nil {
		t.Fatalf("FCDistribution on empty store: %v", err)
	}
	if counts == nil {
		t.Fatal("FCDistribution: got nil, want empty slice")
	}
	if len(counts) != 0 {
		t.Errorf("FCDistribution: got %d entries, want 0", len(counts))
	}
}

// TestFCDistribution_CorrectCounts verifies that FCDistribution returns the
// correct per-FC counts, ordered by count descending.
func TestFCDistribution_CorrectCounts(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	// FC3: 10 reads, FC16: 2 writes, FC1: 5 reads -- for target device only.
	for i := 0; i < 10; i++ {
		mustInsert(t, s, makeEvent("target-dev", 3, false))
	}
	for i := 0; i < 2; i++ {
		mustInsert(t, s, makeEvent("target-dev", 16, true))
	}
	for i := 0; i < 5; i++ {
		mustInsert(t, s, makeEvent("target-dev", 1, false))
	}
	// Other device -- must not appear in results.
	mustInsert(t, s, makeEvent("other-dev", 3, false))

	counts, err := s.FCDistribution(ctx, "target-dev")
	if err != nil {
		t.Fatalf("FCDistribution: %v", err)
	}
	if len(counts) != 3 {
		t.Fatalf("FCDistribution: got %d entries, want 3", len(counts))
	}

	// Ordered by count DESC: FC3(10), FC1(5), FC16(2).
	if counts[0].FuncCode != 3 || counts[0].Count != 10 {
		t.Errorf("counts[0]: got FC%d count=%d, want FC3 count=10",
			counts[0].FuncCode, counts[0].Count)
	}
	if counts[1].FuncCode != 1 || counts[1].Count != 5 {
		t.Errorf("counts[1]: got FC%d count=%d, want FC1 count=5",
			counts[1].FuncCode, counts[1].Count)
	}
	if counts[2].FuncCode != 16 || counts[2].Count != 2 {
		t.Errorf("counts[2]: got FC%d count=%d, want FC16 count=2",
			counts[2].FuncCode, counts[2].Count)
	}
}

// TestFCDistribution_IsWriteFlag verifies that IsWrite is correctly set for
// read vs. write function codes in the distribution output.
func TestFCDistribution_IsWriteFlag(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	mustInsert(t, s, makeEvent("dev-iswrite", 3, false))  // read
	mustInsert(t, s, makeEvent("dev-iswrite", 16, true))  // write

	counts, err := s.FCDistribution(ctx, "dev-iswrite")
	if err != nil {
		t.Fatalf("FCDistribution: %v", err)
	}
	if len(counts) != 2 {
		t.Fatalf("FCDistribution: got %d entries, want 2", len(counts))
	}

	byFC := indexFCCountsByCode(counts)
	if byFC[3].IsWrite {
		t.Error("FC3 IsWrite: got true, want false (read function code)")
	}
	if !byFC[16].IsWrite {
		t.Error("FC16 IsWrite: got false, want true (write function code)")
	}
}

// TestFCDistribution_IsolatedByDevice verifies that FCDistribution only
// returns events for the specified device, not all devices.
func TestFCDistribution_IsolatedByDevice(t *testing.T) {
	s := openMemStore(t)
	ctx := context.Background()

	mustInsert(t, s, makeEvent("dev-query", 3, false))
	mustInsert(t, s, makeEvent("dev-other", 16, true))
	mustInsert(t, s, makeEvent("dev-other", 1, false))

	counts, err := s.FCDistribution(ctx, "dev-query")
	if err != nil {
		t.Fatalf("FCDistribution: %v", err)
	}
	if len(counts) != 1 {
		t.Fatalf("FCDistribution: got %d entries, want 1 (only FC3 for dev-query)",
			len(counts))
	}
	if counts[0].FuncCode != 3 {
		t.Errorf("FCDistribution: got FC%d, want FC3", counts[0].FuncCode)
	}
}

// --- helpers ---

// mustInsert inserts an event or fails the test.
func mustInsert(t *testing.T, s *eventstore.Store, e *eventstore.TransactionEvent) {
	t.Helper()
	if err := s.Insert(context.Background(), e); err != nil {
		t.Fatalf("Insert: %v", err)
	}
}

// makeEventWithSrc creates an event with a specific source address for
// testing CommEdges grouping by src_addr.
func makeEventWithSrc(src, deviceID string, fc uint8, isWrite bool) *eventstore.TransactionEvent {
	return eventstore.NewEvent(
		time.Now().UTC().Truncate(time.Microsecond),
		src,
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

// indexStatsByID indexes a DeviceStat slice by DeviceID for test assertions.
func indexStatsByID(stats []eventstore.DeviceStat) map[string]eventstore.DeviceStat {
	m := make(map[string]eventstore.DeviceStat, len(stats))
	for _, s := range stats {
		m[s.DeviceID] = s
	}
	return m
}

// indexFCCountsByCode indexes an FCCount slice by FuncCode for test assertions.
func indexFCCountsByCode(counts []eventstore.FCCount) map[uint8]eventstore.FCCount {
	m := make(map[uint8]eventstore.FCCount, len(counts))
	for _, c := range counts {
		m[c.FuncCode] = c
	}
	return m
}
