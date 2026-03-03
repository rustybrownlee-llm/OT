package eventstore

import "database/sql"

// schemaSQL defines the events table and supporting indexes.
//
// Column naming follows ADR-011 conventions:
//   - addr_start / addr_count use protocol-neutral naming per [OT-REVIEW]
//   - device_name is populated by SOW-028.0 when the poller resolves asset names
//   - write_values stores JSON-encoded WriteDetail; NULL for read operations
//
// Indexes support the common dashboard query patterns:
//   - timestamp: time-range filtering and newest-first ordering
//   - device_id: per-device event queries
//   - is_write (partial, WHERE is_write=1): write-only filter for security analysis
//   - func_code: function code distribution queries
//
// CREATE TABLE uses IF NOT EXISTS and CREATE INDEX uses IF NOT EXISTS to
// support idempotent schema initialization on every store open. No migration
// framework -- the schema is append-only for Beta 0.6.
//
// PROTOTYPE-DEBT: [td-events-066] No database migration framework. Schema
// changes require drop-and-recreate. Acceptable due to 7-day retention policy;
// no long-lived data to preserve.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS events (
    id           TEXT PRIMARY KEY,
    timestamp    TEXT NOT NULL,
    src_addr     TEXT NOT NULL,
    dst_addr     TEXT NOT NULL,
    unit_id      INTEGER NOT NULL,
    func_code    INTEGER NOT NULL,
    func_name    TEXT NOT NULL,
    addr_start   INTEGER,
    addr_count   INTEGER,
    is_write     INTEGER NOT NULL DEFAULT 0,
    success      INTEGER NOT NULL DEFAULT 1,
    exception    INTEGER,
    response_us  INTEGER,
    device_id    TEXT,
    device_name  TEXT,
    env_id       TEXT,
    write_values TEXT
);

CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
CREATE INDEX IF NOT EXISTS idx_events_device    ON events(device_id);
CREATE INDEX IF NOT EXISTS idx_events_write     ON events(is_write) WHERE is_write = 1;
CREATE INDEX IF NOT EXISTS idx_events_func      ON events(func_code);
`

// createSchema executes the schema SQL against the provided database connection.
// All statements are idempotent (IF NOT EXISTS). Returns an error if any
// statement fails to execute.
func createSchema(db *sql.DB) error {
	_, err := db.Exec(schemaSQL)
	return err
}
