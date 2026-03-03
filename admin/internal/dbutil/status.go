// Package dbutil implements direct SQLite access for the admin CLI's db commands.
// All read operations open the database in read-only mode (?mode=ro).
// Only prune opens the database in read-write mode.
//
// PROTOTYPE-DEBT: [td-admin-091] Event store queries reimplemented outside
// monitoring/internal/eventstore/. Schema changes must be reflected in both places.
// Future: extract shared query types or use a schema-driven query builder.
package dbutil

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// DBStatus holds the statistics reported by "admin db status".
type DBStatus struct {
	EventCount        int64
	FileSizeBytes     int64
	OldestEvent       *time.Time
	NewestEvent       *time.Time
	RetentionDays     int
	PruneableEvents   int64
}

// Stats opens the SQLite database at dbPath in read-only mode and returns
// event count, file size, oldest/newest event timestamps, and pruneable count.
func Stats(dbPath string, retentionDays int) (*DBStatus, error) {
	info, err := os.Stat(dbPath)
	if err != nil {
		return nil, fmt.Errorf("stat database file %q: %w", dbPath, err)
	}
	fileSize := info.Size()

	db, err := openReadOnly(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	count, err := queryCount(db)
	if err != nil {
		return nil, err
	}

	oldest, newest, err := queryTimestampRange(db)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	pruneable, err := queryCountBefore(db, cutoff)
	if err != nil {
		return nil, err
	}

	return &DBStatus{
		EventCount:      count,
		FileSizeBytes:   fileSize,
		OldestEvent:     oldest,
		NewestEvent:     newest,
		RetentionDays:   retentionDays,
		PruneableEvents: pruneable,
	}, nil
}

// openReadOnly opens the SQLite file at path in read-only mode.
func openReadOnly(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("opening database read-only %q: %w", path, err)
	}
	return db, nil
}

// queryCount returns the total number of rows in the events table.
func queryCount(db *sql.DB) (int64, error) {
	var n int64
	err := db.QueryRow("SELECT COUNT(*) FROM events").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting events: %w", err)
	}
	return n, nil
}

// queryTimestampRange returns the oldest and newest event timestamps.
// Returns nil pointers when the table is empty.
func queryTimestampRange(db *sql.DB) (*time.Time, *time.Time, error) {
	var oldestStr, newestStr sql.NullString

	err := db.QueryRow(
		"SELECT MIN(timestamp), MAX(timestamp) FROM events",
	).Scan(&oldestStr, &newestStr)
	if err != nil {
		return nil, nil, fmt.Errorf("querying timestamp range: %w", err)
	}

	oldest, err := parseNullTime(oldestStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing oldest timestamp: %w", err)
	}

	newest, err := parseNullTime(newestStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing newest timestamp: %w", err)
	}

	return oldest, newest, nil
}

// queryCountBefore returns the number of events older than cutoff.
func queryCountBefore(db *sql.DB, cutoff time.Time) (int64, error) {
	cutoffStr := cutoff.UTC().Format(time.RFC3339Nano)
	var n int64
	err := db.QueryRow(
		"SELECT COUNT(*) FROM events WHERE timestamp < ?", cutoffStr,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting pruneable events: %w", err)
	}
	return n, nil
}

// parseNullTime converts a NullString timestamp to a *time.Time.
// Returns nil when the value is SQL NULL (empty table).
func parseNullTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
