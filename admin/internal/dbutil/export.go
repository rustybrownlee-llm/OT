package dbutil

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// ExportOptions controls which events are exported and how.
type ExportOptions struct {
	Format   string     // "csv" or "json"
	DeviceID string     // filter by device_id (empty = all)
	After    *time.Time // inclusive lower bound on timestamp
	Before   *time.Time // exclusive upper bound on timestamp
	Output   io.Writer  // destination writer
}

// exportRow mirrors the events table schema for export.
type exportRow struct {
	ID           string
	Timestamp    string
	SrcAddr      string
	DstAddr      string
	UnitID       int64
	FuncCode     int64
	FuncName     string
	AddrStart    sql.NullInt64
	AddrCount    sql.NullInt64
	IsWrite      int64
	Success      int64
	Exception    sql.NullInt64
	ResponseUs   sql.NullInt64
	DeviceID     sql.NullString
	DeviceName   sql.NullString
	EnvID        sql.NullString
	WriteValues  sql.NullString
}

// csvHeaders defines the column order for CSV export.
// The write_values column is the most forensically significant for OT security
// analysis; it stores the actual register values written during Modbus write operations.
var csvHeaders = []string{
	"id", "timestamp", "src_addr", "dst_addr", "unit_id",
	"func_code", "func_name", "addr_start", "addr_count",
	"is_write", "success", "exception", "response_us",
	"device_id", "device_name", "env_id", "write_values",
}

// maxExportRows is the maximum number of rows returned by Export.
const maxExportRows = 100_000

// Export queries events from the database and writes them in the specified format.
// Returns the number of rows exported and whether the limit was reached.
func Export(dbPath string, opts ExportOptions) (int, bool, error) {
	db, err := openReadOnly(dbPath)
	if err != nil {
		return 0, false, err
	}
	defer db.Close()

	query, args := buildExportQuery(opts)
	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, false, fmt.Errorf("querying events for export: %w", err)
	}
	defer rows.Close()

	switch opts.Format {
	case "json":
		return writeJSON(rows, opts.Output)
	default:
		return writeCSV(rows, opts.Output)
	}
}

// buildExportQuery constructs the SELECT with optional WHERE filters.
func buildExportQuery(opts ExportOptions) (string, []any) {
	base := "SELECT id, timestamp, src_addr, dst_addr, unit_id, " +
		"func_code, func_name, addr_start, addr_count, " +
		"is_write, success, exception, response_us, " +
		"device_id, device_name, env_id, write_values " +
		"FROM events"

	var clauses []string
	var args []any

	if opts.DeviceID != "" {
		clauses = append(clauses, "device_id = ?")
		args = append(args, opts.DeviceID)
	}
	if opts.After != nil {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, opts.After.UTC().Format(time.RFC3339Nano))
	}
	if opts.Before != nil {
		clauses = append(clauses, "timestamp < ?")
		args = append(args, opts.Before.UTC().Format(time.RFC3339Nano))
	}

	q := base
	for i, clause := range clauses {
		if i == 0 {
			q += " WHERE " + clause
		} else {
			q += " AND " + clause
		}
	}
	q += fmt.Sprintf(" ORDER BY timestamp ASC LIMIT %d", maxExportRows+1)
	return q, args
}

// scanRow reads one row from a query result into an exportRow struct.
func scanRow(rows *sql.Rows) (*exportRow, error) {
	var r exportRow
	err := rows.Scan(
		&r.ID, &r.Timestamp, &r.SrcAddr, &r.DstAddr,
		&r.UnitID, &r.FuncCode, &r.FuncName,
		&r.AddrStart, &r.AddrCount,
		&r.IsWrite, &r.Success,
		&r.Exception, &r.ResponseUs,
		&r.DeviceID, &r.DeviceName, &r.EnvID,
		&r.WriteValues,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning event row: %w", err)
	}
	return &r, nil
}

// rowToStrings converts an exportRow to a CSV string slice.
func rowToStrings(r *exportRow) []string {
	return []string{
		r.ID,
		r.Timestamp,
		r.SrcAddr,
		r.DstAddr,
		fmt.Sprintf("%d", r.UnitID),
		fmt.Sprintf("%d", r.FuncCode),
		r.FuncName,
		nullInt64Str(r.AddrStart),
		nullInt64Str(r.AddrCount),
		fmt.Sprintf("%d", r.IsWrite),
		fmt.Sprintf("%d", r.Success),
		nullInt64Str(r.Exception),
		nullInt64Str(r.ResponseUs),
		nullStringVal(r.DeviceID),
		nullStringVal(r.DeviceName),
		nullStringVal(r.EnvID),
		nullStringVal(r.WriteValues),
	}
}

// rowToMap converts an exportRow to a map for JSON serialization.
func rowToMap(r *exportRow) map[string]any {
	return map[string]any{
		"id": r.ID, "timestamp": r.Timestamp,
		"src_addr": r.SrcAddr, "dst_addr": r.DstAddr,
		"unit_id": r.UnitID, "func_code": r.FuncCode, "func_name": r.FuncName,
		"addr_start": nullInt64Val(r.AddrStart), "addr_count": nullInt64Val(r.AddrCount),
		"is_write": r.IsWrite, "success": r.Success,
		"exception": nullInt64Val(r.Exception), "response_us": nullInt64Val(r.ResponseUs),
		"device_id": nullStringVal(r.DeviceID), "device_name": nullStringVal(r.DeviceName),
		"env_id": nullStringVal(r.EnvID), "write_values": nullStringVal(r.WriteValues),
	}
}

// writeCSV writes all rows from a sql.Rows result as CSV to w.
func writeCSV(rows *sql.Rows, w io.Writer) (int, bool, error) {
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeaders); err != nil {
		return 0, false, fmt.Errorf("writing CSV header: %w", err)
	}

	count := 0
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return count, false, err
		}
		count++
		if count > maxExportRows {
			cw.Flush()
			return maxExportRows, true, cw.Error()
		}
		if err := cw.Write(rowToStrings(r)); err != nil {
			return count - 1, false, fmt.Errorf("writing CSV row: %w", err)
		}
	}
	cw.Flush()
	return count, false, cw.Error()
}

// writeJSON writes all rows from a sql.Rows result as a JSON array to w.
func writeJSON(rows *sql.Rows, w io.Writer) (int, bool, error) {
	var records []map[string]any
	count := 0

	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return count, false, err
		}
		count++
		if count > maxExportRows {
			return maxExportRows, true, encodeJSON(records[:maxExportRows], w)
		}
		records = append(records, rowToMap(r))
	}

	return count, false, encodeJSON(records, w)
}

// encodeJSON writes v as indented JSON to w.
func encodeJSON(v any, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

// nullInt64Str returns the string representation of a NullInt64, or "" for NULL.
func nullInt64Str(n sql.NullInt64) string {
	if !n.Valid {
		return ""
	}
	return fmt.Sprintf("%d", n.Int64)
}

// nullInt64Val returns the int64 value or nil for NULL.
func nullInt64Val(n sql.NullInt64) any {
	if !n.Valid {
		return nil
	}
	return n.Int64
}

// nullStringVal returns the string value or "" for NULL.
func nullStringVal(n sql.NullString) string {
	if !n.Valid {
		return ""
	}
	return n.String
}
