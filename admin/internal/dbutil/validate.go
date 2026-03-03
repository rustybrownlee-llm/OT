package dbutil

import (
	"fmt"
	"os"
)

// IntegrityResult holds the output of a database integrity check.
type IntegrityResult struct {
	IntegrityOK      bool
	IntegrityErrors  []string
	ForeignKeyErrors []string
}

// ValidateDB runs PRAGMA integrity_check and PRAGMA foreign_key_check against
// the database at dbPath (opened read-only). Returns nil error when the database
// is reachable; errors in IntegrityErrors indicate database corruption.
func ValidateDB(dbPath string) (*IntegrityResult, error) {
	// Pre-check existence to produce a clear error instead of the SQLite
	// "out of memory" error that results from opening a missing file read-only.
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("database file not found: %s", dbPath)
	}

	integrityErrs, err := runIntegrityCheck(dbPath)
	if err != nil {
		return nil, err
	}

	fkErrs, err := runForeignKeyCheck(dbPath)
	if err != nil {
		return nil, err
	}

	return &IntegrityResult{
		IntegrityOK:      len(integrityErrs) == 0,
		IntegrityErrors:  integrityErrs,
		ForeignKeyErrors: fkErrs,
	}, nil
}

// runIntegrityCheck executes PRAGMA integrity_check and collects non-"ok" results.
func runIntegrityCheck(dbPath string) ([]string, error) {
	db, err := openReadOnly(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("PRAGMA integrity_check")
	if err != nil {
		return nil, fmt.Errorf("PRAGMA integrity_check: %w", err)
	}
	defer rows.Close()

	var errs []string
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err != nil {
			return nil, fmt.Errorf("scanning integrity_check row: %w", err)
		}
		if msg != "ok" {
			errs = append(errs, msg)
		}
	}
	return errs, rows.Err()
}

// runForeignKeyCheck executes PRAGMA foreign_key_check and collects violations.
func runForeignKeyCheck(dbPath string) ([]string, error) {
	db, err := openReadOnly(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		return nil, fmt.Errorf("PRAGMA foreign_key_check: %w", err)
	}
	defer rows.Close()

	var violations []string
	for rows.Next() {
		var table, parent string
		var rowid, fkid int64
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return nil, fmt.Errorf("scanning foreign_key_check row: %w", err)
		}
		violations = append(violations,
			fmt.Sprintf("table=%s rowid=%d parent=%s fkid=%d", table, rowid, parent, fkid))
	}
	return violations, rows.Err()
}
