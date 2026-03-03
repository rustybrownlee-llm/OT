package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rustybrownlee/ot-simulator/admin/internal/configparse"
	"github.com/rustybrownlee/ot-simulator/admin/internal/dbutil"
)

// RunDB dispatches to the correct db subcommand: status, validate, prune, export.
func RunDB(g Globals, args []string) {
	if len(args) == 0 {
		printDBUsage()
		os.Exit(1)
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "status":
		runDBStatus(g, rest)
	case "validate":
		runDBValidate(g, rest)
	case "prune":
		runDBPrune(g, rest)
	case "export":
		runDBExport(g, rest)
	default:
		fmt.Fprintf(os.Stderr, "admin db: unknown subcommand %q\n\n", sub)
		printDBUsage()
		os.Exit(1)
	}
}

// runDBStatus prints event store statistics.
func runDBStatus(g Globals, args []string) {
	dbPath := effectiveDBPath(g)
	retentionDays := retentionFromConfig(g)

	stats, err := dbutil.Stats(dbPath, retentionDays)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin db status: %v\n", err)
		os.Exit(1)
	}

	const kw = 24
	PrintKV(os.Stdout, kw, "Event count", fmt.Sprintf("%d", stats.EventCount))
	PrintKV(os.Stdout, kw, "Database size",
		fmt.Sprintf("%.2f MB", float64(stats.FileSizeBytes)/(1024*1024)))
	PrintKV(os.Stdout, kw, "Oldest event", formatTimePtr(stats.OldestEvent))
	PrintKV(os.Stdout, kw, "Newest event", formatTimePtr(stats.NewestEvent))
	PrintKV(os.Stdout, kw, "Retention window",
		fmt.Sprintf("%d days", stats.RetentionDays))
	PrintKV(os.Stdout, kw, "Pruneable events",
		fmt.Sprintf("%d (older than %d days)", stats.PruneableEvents, retentionDays))
	PrintKV(os.Stdout, kw, "Database path", dbPath)
}

// runDBValidate runs database integrity checks.
func runDBValidate(g Globals, args []string) {
	dbPath := effectiveDBPath(g)

	result, err := dbutil.ValidateDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin db validate: %v\n", err)
		os.Exit(1)
	}

	if result.IntegrityOK && len(result.ForeignKeyErrors) == 0 {
		fmt.Println("Database integrity: OK")
		fmt.Println("Foreign key check: OK")
		return
	}

	if !result.IntegrityOK {
		fmt.Println("Database integrity: FAILED")
		for _, e := range result.IntegrityErrors {
			fmt.Printf("  error: %s\n", e)
		}
	} else {
		fmt.Println("Database integrity: OK")
	}

	if len(result.ForeignKeyErrors) > 0 {
		fmt.Println("Foreign key check: FAILED")
		for _, v := range result.ForeignKeyErrors {
			fmt.Printf("  violation: %s\n", v)
		}
	} else {
		fmt.Println("Foreign key check: OK")
	}
	os.Exit(1)
}

// runDBPrune deletes events older than the specified number of days.
func runDBPrune(g Globals, args []string) {
	fs := flag.NewFlagSet("db-prune", flag.ExitOnError)
	olderThan := fs.Int("older-than", 0, "delete events older than N days (default: configured retention)")
	force := fs.Bool("force", false, "skip confirmation prompt")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	days := *olderThan
	if days <= 0 {
		days = retentionFromConfig(g)
	}
	dbPath := effectiveDBPath(g)

	cutoff := time.Now().AddDate(0, 0, -days)

	if !*force {
		fmt.Printf("WARNING: This will delete all events older than %s (%d days).\n",
			cutoff.Format(time.RFC3339), days)
		fmt.Print("Type 'yes' to confirm: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		if strings.TrimSpace(scanner.Text()) != "yes" {
			fmt.Println("Aborted.")
			os.Exit(0)
		}
	}

	n, err := dbutil.Prune(dbPath, cutoff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin db prune: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Deleted %d events older than %d days.\n", n, days)
}

// runDBExport exports events to CSV or JSON.
func runDBExport(g Globals, args []string) {
	fs := flag.NewFlagSet("db-export", flag.ExitOnError)
	format := fs.String("format", "csv", "output format: csv or json")
	outputPath := fs.String("output", "", "output file path (default: stdout)")
	deviceID := fs.String("device", "", "filter by device ID")
	afterStr := fs.String("after", "", "include events at or after this RFC3339 timestamp")
	beforeStr := fs.String("before", "", "include events before this RFC3339 timestamp")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	dbPath := effectiveDBPath(g)
	opts := dbutil.ExportOptions{Format: *format, DeviceID: *deviceID}

	if *afterStr != "" {
		t, err := time.Parse(time.RFC3339, *afterStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "admin db export: invalid --after time %q: %v\n", *afterStr, err)
			os.Exit(1)
		}
		opts.After = &t
	}
	if *beforeStr != "" {
		t, err := time.Parse(time.RFC3339, *beforeStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "admin db export: invalid --before time %q: %v\n", *beforeStr, err)
			os.Exit(1)
		}
		opts.Before = &t
	}

	out, closeOut := openOutput(*outputPath)
	defer closeOut()
	opts.Output = out

	n, limitReached, err := dbutil.Export(dbPath, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin db export: %v\n", err)
		os.Exit(1)
	}

	if limitReached {
		fmt.Fprintf(os.Stderr,
			"WARNING: Export limit reached (%d rows). Use --before/--after to narrow the range.\n", n)
	}
	if *outputPath != "" {
		fmt.Printf("Exported %d events to %s.\n", n, *outputPath)
	}
}

// openOutput returns a writer and closer for the given path.
// If path is empty, returns os.Stdout with a no-op closer.
func openOutput(path string) (*os.File, func()) {
	if path == "" {
		return os.Stdout, func() {}
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin db export: creating output file %q: %v\n", path, err)
		os.Exit(1)
	}
	return f, func() { f.Close() }
}

// retentionFromConfig reads the event_retention_days from the monitor config.
// Returns the default (7) if the config cannot be read.
func retentionFromConfig(g Globals) int {
	cfg, err := configparse.ParseLenient(g.ConfigPath)
	if err != nil {
		return configparse.DefaultEventRetentionDays
	}
	if cfg.EventRetentionDays > 0 {
		return cfg.EventRetentionDays
	}
	return configparse.DefaultEventRetentionDays
}

// formatTimePtr formats a *time.Time for display, returning "none" for nil.
func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "none (empty database)"
	}
	return t.UTC().Format(time.RFC3339)
}

// printDBUsage prints db subcommand usage to stderr.
func printDBUsage() {
	fmt.Fprintln(os.Stderr, `Usage: admin db <subcommand> [flags]

Subcommands:
  status                        Event store statistics
  validate                      Database integrity check
  prune [--older-than N] [--force]
                                Delete events older than N days
  export [--format csv|json] [--output PATH] [--device ID]
         [--after RFC3339] [--before RFC3339]
                                Export events`)
}
