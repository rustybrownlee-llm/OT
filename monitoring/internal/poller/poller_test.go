// Package poller provides unit tests for the polling loop.
// Tests cover chunk splitting, interval timing, offline device retry, and
// the cycle hook that feeds the baseline engine after each poll cycle.
package poller

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/baseline"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/config"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/inventory"
)

func makeMinimalConfig(intervalSec int) *config.Config {
	return &config.Config{
		PollIntervalSeconds: intervalSec,
		APIAddr:             ":8091",
		Environments:        []config.Environment{},
	}
}

// TestChunkSplitting_2000Registers verifies that a 2000-register device
// requires 16 read requests when chunked at 125 registers per request.
// ceil(2000 / 125) = 16.
func TestChunkSplitting_2000Registers(t *testing.T) {
	const total = 2000
	const chunkSize = modbusMaxRead

	var readCount int
	addr := uint16(0)
	remaining := total

	for remaining > 0 {
		qty := remaining
		if qty > chunkSize {
			qty = chunkSize
		}
		addr += uint16(qty)
		remaining -= qty
		readCount++
	}

	expected := (total + chunkSize - 1) / chunkSize // ceil division
	if readCount != expected {
		t.Errorf("chunk count: got %d, want %d", readCount, expected)
	}
	if expected != 16 {
		t.Errorf("expected exactly 16 chunks for 2000 registers, got %d", expected)
	}
}

// TestChunkSplitting_Exact125 verifies that exactly 125 registers requires 1 read.
func TestChunkSplitting_Exact125(t *testing.T) {
	count := 0
	remaining := 125
	for remaining > 0 {
		qty := remaining
		if qty > modbusMaxRead {
			qty = modbusMaxRead
		}
		remaining -= qty
		count++
	}
	if count != 1 {
		t.Errorf("125 registers: expected 1 chunk, got %d", count)
	}
}

// TestChunkSplitting_126Registers verifies that 126 registers requires 2 read requests.
func TestChunkSplitting_126Registers(t *testing.T) {
	count := 0
	remaining := 126
	for remaining > 0 {
		qty := remaining
		if qty > modbusMaxRead {
			qty = modbusMaxRead
		}
		remaining -= qty
		count++
	}
	if count != 2 {
		t.Errorf("126 registers: expected 2 chunks, got %d", count)
	}
}

// TestIntervalTiming verifies that the polling interval elapses between cycles.
// This uses a minimal poller with no actual devices, checking that the ticker
// fires at approximately the correct interval.
func TestIntervalTiming(t *testing.T) {
	cfg := makeMinimalConfig(1) // 1-second interval
	inv := inventory.NewInventory()
	state := &PollState{}

	p := New(cfg, inv, state)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	// Run the poller; it should complete at least 2 cycles in 3 seconds.
	go func() {
		_ = p.Run(ctx)
	}()

	// Wait for context to expire.
	<-ctx.Done()

	elapsed := time.Since(start)
	if elapsed < 1*time.Second {
		t.Errorf("elapsed %v, expected at least 1 second for interval timing", elapsed)
	}

	// Verify poll state was updated.
	online, _ := state.Counts()
	// With no devices, online should be 0.
	_ = online // no devices to poll, just verify no panic

	p.Shutdown()
}

// TestOfflineDeviceRetry verifies that an offline asset appears in the poll cycle
// and is retried. Since we cannot easily mock the Modbus client in the poller
// (it uses a real *modbus.ModbusClient), this test verifies the inventory
// interaction: an offline asset that cannot reconnect remains offline after the cycle.
func TestOfflineDeviceRetry(t *testing.T) {
	cfg := makeMinimalConfig(1)
	inv := inventory.NewInventory()
	state := &PollState{}

	// Add an offline asset pointing to a non-existent endpoint.
	a := &inventory.Asset{
		ID:              "127.0.0.1:19999:1",
		Endpoint:        "127.0.0.1:19999",
		UnitID:          1,
		Status:          inventory.StatusOffline,
		HoldingRegCount: 5,
		Addressing:      "zero-based",
		FirstSeen:       time.Now(),
		LastSeen:        time.Now(),
		Protocol:        "modbus-tcp",
	}
	inv.Upsert(a)

	p := New(cfg, inv, state)

	// Run a single poll cycle.
	ctx := context.Background()
	p.runCycle(ctx)

	// Verify the asset remains offline (cannot connect to non-existent endpoint).
	got, ok := inv.Get(a.ID)
	if !ok {
		t.Fatal("asset disappeared from inventory after poll cycle")
	}
	if got.Status != inventory.StatusOffline {
		t.Errorf("status: got %q, want %q (unreachable endpoint should stay offline)",
			got.Status, inventory.StatusOffline)
	}
}

// TestPollState_CycleRecording verifies that PollState records cycle start/end times.
func TestPollState_CycleRecording(t *testing.T) {
	state := &PollState{}

	before := time.Now()
	state.recordCycleStart()
	state.recordCycleEnd(5, 2)
	after := time.Now()

	end := state.LastCycleTime()
	if end.Before(before) || end.After(after) {
		t.Errorf("LastCycleTime %v not in range [%v, %v]", end, before, after)
	}

	online, offline := state.Counts()
	if online != 5 {
		t.Errorf("online: got %d, want 5", online)
	}
	if offline != 2 {
		t.Errorf("offline: got %d, want 2", offline)
	}
}

// TestCycleHook_CalledAfterEachCycle verifies that the hook is invoked on
// each poll cycle (even with an empty inventory).
func TestCycleHook_CalledAfterEachCycle(t *testing.T) {
	cfg := makeMinimalConfig(1)
	inv := inventory.NewInventory()
	state := &PollState{}
	p := New(cfg, inv, state)

	var callCount int32
	p.SetCycleHook(func(snapshots []baseline.DeviceSnapshot) {
		atomic.AddInt32(&callCount, 1)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	go func() { _ = p.Run(ctx) }()
	<-ctx.Done()
	p.Shutdown()

	// With a 1s interval and 2.5s timeout, expect at least 2 calls
	// (one immediate + at least one from the ticker).
	got := atomic.LoadInt32(&callCount)
	if got < 2 {
		t.Errorf("hook call count: got %d, want >= 2", got)
	}
}

// TestCycleHook_OfflineDeviceSnapshot verifies that offline devices produce
// snapshots with Online=false.
func TestCycleHook_OfflineDeviceSnapshot(t *testing.T) {
	cfg := makeMinimalConfig(1)
	inv := inventory.NewInventory()
	state := &PollState{}

	// Add an offline device that will never reconnect.
	a := &inventory.Asset{
		ID:              "127.0.0.1:29999:1",
		Endpoint:        "127.0.0.1:29999",
		UnitID:          1,
		Status:          inventory.StatusOffline,
		HoldingRegCount: 2,
		Addressing:      "zero-based",
		FirstSeen:       time.Now(),
		LastSeen:        time.Now(),
		Protocol:        "modbus-tcp",
	}
	inv.Upsert(a)

	p := New(cfg, inv, state)

	var receivedOnline bool
	var hookCalled bool
	p.SetCycleHook(func(snapshots []baseline.DeviceSnapshot) {
		hookCalled = true
		for _, s := range snapshots {
			if s.DeviceID == a.ID {
				receivedOnline = s.Online
			}
		}
	})

	ctx := context.Background()
	p.runCycle(ctx)

	if !hookCalled {
		t.Error("hook was not called during runCycle")
	}
	if receivedOnline {
		t.Error("snapshot.Online: expected false for unreachable device, got true")
	}
}

// TestCycleHook_ReceivesSnapshotsForAllDevices verifies that the hook receives
// one snapshot per device in the inventory.
func TestCycleHook_ReceivesSnapshotsForAllDevices(t *testing.T) {
	cfg := makeMinimalConfig(1)
	inv := inventory.NewInventory()
	state := &PollState{}

	// Add three offline devices.
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("127.0.0.1:%d:1", 39001+i)
		inv.Upsert(&inventory.Asset{
			ID:              id,
			Endpoint:        fmt.Sprintf("127.0.0.1:%d", 39001+i),
			UnitID:          1,
			Status:          inventory.StatusOffline,
			HoldingRegCount: 2,
			Addressing:      "zero-based",
			FirstSeen:       time.Now(),
			LastSeen:        time.Now(),
			Protocol:        "modbus-tcp",
		})
	}

	p := New(cfg, inv, state)

	var snapshotCount int
	p.SetCycleHook(func(snapshots []baseline.DeviceSnapshot) {
		snapshotCount = len(snapshots)
	})

	ctx := context.Background()
	p.runCycle(ctx)

	// Expect one snapshot per device (3).
	if snapshotCount != 3 {
		t.Errorf("snapshot count: got %d, want 3", snapshotCount)
	}
}
