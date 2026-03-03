// Package cli implements the admin CLI command handlers.
// Each command function accepts a Globals config and a string slice of
// command-specific arguments, parsed internally using flag.NewFlagSet.
package cli

import (
	"fmt"
	"io"
	"strings"
)

// Globals holds resolved global configuration passed to every command.
// Values reflect CLI flag > environment variable > default precedence.
type Globals struct {
	DesignDir  string // --design-dir / OTS_DESIGN_DIR
	ConfigPath string // --config / OTS_MONITOR_CONFIG
	DBPath     string // --db (overrides config event_db_path)
	APIAddr    string // --api-addr / OTS_API_ADDR
	PlantPorts string // --plant-ports / OTS_HEALTH_PORTS
}

// TablePrinter writes aligned tabular output to a writer.
// Column widths are computed from header lengths; content is not truncated.
type TablePrinter struct {
	w       io.Writer
	headers []string
	widths  []int
	rows    [][]string
}

// NewTablePrinter creates a TablePrinter with the given column headers.
func NewTablePrinter(w io.Writer, headers ...string) *TablePrinter {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &TablePrinter{w: w, headers: headers, widths: widths}
}

// AddRow appends a row to the table. Values are coerced to strings.
// Columns wider than the current max are tracked for alignment.
func (t *TablePrinter) AddRow(cols ...string) {
	for i, c := range cols {
		if i < len(t.widths) && len(c) > t.widths[i] {
			t.widths[i] = len(c)
		}
	}
	t.rows = append(t.rows, cols)
}

// Print writes the headers followed by all rows to the writer.
// Columns are left-aligned and padded to the widest value in each column.
func (t *TablePrinter) Print() {
	t.printRow(t.headers)
	sep := make([]string, len(t.headers))
	for i, w := range t.widths {
		sep[i] = strings.Repeat("-", w)
	}
	t.printRow(sep)
	for _, row := range t.rows {
		t.printRow(row)
	}
}

// printRow writes a single row with column padding.
func (t *TablePrinter) printRow(cols []string) {
	parts := make([]string, len(t.widths))
	for i, w := range t.widths {
		v := ""
		if i < len(cols) {
			v = cols[i]
		}
		parts[i] = fmt.Sprintf("%-*s", w, v)
	}
	fmt.Fprintln(t.w, strings.Join(parts, "  "))
}

// PrintKV writes a key-value pair to w with aligned values.
// Intended for status displays like "db status" and "config view".
// The key column is right-padded to keyWidth characters.
func PrintKV(w io.Writer, keyWidth int, key, value string) {
	fmt.Fprintf(w, "  %-*s  %s\n", keyWidth, key+":", value)
}
