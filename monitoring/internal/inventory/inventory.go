// Package inventory provides the in-memory asset model for discovered Modbus devices.
// The Inventory type is safe for concurrent access from the polling goroutine (writes)
// and HTTP handler goroutines (reads).
package inventory

import (
	"sort"
	"sync"
	"time"
)

// StatusOnline and StatusOffline are the valid values for Asset.Status.
const (
	StatusOnline  = "online"
	StatusOffline = "offline"
)

// Asset represents a discovered Modbus device endpoint and its characteristics.
// All fields are discovered by network probing; none are assumed from configuration.
//
// [OT-REVIEW] LastGoodPollTime enables detection of "stuck" registers -- a value
// unchanged across many poll cycles may indicate a frozen sensor, not a stable reading.
type Asset struct {
	ID               string    `json:"id"`                    // "10.10.30.10:5020:1"
	Endpoint         string    `json:"endpoint"`              // "10.10.30.10:5020"
	UnitID           uint8     `json:"unit_id"`
	Addressing       string    `json:"addressing"`            // "zero-based" or "one-based"
	HoldingRegCount  int       `json:"holding_register_count"`
	CoilCount        int       `json:"coil_count"`
	ResponseTimeMs   float64   `json:"response_time_ms"`      // mean RTT
	ResponseJitterMs float64   `json:"response_jitter_ms"`    // RTT standard deviation
	Status           string    `json:"status"`                // "online" or "offline"
	FirstSeen        time.Time `json:"first_seen"`
	LastSeen         time.Time `json:"last_seen"`
	LastGoodPollTime time.Time `json:"last_good_poll_time"`   // [OT-REVIEW] last error-free poll
	ViaGateway       string    `json:"via_gateway,omitempty"` // gateway asset ID, if any
	EnvironmentName  string    `json:"environment"`
	ConfigDesc       string    `json:"config_description"`
	Protocol         string    `json:"protocol"`              // always "modbus-tcp"
	Category         string    `json:"category"`
	PurdueLevel      string    `json:"purdue_level"`

	// Register snapshot -- not serialized in asset list, returned only by
	// the /api/assets/{id}/registers endpoint.
	latestHolding []uint16
	latestCoils   []bool
}

// LatestHolding returns a copy of the most recent holding register snapshot.
// Returns nil if no poll has completed for this asset.
func (a *Asset) LatestHolding() []uint16 {
	if a.latestHolding == nil {
		return nil
	}
	out := make([]uint16, len(a.latestHolding))
	copy(out, a.latestHolding)
	return out
}

// LatestCoils returns a copy of the most recent coil snapshot.
// Returns nil if no poll has completed for this asset.
func (a *Asset) LatestCoils() []bool {
	if a.latestCoils == nil {
		return nil
	}
	out := make([]bool, len(a.latestCoils))
	copy(out, a.latestCoils)
	return out
}

// Inventory provides concurrent-safe storage of discovered assets.
// PROTOTYPE-DEBT: [td-inventory-021] In-memory only; no persistence across restarts.
// All discovered state is rebuilt each startup via DiscoverAll.
type Inventory struct {
	mu     sync.RWMutex
	assets map[string]*Asset
}

// NewInventory creates an empty Inventory.
func NewInventory() *Inventory {
	return &Inventory{
		assets: make(map[string]*Asset),
	}
}

// Upsert inserts a new asset or updates an existing one by asset ID.
// The full asset record is replaced on update.
func (inv *Inventory) Upsert(asset *Asset) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	existing, ok := inv.assets[asset.ID]
	if ok {
		// Preserve discovery timestamps and register snapshot across updates.
		asset.FirstSeen = existing.FirstSeen
		asset.latestHolding = existing.latestHolding
		asset.latestCoils = existing.latestCoils
	}
	inv.assets[asset.ID] = asset
}

// Get retrieves an asset by ID. Returns (nil, false) if not found.
func (inv *Inventory) Get(id string) (*Asset, bool) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	a, ok := inv.assets[id]
	return a, ok
}

// List returns all assets sorted by ID. The returned slice contains copies
// of the internal records; callers must not modify the Asset values.
func (inv *Inventory) List() []*Asset {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	out := make([]*Asset, 0, len(inv.assets))
	for _, a := range inv.assets {
		cp := *a
		out = append(out, &cp)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// UpdateRegisters stores the latest register snapshot for an asset and
// updates the last good poll time. No-op if the asset ID is not found.
func (inv *Inventory) UpdateRegisters(id string, holding []uint16, coils []bool, pollTime time.Time) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	a, ok := inv.assets[id]
	if !ok {
		return
	}

	h := make([]uint16, len(holding))
	copy(h, holding)
	a.latestHolding = h

	c := make([]bool, len(coils))
	copy(c, coils)
	a.latestCoils = c

	a.LastGoodPollTime = pollTime
}

// SetStatus updates the online/offline status and last-seen timestamp for an asset.
// No-op if the asset ID is not found.
func (inv *Inventory) SetStatus(id string, status string, lastSeen time.Time) {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	a, ok := inv.assets[id]
	if !ok {
		return
	}
	a.Status = status
	a.LastSeen = lastSeen
}

// Count returns the total number of assets in the inventory.
func (inv *Inventory) Count() int {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return len(inv.assets)
}
