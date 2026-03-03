package eventstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Register the modernc pure-Go SQLite driver under the "sqlite" driver name.
	// Imported for side-effects only; all access is through database/sql.
	_ "modernc.org/sqlite"
)

// maxQueryLimit caps the number of rows returned by a single Query call.
// Protects against accidental full-table scans in the dashboard.
const maxQueryLimit = 10000

// insertSQL is the parameterized INSERT statement for a single event row.
// Uses named columns to make the statement self-documenting and resilient to
// column-order changes in the schema.
const insertSQL = `
INSERT INTO events
    (id, timestamp, src_addr, dst_addr, unit_id, func_code, func_name,
     addr_start, addr_count, is_write, success, exception, response_us,
     device_id, device_name, env_id, write_values)
VALUES
    (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

// Store provides persistent storage for Modbus transaction events.
// It wraps a SQLite database with WAL mode for concurrent read/write access
// from the polling goroutine (writes) and the dashboard goroutine (reads).
//
// PROTOTYPE-DEBT: [td-events-060] SQLite is single-writer; concurrent inserts
// from multiple goroutines are serialized by the sql.DB connection pool.
// Acceptable for educational scale. If contention becomes measurable in Beta
// 0.7+, add a write-ahead buffer goroutine.
type Store struct {
	db         *sql.DB
	insertStmt *sql.Stmt
}

// New opens (or creates) the SQLite database at dbPath. On first open it
// creates the parent directory, enables WAL journal mode, applies the schema,
// and prepares the insert statement.
//
// Use ":memory:" as dbPath for in-process test databases that require no disk
// I/O and are automatically cleaned up when the Store is closed.
func New(dbPath string) (*Store, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, fmt.Errorf("eventstore: creating database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("eventstore: opening database %q: %w", dbPath, err)
	}

	if err := configureDB(db); err != nil {
		db.Close()
		return nil, err
	}

	if err := createSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("eventstore: creating schema: %w", err)
	}

	stmt, err := db.Prepare(insertSQL)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("eventstore: preparing insert statement: %w", err)
	}

	return &Store{db: db, insertStmt: stmt}, nil
}

// configureDB applies WAL mode and connection pool settings to a newly opened
// database. WAL mode allows the dashboard to read while the poller writes.
func configureDB(db *sql.DB) error {
	// SQLite WAL mode: single writer does not block concurrent readers.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("eventstore: enabling WAL mode: %w", err)
	}
	// Limit to one writer connection to avoid SQLITE_BUSY under WAL mode.
	db.SetMaxOpenConns(1)
	return nil
}

// Close releases the prepared insert statement and closes the database.
// Call Close when the store is no longer needed (typically on monitor shutdown).
func (s *Store) Close() error {
	if err := s.insertStmt.Close(); err != nil {
		return fmt.Errorf("eventstore: closing insert statement: %w", err)
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("eventstore: closing database: %w", err)
	}
	return nil
}

// Insert stores a single TransactionEvent. The event must have a non-empty ID.
// Returns an error if the insert fails (e.g., duplicate ID, constraint violation).
func (s *Store) Insert(ctx context.Context, e *TransactionEvent) error {
	writeValues, err := marshalWriteDetail(e.WriteDetail)
	if err != nil {
		return fmt.Errorf("eventstore: marshaling write detail: %w", err)
	}

	_, err = s.insertStmt.ExecContext(ctx,
		e.ID,
		e.Timestamp.UTC().Format(time.RFC3339Nano),
		e.SrcAddr,
		e.DstAddr,
		int(e.UnitID),
		int(e.FunctionCode),
		e.FunctionName,
		int(e.AddressStart),
		int(e.AddressCount),
		boolToInt(e.IsWrite),
		boolToInt(e.Success),
		nullableUint8(e.ExceptionCode, !e.Success),
		e.ResponseTimeUs,
		nullableString(e.DeviceID),
		nullableString(e.DeviceName),
		nullableString(e.EnvID),
		writeValues,
	)
	if err != nil {
		return fmt.Errorf("eventstore: inserting event %q: %w", e.ID, err)
	}
	return nil
}

// InsertBatch stores multiple events in a single SQLite transaction.
// More efficient than calling Insert in a loop for bulk loads (e.g., 100 events).
// The entire batch is rolled back if any single insert fails.
func (s *Store) InsertBatch(ctx context.Context, events []*TransactionEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("eventstore: beginning batch transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck -- rollback on failure is intentional

	batchStmt := tx.StmtContext(ctx, s.insertStmt)
	defer batchStmt.Close()

	for _, e := range events {
		if err := insertWithStmt(ctx, batchStmt, e); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("eventstore: committing batch transaction: %w", err)
	}
	return nil
}

// insertWithStmt executes an insert using the provided prepared statement.
// Used by InsertBatch to reuse the statement within a transaction.
func insertWithStmt(ctx context.Context, stmt *sql.Stmt, e *TransactionEvent) error {
	writeValues, err := marshalWriteDetail(e.WriteDetail)
	if err != nil {
		return fmt.Errorf("eventstore: marshaling write detail for event %q: %w", e.ID, err)
	}

	_, err = stmt.ExecContext(ctx,
		e.ID,
		e.Timestamp.UTC().Format(time.RFC3339Nano),
		e.SrcAddr,
		e.DstAddr,
		int(e.UnitID),
		int(e.FunctionCode),
		e.FunctionName,
		int(e.AddressStart),
		int(e.AddressCount),
		boolToInt(e.IsWrite),
		boolToInt(e.Success),
		nullableUint8(e.ExceptionCode, !e.Success),
		e.ResponseTimeUs,
		nullableString(e.DeviceID),
		nullableString(e.DeviceName),
		nullableString(e.EnvID),
		writeValues,
	)
	if err != nil {
		return fmt.Errorf("eventstore: inserting batch event %q: %w", e.ID, err)
	}
	return nil
}

// Query returns events matching the filter options, ordered by timestamp
// descending (newest first). Returns an empty slice (not nil) when no events
// match. The Limit field is capped at maxQueryLimit (10000).
func (s *Store) Query(ctx context.Context, opts FilterOptions) ([]*TransactionEvent, error) {
	q, args := buildQuery("SELECT id, timestamp, src_addr, dst_addr, unit_id, "+
		"func_code, func_name, addr_start, addr_count, is_write, success, "+
		"exception, response_us, device_id, device_name, env_id, write_values "+
		"FROM events", opts)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("eventstore: querying events: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// Count returns the number of events matching the filter options.
// Uses the same filter logic as Query for consistent pagination support.
func (s *Store) Count(ctx context.Context, opts FilterOptions) (int64, error) {
	q, args := buildQuery("SELECT COUNT(*) FROM events", opts)

	var count int64
	if err := s.db.QueryRowContext(ctx, q, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("eventstore: counting events: %w", err)
	}
	return count, nil
}

// Prune deletes events older than olderThan (exclusive upper bound on keep).
// Returns the number of rows deleted. Safe to call concurrently with Query;
// WAL mode prevents read-blocking during the delete operation.
func (s *Store) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	cutoff := olderThan.UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM events WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("eventstore: pruning events older than %s: %w",
			olderThan.Format(time.RFC3339), err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("eventstore: reading prune rows affected: %w", err)
	}
	return n, nil
}

// buildQuery constructs the SQL query string and argument slice from
// a base SELECT clause and the provided FilterOptions. Filters are combined
// with AND logic. Appends ORDER BY, LIMIT, and OFFSET clauses.
func buildQuery(base string, opts FilterOptions) (string, []any) {
	var clauses []string
	var args []any

	if opts.After != nil {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, opts.After.UTC().Format(time.RFC3339Nano))
	}
	if opts.Before != nil {
		clauses = append(clauses, "timestamp < ?")
		args = append(args, opts.Before.UTC().Format(time.RFC3339Nano))
	}
	if opts.DeviceID != nil {
		clauses = append(clauses, "device_id = ?")
		args = append(args, *opts.DeviceID)
	}
	if opts.FuncCode != nil {
		clauses = append(clauses, "func_code = ?")
		args = append(args, int(*opts.FuncCode))
	}
	if opts.IsWrite != nil {
		clauses = append(clauses, "is_write = ?")
		args = append(args, boolToInt(*opts.IsWrite))
	}
	if opts.Success != nil {
		clauses = append(clauses, "success = ?")
		args = append(args, boolToInt(*opts.Success))
	}

	q := base
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}

	// COUNT queries do not use ORDER BY / LIMIT / OFFSET.
	if !strings.HasPrefix(base, "SELECT COUNT") {
		q += " ORDER BY timestamp DESC"
		limit := opts.Limit
		if limit <= 0 || limit > maxQueryLimit {
			limit = maxQueryLimit
		}
		q += fmt.Sprintf(" LIMIT %d", limit)
		if opts.Offset > 0 {
			q += fmt.Sprintf(" OFFSET %d", opts.Offset)
		}
	}

	return q, args
}

// scanEvents reads all rows from a query result and returns TransactionEvent
// pointers. Handles NULL columns for optional fields.
func scanEvents(rows *sql.Rows) ([]*TransactionEvent, error) {
	var events []*TransactionEvent

	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("eventstore: scanning event row: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("eventstore: iterating event rows: %w", err)
	}
	if events == nil {
		events = []*TransactionEvent{}
	}
	return events, nil
}

// rowScanResult holds the raw column values scanned from a single event row.
// Separating the scan variables from the TransactionEvent construction keeps
// scanEvent and applyNullables each under 60 lines.
type rowScanResult struct {
	e              TransactionEvent
	tsStr          string
	unitID         int
	funcCode       int
	addrStart      int
	addrCount      int
	isWrite        int
	success        int
	exception      sql.NullInt64
	responseUs     sql.NullInt64
	deviceID       sql.NullString
	deviceName     sql.NullString
	envID          sql.NullString
	writeValuesStr sql.NullString
}

// scanEvent scans a single row into a TransactionEvent. Delegates nullable
// field assignment to applyNullables.
func scanEvent(rows *sql.Rows) (*TransactionEvent, error) {
	var r rowScanResult

	if err := rows.Scan(
		&r.e.ID, &r.tsStr, &r.e.SrcAddr, &r.e.DstAddr,
		&r.unitID, &r.funcCode, &r.e.FunctionName,
		&r.addrStart, &r.addrCount,
		&r.isWrite, &r.success,
		&r.exception, &r.responseUs,
		&r.deviceID, &r.deviceName, &r.envID,
		&r.writeValuesStr,
	); err != nil {
		return nil, err
	}

	ts, err := time.Parse(time.RFC3339Nano, r.tsStr)
	if err != nil {
		return nil, fmt.Errorf("parsing timestamp %q: %w", r.tsStr, err)
	}
	r.e.Timestamp = ts
	r.e.UnitID = uint8(r.unitID)
	r.e.FunctionCode = uint8(r.funcCode)
	r.e.AddressStart = uint16(r.addrStart)
	r.e.AddressCount = uint16(r.addrCount)
	r.e.IsWrite = intToBool(r.isWrite)
	r.e.Success = intToBool(r.success)

	if err := applyNullables(&r.e, r); err != nil {
		return nil, err
	}
	return &r.e, nil
}

// applyNullables copies nullable column values from the scan result into the
// TransactionEvent. NULL columns are left at their zero values.
func applyNullables(e *TransactionEvent, r rowScanResult) error {
	if r.exception.Valid {
		e.ExceptionCode = uint8(r.exception.Int64)
	}
	if r.responseUs.Valid {
		e.ResponseTimeUs = r.responseUs.Int64
	}
	if r.deviceID.Valid {
		e.DeviceID = r.deviceID.String
	}
	if r.deviceName.Valid {
		e.DeviceName = r.deviceName.String
	}
	if r.envID.Valid {
		e.EnvID = r.envID.String
	}
	if r.writeValuesStr.Valid && r.writeValuesStr.String != "" {
		detail, err := unmarshalWriteDetail(r.writeValuesStr.String)
		if err != nil {
			return fmt.Errorf("unmarshaling write_values: %w", err)
		}
		e.WriteDetail = detail
	}
	return nil
}

// marshalWriteDetail serializes a WriteDetail to JSON for SQLite storage.
// Returns nil (SQL NULL) when detail is nil (read operations).
//
// PROTOTYPE-DEBT: [td-events-065] WriteDetail stored as JSON text column.
// Query performance for value extraction requires JSON1 extension.
// Not expected to need resolution; JSON queries are rare in the educational case.
func marshalWriteDetail(detail *WriteDetail) (any, error) {
	if detail == nil {
		return nil, nil
	}
	data, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// unmarshalWriteDetail deserializes a WriteDetail from its JSON string form.
func unmarshalWriteDetail(s string) (*WriteDetail, error) {
	var d WriteDetail
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// boolToInt converts a Go bool to the SQLite integer representation (1/0).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// intToBool converts a SQLite integer (1/0) back to a Go bool.
func intToBool(i int) bool {
	return i != 0
}

// nullableString returns a nil interface (SQL NULL) for an empty string,
// or the string value otherwise. Keeps optional text columns clean.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullableUint8 returns a nil interface (SQL NULL) when store is false,
// or the uint8 value when store is true. Used for ExceptionCode (only
// meaningful when Success is false).
func nullableUint8(v uint8, store bool) any {
	if !store {
		return nil
	}
	return int(v)
}
