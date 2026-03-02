package inventory_test

import (
	"sync"
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/inventory"
)

func makeAsset(id string) *inventory.Asset {
	return &inventory.Asset{
		ID:       id,
		Endpoint: "10.10.30.10:5020",
		UnitID:   1,
		Status:   inventory.StatusOnline,
		Protocol: "modbus-tcp",
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}
}

func TestUpsert_NewAsset(t *testing.T) {
	inv := inventory.NewInventory()
	a := makeAsset("10.10.30.10:5020:1")
	inv.Upsert(a)

	got, ok := inv.Get(a.ID)
	if !ok {
		t.Fatal("Get after Upsert: asset not found")
	}
	if got.ID != a.ID {
		t.Errorf("ID: got %q, want %q", got.ID, a.ID)
	}
}

func TestUpsert_ExistingAsset_Updates(t *testing.T) {
	inv := inventory.NewInventory()
	a := makeAsset("10.10.30.10:5020:1")
	a.Status = inventory.StatusOnline
	inv.Upsert(a)

	updated := makeAsset("10.10.30.10:5020:1")
	updated.Status = inventory.StatusOffline
	updated.HoldingRegCount = 5
	inv.Upsert(updated)

	got, ok := inv.Get(a.ID)
	if !ok {
		t.Fatal("Get after second Upsert: asset not found")
	}
	if got.Status != inventory.StatusOffline {
		t.Errorf("Status: got %q, want %q", got.Status, inventory.StatusOffline)
	}
	if got.HoldingRegCount != 5 {
		t.Errorf("HoldingRegCount: got %d, want 5", got.HoldingRegCount)
	}
}

func TestUpsert_PreservesFirstSeen(t *testing.T) {
	inv := inventory.NewInventory()
	first := makeAsset("10.10.30.10:5020:1")
	first.FirstSeen = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	inv.Upsert(first)

	second := makeAsset("10.10.30.10:5020:1")
	second.FirstSeen = time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	inv.Upsert(second)

	got, _ := inv.Get(first.ID)
	if !got.FirstSeen.Equal(first.FirstSeen) {
		t.Errorf("FirstSeen not preserved: got %v, want %v", got.FirstSeen, first.FirstSeen)
	}
}

func TestList_SortedByID(t *testing.T) {
	inv := inventory.NewInventory()
	inv.Upsert(makeAsset("z-last"))
	inv.Upsert(makeAsset("a-first"))
	inv.Upsert(makeAsset("m-middle"))

	list := inv.List()
	if len(list) != 3 {
		t.Fatalf("List length: got %d, want 3", len(list))
	}
	if list[0].ID != "a-first" {
		t.Errorf("list[0]: got %q, want %q", list[0].ID, "a-first")
	}
	if list[1].ID != "m-middle" {
		t.Errorf("list[1]: got %q, want %q", list[1].ID, "m-middle")
	}
	if list[2].ID != "z-last" {
		t.Errorf("list[2]: got %q, want %q", list[2].ID, "z-last")
	}
}

func TestUpdateRegisters_StoresValues(t *testing.T) {
	inv := inventory.NewInventory()
	a := makeAsset("10.10.30.10:5020:1")
	inv.Upsert(a)

	holding := []uint16{100, 200, 300}
	coils := []bool{true, false, true}
	pollTime := time.Now()

	inv.UpdateRegisters(a.ID, holding, coils, pollTime)

	got, _ := inv.Get(a.ID)
	if got.LatestHolding() == nil {
		t.Fatal("LatestHolding: expected non-nil after UpdateRegisters")
	}
	if len(got.LatestHolding()) != 3 {
		t.Errorf("LatestHolding length: got %d, want 3", len(got.LatestHolding()))
	}
	if got.LatestHolding()[1] != 200 {
		t.Errorf("LatestHolding[1]: got %d, want 200", got.LatestHolding()[1])
	}
	if got.LastGoodPollTime.IsZero() {
		t.Error("LastGoodPollTime: should be set after UpdateRegisters")
	}
}

func TestUpdateRegisters_UnknownID_NoOp(t *testing.T) {
	inv := inventory.NewInventory()
	// Should not panic.
	inv.UpdateRegisters("nonexistent", []uint16{1, 2}, nil, time.Now())
}

func TestSetStatus_Transitions(t *testing.T) {
	inv := inventory.NewInventory()
	a := makeAsset("10.10.30.10:5020:1")
	a.Status = inventory.StatusOnline
	inv.Upsert(a)

	now := time.Now()
	inv.SetStatus(a.ID, inventory.StatusOffline, now)

	got, _ := inv.Get(a.ID)
	if got.Status != inventory.StatusOffline {
		t.Errorf("Status: got %q, want %q", got.Status, inventory.StatusOffline)
	}
	if !got.LastSeen.Equal(now) {
		t.Errorf("LastSeen: got %v, want %v", got.LastSeen, now)
	}
}

func TestSetStatus_UnknownID_NoOp(t *testing.T) {
	inv := inventory.NewInventory()
	// Should not panic.
	inv.SetStatus("nonexistent", inventory.StatusOffline, time.Now())
}

func TestConcurrentUpsertAndGet(t *testing.T) {
	inv := inventory.NewInventory()
	const goroutines = 20
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Writers
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				a := makeAsset("10.10.30.10:5020:1")
				a.HoldingRegCount = n*iterations + j
				inv.Upsert(a)
			}
		}(i)
	}

	// Readers
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				inv.Get("10.10.30.10:5020:1")
				inv.List()
			}
		}()
	}

	wg.Wait()
}
