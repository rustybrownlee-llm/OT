package dbutil

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Prune deletes events older than olderThan from the database at dbPath.
// Opens the database in read-write mode (write access required for DELETE).
// Returns the number of rows deleted.
func Prune(dbPath string, olderThan time.Time) (int64, error) {
	db, err := openWrite(dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	cutoff := olderThan.UTC().Format(time.RFC3339Nano)
	res, err := db.Exec("DELETE FROM events WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("deleting events older than %s: %w",
			olderThan.Format(time.RFC3339), err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("reading rows affected: %w", err)
	}
	return n, nil
}

// openWrite opens the SQLite file at path in read-write mode.
func openWrite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database %q: %w", path, err)
	}
	// Enable WAL mode for compatibility with concurrent monitoring reads.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}
	return db, nil
}
