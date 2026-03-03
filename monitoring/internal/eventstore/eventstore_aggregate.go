package eventstore

import (
	"context"
	"fmt"
	"time"
)

// DeviceStat holds per-device aggregate statistics derived from the event store.
// Used by the /comms page to display the communication matrix.
//
// [OT-REVIEW] In Beta 0.6, all traffic originates from the monitor (single
// Modbus client), so the matrix is a star topology: monitor -> each device.
// DistinctFCs reveals which function codes each device has received, which is
// the highest-value indicator for identifying unexpected write targets.
type DeviceStat struct {
	DeviceID    string
	DeviceName  string
	TotalEvents int64
	WriteEvents int64
	LastSeen    time.Time
	DistinctFCs []uint8 // distinct function codes observed for this device
}

// CommEdgeStat holds communication edge statistics for a source -> destination pair.
//
// [OT-REVIEW] CommEdges() is implemented for Beta 0.7 forward compatibility.
// In Beta 0.6, all traffic originates from the monitor (single source), so the
// /comms page uses DeviceStats() instead. When passive capture introduces
// multi-source traffic in Beta 0.7, CommEdges() will power the NxN matrix.
type CommEdgeStat struct {
	SrcAddr       string
	DstDeviceID   string
	DstDeviceName string
	EventCount    int64
	WriteCount    int64
	LastEvent     time.Time
}

// FCCount holds the event count for a single Modbus function code on one device.
// Used by the FC histogram on the asset detail page.
type FCCount struct {
	FuncCode uint8
	FuncName string
	Count    int64
	IsWrite  bool
}

// deviceStatsSQL queries per-device aggregate statistics.
// MAX(device_name) is required because device_name is not in the GROUP BY clause;
// in practice the value is functionally dependent on device_id (same name for all rows).
const deviceStatsSQL = `
SELECT device_id, MAX(device_name) as device_name, COUNT(*) as total_events,
       SUM(CASE WHEN is_write = 1 THEN 1 ELSE 0 END) as write_events,
       MAX(timestamp) as last_seen
FROM events
GROUP BY device_id
ORDER BY device_id
`

// distinctFCsSQL queries the distinct function codes observed for one device.
// Run once per device after the main DeviceStats query.
const distinctFCsSQL = `
SELECT DISTINCT func_code FROM events WHERE device_id = ? ORDER BY func_code
`

// commEdgesSQL queries communication edge statistics grouped by source -> device.
// MAX(device_name) is required because device_name is not in the GROUP BY clause.
const commEdgesSQL = `
SELECT src_addr, device_id, MAX(device_name) as device_name,
       COUNT(*) as event_count,
       SUM(CASE WHEN is_write = 1 THEN 1 ELSE 0 END) as write_count,
       MAX(timestamp) as last_event
FROM events
GROUP BY src_addr, device_id
ORDER BY event_count DESC
`

// fcDistributionSQL queries function code distribution for a single device.
// Groups by func_code only; MAX() on func_name and is_write because those are
// functionally dependent on func_code (one-to-one mapping from the spec).
const fcDistributionSQL = `
SELECT func_code, MAX(func_name) as func_name, COUNT(*) as count,
       MAX(is_write) as is_write
FROM events
WHERE device_id = ?
GROUP BY func_code
ORDER BY count DESC
`

// DeviceStats returns per-device aggregate statistics from the event store.
// Returns an empty slice (not nil) when no events exist.
//
// PROTOTYPE-DEBT: [td-events-062] Communication graph rebuilt from full event
// query on each page load; no cache. Add materialized view or periodic cache
// in Beta 0.7 if latency exceeds 2s.
func (s *Store) DeviceStats(ctx context.Context) ([]DeviceStat, error) {
	rows, err := s.db.QueryContext(ctx, deviceStatsSQL)
	if err != nil {
		return nil, fmt.Errorf("eventstore: querying device stats: %w", err)
	}
	defer rows.Close()

	var stats []DeviceStat
	for rows.Next() {
		st, err := scanDeviceStat(rows)
		if err != nil {
			return nil, fmt.Errorf("eventstore: scanning device stat row: %w", err)
		}
		stats = append(stats, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("eventstore: iterating device stat rows: %w", err)
	}
	if stats == nil {
		return []DeviceStat{}, nil
	}

	// Populate DistinctFCs for each device via sub-query.
	for i := range stats {
		fcs, err := s.queryDistinctFCs(ctx, stats[i].DeviceID)
		if err != nil {
			return nil, err
		}
		stats[i].DistinctFCs = fcs
	}
	return stats, nil
}

// scanDeviceStat scans a single row from the deviceStatsSQL result.
func scanDeviceStat(rows interface {
	Scan(...any) error
}) (DeviceStat, error) {
	var st DeviceStat
	var nameNull nullableStr
	var lastSeenStr string
	if err := rows.Scan(
		&st.DeviceID, &nameNull, &st.TotalEvents, &st.WriteEvents, &lastSeenStr,
	); err != nil {
		return DeviceStat{}, err
	}
	st.DeviceName = nameNull.val
	ts, err := time.Parse(time.RFC3339Nano, lastSeenStr)
	if err != nil {
		return DeviceStat{}, fmt.Errorf("parsing last_seen timestamp %q: %w", lastSeenStr, err)
	}
	st.LastSeen = ts
	return st, nil
}

// queryDistinctFCs returns the distinct function codes observed for deviceID.
func (s *Store) queryDistinctFCs(ctx context.Context, deviceID string) ([]uint8, error) {
	rows, err := s.db.QueryContext(ctx, distinctFCsSQL, deviceID)
	if err != nil {
		return nil, fmt.Errorf("eventstore: querying distinct FCs for device %q: %w", deviceID, err)
	}
	defer rows.Close()

	var fcs []uint8
	for rows.Next() {
		var fc int
		if err := rows.Scan(&fc); err != nil {
			return nil, fmt.Errorf("eventstore: scanning FC row: %w", err)
		}
		fcs = append(fcs, uint8(fc))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("eventstore: iterating FC rows: %w", err)
	}
	return fcs, nil
}

// CommEdges returns communication edge statistics grouped by source address and
// destination device. Returns an empty slice (not nil) when no events exist.
func (s *Store) CommEdges(ctx context.Context) ([]CommEdgeStat, error) {
	rows, err := s.db.QueryContext(ctx, commEdgesSQL)
	if err != nil {
		return nil, fmt.Errorf("eventstore: querying comm edges: %w", err)
	}
	defer rows.Close()

	var edges []CommEdgeStat
	for rows.Next() {
		edge, err := scanCommEdge(rows)
		if err != nil {
			return nil, fmt.Errorf("eventstore: scanning comm edge row: %w", err)
		}
		edges = append(edges, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("eventstore: iterating comm edge rows: %w", err)
	}
	if edges == nil {
		return []CommEdgeStat{}, nil
	}
	return edges, nil
}

// scanCommEdge scans a single row from the commEdgesSQL result.
func scanCommEdge(rows interface {
	Scan(...any) error
}) (CommEdgeStat, error) {
	var edge CommEdgeStat
	var nameNull nullableStr
	var lastEventStr string
	if err := rows.Scan(
		&edge.SrcAddr, &edge.DstDeviceID, &nameNull,
		&edge.EventCount, &edge.WriteCount, &lastEventStr,
	); err != nil {
		return CommEdgeStat{}, err
	}
	edge.DstDeviceName = nameNull.val
	ts, err := time.Parse(time.RFC3339Nano, lastEventStr)
	if err != nil {
		return CommEdgeStat{}, fmt.Errorf("parsing last_event timestamp %q: %w", lastEventStr, err)
	}
	edge.LastEvent = ts
	return edge, nil
}

// FCDistribution returns the function code distribution for a single device,
// ordered by count descending. Returns an empty slice (not nil) when no events
// exist for the device.
func (s *Store) FCDistribution(ctx context.Context, deviceID string) ([]FCCount, error) {
	rows, err := s.db.QueryContext(ctx, fcDistributionSQL, deviceID)
	if err != nil {
		return nil, fmt.Errorf("eventstore: querying FC distribution for device %q: %w", deviceID, err)
	}
	defer rows.Close()

	var counts []FCCount
	for rows.Next() {
		fc, err := scanFCCount(rows)
		if err != nil {
			return nil, fmt.Errorf("eventstore: scanning FC count row: %w", err)
		}
		counts = append(counts, fc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("eventstore: iterating FC count rows: %w", err)
	}
	if counts == nil {
		return []FCCount{}, nil
	}
	return counts, nil
}

// scanFCCount scans a single row from the fcDistributionSQL result.
// is_write is stored as an integer (0/1); MAX() returns the same type.
func scanFCCount(rows interface {
	Scan(...any) error
}) (FCCount, error) {
	var fc FCCount
	var code int
	var isWriteInt int
	if err := rows.Scan(&code, &fc.FuncName, &fc.Count, &isWriteInt); err != nil {
		return FCCount{}, err
	}
	fc.FuncCode = uint8(code)
	fc.IsWrite = intToBool(isWriteInt)
	return fc, nil
}

// nullableStr is a local helper for scanning nullable TEXT columns returned
// by MAX(). The MAX aggregate can return NULL when the column value is NULL.
// database/sql.NullString provides the same behaviour but is more verbose
// to use in struct literals; this thin wrapper reduces scanner boilerplate.
type nullableStr struct {
	val string
}

// Scan implements the sql.Scanner interface for nullableStr.
func (n *nullableStr) Scan(src any) error {
	if src == nil {
		n.val = ""
		return nil
	}
	switch v := src.(type) {
	case string:
		n.val = v
	case []byte:
		n.val = string(v)
	default:
		n.val = fmt.Sprintf("%v", v)
	}
	return nil
}
