// Package alert provides the alert model and thread-safe in-memory alert store
// for the behavioral anomaly detection system.
//
// Alert IDs are deterministic: the same rule, device, and register address
// always produce the same 16-character hex ID. This prevents duplicate alerts
// when the same condition persists across multiple polling cycles.
//
// PROTOTYPE-DEBT: [td-alert-021] Alerts are in-memory only. On restart the
// store is empty and all baselines re-enter the learning period. Persistent
// storage (SQLite or time-series DB) is deferred to a future milestone.
package alert

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Alert represents a detected anomaly condition.
type Alert struct {
	ID              string     `json:"id"`
	Timestamp       time.Time  `json:"timestamp"`
	Severity        string     `json:"severity"`         // "critical", "high", "warning", "info"
	RuleID          string     `json:"rule_id"`
	DeviceID        string     `json:"device_id"`
	Description     string     `json:"description"`
	RegisterAddress int        `json:"register_address"` // -1 if not register-specific
	ExpectedValue   string     `json:"expected_value"`
	ActualValue     string     `json:"actual_value"`
	Acknowledged    bool       `json:"acknowledged"`
	Active          bool       `json:"active"`
	ClearedAt       *time.Time `json:"cleared_at,omitempty"`
}

// FilterFunc is a predicate over an Alert used by Store.List.
type FilterFunc func(*Alert) bool

// BySeverity returns a FilterFunc that matches alerts with the given severity.
func BySeverity(severity string) FilterFunc {
	return func(a *Alert) bool { return a.Severity == severity }
}

// ByDevice returns a FilterFunc that matches alerts for the given device ID.
func ByDevice(deviceID string) FilterFunc {
	return func(a *Alert) bool { return a.DeviceID == deviceID }
}

// ByActive returns a FilterFunc that matches alerts by their active status.
func ByActive(active bool) FilterFunc {
	return func(a *Alert) bool { return a.Active == active }
}

// AlertID returns a deterministic 16-character hex identifier derived from
// the SHA-256 hash of the concatenation of ruleID, deviceID, and registerAddr.
// The same inputs always produce the same ID, preventing duplicate alerts.
func AlertID(ruleID, deviceID string, registerAddr int) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s:%s:%d", ruleID, deviceID, registerAddr)
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// Store is a thread-safe ring buffer of alerts with O(1) lookup by ID.
// When the store reaches capacity, the oldest cleared (inactive) alert is
// evicted. If no cleared alerts exist, the oldest active alert is evicted.
type Store struct {
	mu     sync.RWMutex
	alerts []*Alert      // ring buffer backing array; positions are stable until eviction
	index  map[string]int // alert ID -> position in alerts slice
	size   int            // capacity
	count  int            // number of valid entries [0, size]
}

// NewStore creates a Store with the given maximum capacity.
// Panics if maxAlerts is less than 1.
func NewStore(maxAlerts int) *Store {
	if maxAlerts < 1 {
		panic("alert.Store: maxAlerts must be >= 1")
	}
	return &Store{
		alerts: make([]*Alert, maxAlerts),
		index:  make(map[string]int, maxAlerts),
		size:   maxAlerts,
	}
}

// GetOrCreate returns the existing alert with the given id, or creates a new
// one by calling createFn and inserting it into the store.
// If the store is at capacity, an old alert is evicted to make room.
// GetOrCreate is idempotent: calling it twice with the same id returns the
// same alert without invoking createFn on the second call.
func (s *Store) GetOrCreate(id string, createFn func() *Alert) *Alert {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pos, ok := s.index[id]; ok {
		return s.alerts[pos]
	}

	a := createFn()
	s.insert(a)
	return a
}

// Get returns the alert with the given ID and true, or nil and false if not found.
func (s *Store) Get(id string) (*Alert, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pos, ok := s.index[id]
	if !ok {
		return nil, false
	}
	return s.alerts[pos], true
}

// List returns a copy of alerts matching all provided filters, ordered by
// Timestamp descending (newest first). With no filters all alerts are returned.
func (s *Store) List(filters ...FilterFunc) []*Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*Alert
	for i := 0; i < s.count; i++ {
		a := s.alerts[i]
		if a == nil {
			continue
		}
		if matchAll(a, filters) {
			cp := *a
			out = append(out, &cp)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	return out
}

// Acknowledge sets the Acknowledged flag to true for the alert with the given ID.
// Returns the updated alert and true on success, or nil and false if not found.
func (s *Store) Acknowledge(id string) (*Alert, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pos, ok := s.index[id]
	if !ok {
		return nil, false
	}
	s.alerts[pos].Acknowledged = true
	cp := *s.alerts[pos]
	return &cp, true
}

// Clear marks the alert with the given ID as inactive and sets ClearedAt.
// No-op if the ID is not found.
func (s *Store) Clear(id string, clearedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pos, ok := s.index[id]
	if !ok {
		return
	}
	a := s.alerts[pos]
	a.Active = false
	t := clearedAt
	a.ClearedAt = &t
}

// Count returns the number of valid entries in the store.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.count
}

// insert adds an alert to the store. If the store is full, the oldest cleared
// (inactive) alert is evicted. If none are cleared, the oldest active alert
// is evicted. Caller must hold the write lock.
func (s *Store) insert(a *Alert) {
	if s.count < s.size {
		s.alerts[s.count] = a
		s.index[a.ID] = s.count
		s.count++
		return
	}

	// Store is full. Find a slot to evict.
	evictPos := s.findEvictCandidate()
	old := s.alerts[evictPos]
	delete(s.index, old.ID)
	s.alerts[evictPos] = a
	s.index[a.ID] = evictPos
}

// findEvictCandidate returns the index of the alert to evict.
// Prefers the oldest cleared alert; falls back to the oldest active alert.
// Caller must hold the write lock.
func (s *Store) findEvictCandidate() int {
	oldestClearedPos := -1
	oldestActivePos := 0

	for i := 0; i < s.count; i++ {
		a := s.alerts[i]
		if a == nil {
			continue
		}
		if !a.Active {
			if oldestClearedPos == -1 ||
				a.Timestamp.Before(s.alerts[oldestClearedPos].Timestamp) {
				oldestClearedPos = i
			}
		} else {
			if a.Timestamp.Before(s.alerts[oldestActivePos].Timestamp) {
				oldestActivePos = i
			}
		}
	}

	if oldestClearedPos != -1 {
		return oldestClearedPos
	}
	return oldestActivePos
}

// matchAll returns true if the alert satisfies all provided filters.
func matchAll(a *Alert, filters []FilterFunc) bool {
	for _, f := range filters {
		if !f(a) {
			return false
		}
	}
	return true
}
