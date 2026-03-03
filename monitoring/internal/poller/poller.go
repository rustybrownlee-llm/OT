// Package poller implements the continuous Modbus register polling loop.
// The poller sequentially reads all holding registers and coils from every
// online device in the asset inventory at a configurable interval.
//
// Sequential polling is intentional: it avoids overloading devices with
// limited concurrent TCP connections (ROC800: 4 max, Moxa NPort: 4 max).
// A 2-second interval is ample for 12 devices on Docker networking (<1ms RTT).
//
// PROTOTYPE-DEBT: [td-poller-027] Sequential polling limits scalability.
// With 12 devices and sub-1ms Docker networking this is not a problem.
// If the simulator grows to 50+ devices or introduces WAN latency, implement
// per-endpoint polling goroutines with a semaphore limiting concurrency.
package poller

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/simonvetter/modbus"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/baseline"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/config"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/inventory"
)

// modbusMaxRead is the Modbus specification maximum registers per read request.
// FC 3/4 (registers): 125 max (125 * 2 bytes = 250 byte PDU payload limit).
const modbusMaxRead = 125

// modbusMaxCoilRead is the Modbus specification maximum coils per read request.
// FC 1/2 (coils): 2000 max (2000 bits = 250 bytes PDU payload limit).
// [OT-REVIEW] The correct Modbus limit for coil reads is 2000, not 125.
// Using the correct limit teaches trainees accurate protocol constraints.
const modbusMaxCoilRead = 2000

// connectTimeout is the TCP connection timeout for reconnection attempts.
const connectTimeout = 2 * time.Second

// CycleHook is called synchronously after each polling cycle with the latest
// device snapshots. The hook is called in the polling goroutine; the next
// cycle does not start until the hook returns.
type CycleHook func(snapshots []baseline.DeviceSnapshot)

// EventHook is called synchronously after each polling cycle with the
// TransactionEvent values collected during that cycle. Called in the polling
// goroutine; the next cycle does not start until the hook returns.
// The slice is owned by the caller after delivery; the poller does not retain it.
// [OT-REVIEW] EventHook fires before CycleHook so that event records exist in
// the store before any derivative processing (baseline engine, alerting) runs.
type EventHook func(events []*eventstore.TransactionEvent)

// PollState records the current state of the polling loop for health reporting.
type PollState struct {
	mu             sync.RWMutex
	lastCycleStart time.Time
	lastCycleEnd   time.Time
	devicesOnline  int
	devicesOffline int
}

// LastCycleTime returns the start time of the most recent completed poll cycle.
func (ps *PollState) LastCycleTime() time.Time {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.lastCycleEnd
}

// Counts returns the most recently recorded online and offline device counts.
func (ps *PollState) Counts() (online, offline int) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.devicesOnline, ps.devicesOffline
}

func (ps *PollState) recordCycleStart() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.lastCycleStart = time.Now()
}

func (ps *PollState) recordCycleEnd(online, offline int) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.lastCycleEnd = time.Now()
	ps.devicesOnline = online
	ps.devicesOffline = offline
}

// clientPool manages one Modbus TCP client per endpoint address (host:port).
// Unit ID switching is done via SetUnitId on a shared client per endpoint,
// which keeps the TCP connection count bounded at one per physical/virtual device.
type clientPool struct {
	mu      sync.Mutex
	clients map[string]*modbus.ModbusClient
}

func newClientPool() *clientPool {
	return &clientPool{
		clients: make(map[string]*modbus.ModbusClient),
	}
}

// get returns the Modbus client for the given endpoint, creating it if needed.
// The caller must call SetUnitId before issuing reads.
func (cp *clientPool) get(endpoint string) (*modbus.ModbusClient, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if c, ok := cp.clients[endpoint]; ok {
		return c, nil
	}

	c, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:     "tcp://" + endpoint,
		Timeout: connectTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("new client for %s: %w", endpoint, err)
	}
	if err := c.Open(); err != nil {
		return nil, fmt.Errorf("open client for %s: %w", endpoint, err)
	}
	cp.clients[endpoint] = c
	return c, nil
}

// remove closes and removes the client for an endpoint. Used when a device goes offline.
func (cp *clientPool) remove(endpoint string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if c, ok := cp.clients[endpoint]; ok {
		_ = c.Close()
		delete(cp.clients, endpoint)
	}
}

// closeAll closes all open Modbus connections.
func (cp *clientPool) closeAll() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for ep, c := range cp.clients {
		if err := c.Close(); err != nil {
			slog.Warn("client close error", "endpoint", ep, "error", err)
		}
	}
	cp.clients = make(map[string]*modbus.ModbusClient)
}

// Poller reads registers from all online devices on a configurable interval.
//
// PROTOTYPE-DEBT: [td-poller-028] Parallel code path: pollAsset (no events) and
// pollAssetWithEvents (with events). Once EventHook is always set, pollAsset can
// be removed. Beta 0.7: Remove pollAsset after all integrations use EventHook.
type Poller struct {
	cfg       *config.Config
	inv       *inventory.Inventory
	state     *PollState
	pool      *clientPool
	interval  time.Duration
	hook      CycleHook
	eventHook EventHook // transaction event delivery hook
	prevCycle map[string]prevSnapshot

	// PROTOTYPE-DEBT: [td-poller-029] SrcAddr uses monitor's known IP with port 0,
	// not the actual TCP ephemeral port of the Modbus connection. Beta 0.7:
	// Passive capture provides actual source addresses including ports.
	srcAddr string // monitor's source address placeholder for TransactionEvent.SrcAddr
}

// prevSnapshot holds the previous cycle's register values for one device.
type prevSnapshot struct {
	holding []uint16
	coils   []bool
}

// New creates a Poller. The poller is idle until Run is called.
func New(cfg *config.Config, inv *inventory.Inventory, state *PollState) *Poller {
	return &Poller{
		cfg:       cfg,
		inv:       inv,
		state:     state,
		pool:      newClientPool(),
		interval:  time.Duration(cfg.PollIntervalSeconds) * time.Second,
		prevCycle: make(map[string]prevSnapshot),
		srcAddr:   resolveMonitorAddr(cfg),
	}
}

// SetCycleHook registers a function to be called synchronously after each
// polling cycle. The hook receives DeviceSnapshot values for all known devices
// (both online and offline). Calling SetCycleHook while the poller is running
// is safe -- the hook is only read inside the polling goroutine.
func (p *Poller) SetCycleHook(hook CycleHook) {
	p.hook = hook
}

// SetEventHook registers a function to be called synchronously after each
// polling cycle with all TransactionEvent values collected during that cycle.
// EventHook fires before CycleHook so that events are persisted before
// derivative processing. Calling SetEventHook while the poller is running
// is safe -- the hook is only read inside the polling goroutine.
func (p *Poller) SetEventHook(hook EventHook) {
	p.eventHook = hook
}

// Run enters the continuous polling loop. It blocks until ctx is cancelled.
// The interval is measured from the START of one cycle to the START of the next.
// If a cycle takes longer than the interval, the next cycle starts immediately.
func (p *Poller) Run(ctx context.Context) error {
	slog.Info("poller starting", "interval_seconds", p.cfg.PollIntervalSeconds)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Run the first cycle immediately rather than waiting for the first tick.
	p.runCycle(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("poller stopping")
			return nil
		case <-ticker.C:
			p.runCycle(ctx)
		}
	}
}

// cycleResult holds the per-asset results of a single polling cycle.
type cycleResult struct {
	assetID    string
	holding    []uint16
	coils      []bool
	responseMs float64
	online     bool
}

// runCycle polls every asset in the inventory once and calls the cycle hooks.
// EventHook fires before CycleHook so that event records exist in the store
// before any derivative processing (baseline engine, alerting) runs.
func (p *Poller) runCycle(ctx context.Context) {
	p.state.recordCycleStart()
	start := time.Now()

	assets := p.inv.List()
	online, offline := 0, 0
	results := make([]cycleResult, 0, len(assets))
	var allEvents []*eventstore.TransactionEvent

	for _, a := range assets {
		if ctx != nil && ctx.Err() != nil {
			break
		}

		if a.Status == inventory.StatusOffline {
			result, events := p.handleOfflineAsset(ctx, a)
			allEvents = append(allEvents, events...)
			results = append(results, result)
			if result.online {
				online++
			} else {
				offline++
			}
			continue
		}

		result, events := p.handleOnlineAsset(a)
		allEvents = append(allEvents, events...)
		results = append(results, result)
		if result.online {
			online++
		} else {
			offline++
		}
	}

	duration := time.Since(start)
	p.state.recordCycleEnd(online, offline)
	slog.Info("poll cycle complete",
		"devices_polled", online,
		"devices_offline", offline,
		"duration_ms", duration.Milliseconds())

	// EventHook fires before CycleHook: persist event records before derivative processing.
	if p.eventHook != nil && len(allEvents) > 0 {
		p.eventHook(allEvents)
	}

	if p.hook != nil {
		snapshots := p.buildSnapshots(results)
		p.hook(snapshots)
	}

	p.updatePrevCycle(results)
}

// handleOfflineAsset attempts reconnection for an offline asset.
// Returns the cycle result and any events collected during a successful reconnect poll.
func (p *Poller) handleOfflineAsset(ctx context.Context, a *inventory.Asset) (cycleResult, []*eventstore.TransactionEvent) {
	if err := p.reconnectAsset(ctx, a); err != nil {
		slog.Debug("reconnect failed", "id", a.ID, "error", err)
		return cycleResult{assetID: a.ID, online: false}, nil
	}

	refreshed, ok := p.inv.Get(a.ID)
	if !ok {
		return cycleResult{assetID: a.ID, online: true}, nil
	}
	return cycleResult{
		assetID: a.ID,
		holding: refreshed.LatestHolding(),
		coils:   refreshed.LatestCoils(),
		online:  true,
	}, nil
}

// handleOnlineAsset polls a single online asset using the appropriate code path
// based on whether an EventHook is registered.
func (p *Poller) handleOnlineAsset(a *inventory.Asset) (cycleResult, []*eventstore.TransactionEvent) {
	if p.eventHook != nil {
		return p.handleOnlineAssetWithEvents(a)
	}

	pollStart := time.Now()
	if err := p.pollAsset(context.Background(), a); err != nil {
		slog.Warn("poll failed", "id", a.ID, "error", err)
		p.pool.remove(a.Endpoint)
		p.inv.SetStatus(a.ID, inventory.StatusOffline, time.Now())
		return cycleResult{assetID: a.ID, online: false}, nil
	}

	responseMs := float64(time.Since(pollStart).Microseconds()) / 1000.0
	refreshed, ok := p.inv.Get(a.ID)
	var holding []uint16
	var coils []bool
	if ok {
		holding = refreshed.LatestHolding()
		coils = refreshed.LatestCoils()
	}
	return cycleResult{
		assetID:    a.ID,
		holding:    holding,
		coils:      coils,
		responseMs: responseMs,
		online:     true,
	}, nil
}

// handleOnlineAssetWithEvents polls a single online asset and collects events.
func (p *Poller) handleOnlineAssetWithEvents(a *inventory.Asset) (cycleResult, []*eventstore.TransactionEvent) {
	pollStart := time.Now()
	events, err := p.pollAssetWithEvents(a)
	if err != nil {
		slog.Warn("poll failed", "id", a.ID, "error", err)
		p.pool.remove(a.Endpoint)
		p.inv.SetStatus(a.ID, inventory.StatusOffline, time.Now())
		return cycleResult{assetID: a.ID, online: false}, events
	}

	responseMs := float64(time.Since(pollStart).Microseconds()) / 1000.0
	refreshed, ok := p.inv.Get(a.ID)
	var holding []uint16
	var coils []bool
	if ok {
		holding = refreshed.LatestHolding()
		coils = refreshed.LatestCoils()
	}
	return cycleResult{
		assetID:    a.ID,
		holding:    holding,
		coils:      coils,
		responseMs: responseMs,
		online:     true,
	}, events
}

// buildSnapshots converts cycle results to DeviceSnapshot values, including
// the previous cycle's values for change-detection rules.
func (p *Poller) buildSnapshots(results []cycleResult) []baseline.DeviceSnapshot {
	snapshots := make([]baseline.DeviceSnapshot, 0, len(results))
	now := time.Now()

	for _, r := range results {
		prev := p.prevCycle[r.assetID]
		snap := baseline.DeviceSnapshot{
			DeviceID:    r.assetID,
			Timestamp:   now,
			Holding:     r.holding,
			Coils:       r.coils,
			ResponseMs:  r.responseMs,
			Online:      r.online,
			PrevHolding: prev.holding,
			PrevCoils:   prev.coils,
		}
		snapshots = append(snapshots, snap)
	}

	return snapshots
}

// updatePrevCycle saves the current cycle's register values for use in the
// next cycle's DeviceSnapshot.PrevHolding and PrevCoils fields.
func (p *Poller) updatePrevCycle(results []cycleResult) {
	for _, r := range results {
		if r.online {
			h := make([]uint16, len(r.holding))
			copy(h, r.holding)
			c := make([]bool, len(r.coils))
			copy(c, r.coils)
			p.prevCycle[r.assetID] = prevSnapshot{holding: h, coils: c}
		}
	}
}

// reconnectAsset attempts to open a new Modbus connection to an offline asset.
// On success, the asset is promoted to online. Returns an error if still unreachable.
func (p *Poller) reconnectAsset(ctx context.Context, a *inventory.Asset) error {
	_, err := p.pool.get(a.Endpoint)
	if err != nil {
		return err
	}
	// Connection succeeded; attempt a poll to confirm registers are readable.
	if err := p.pollAsset(ctx, a); err != nil {
		p.pool.remove(a.Endpoint)
		return err
	}
	p.inv.SetStatus(a.ID, inventory.StatusOnline, time.Now())
	slog.Info("device reconnected", "id", a.ID)
	return nil
}

// Shutdown closes all open Modbus connections. Safe to call from any goroutine.
func (p *Poller) Shutdown() {
	slog.Info("poller shutting down, closing all connections")
	p.pool.closeAll()
}
