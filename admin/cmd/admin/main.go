// Package main is the entry point for the admin CLI.
// It provides platform administration commands for the OT simulator,
// including health checks, database management, config validation,
// baseline management, and a web server skeleton.
//
// Command routing uses the standard flag package -- no external CLI frameworks.
// Global flags are parsed before command dispatch with precedence:
// CLI flag > environment variable > default value.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rustybrownlee/ot-simulator/admin/internal/cli"
)

// defaultPlantPorts is the list of Modbus TCP ports covered by the quickstart
// environment. This covers PLCs, gateways, and modems across all simulated
// environments. See td-admin-092: this list is hardcoded and must be updated
// as new environments are added.
//
// PROTOTYPE-DEBT: [td-admin-092] Default plant port list is hardcoded.
// Adding new plant environments requires updating this default.
// Future: derive port list from design layer environment YAML files.
const defaultPlantPorts = "5020,5021,5022,5030,5040,5041,5042,5043,5050,5051,5052,5062,5063,5064"

// GlobalConfig holds values resolved from CLI flags and environment variables.
// Flag precedence: CLI flag > environment variable > default value.
type GlobalConfig struct {
	DesignDir   string
	ConfigPath  string
	DBPath      string
	APIAddr     string
	PlantPorts  string
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	globals := parseGlobalFlags(os.Args[1:])

	// The first non-flag argument is the command. Global flags must appear
	// before the command, but we find the command by skipping known flags.
	cmd := findCommand(os.Args[1:])
	if cmd == "" {
		printUsage()
		os.Exit(1)
	}

	args := commandArgs(os.Args[1:], cmd)

	switch cmd {
	case "health":
		cli.RunHealth(toCLIGlobals(globals), args)
	case "db":
		cli.RunDB(toCLIGlobals(globals), args)
	case "config":
		cli.RunConfig(toCLIGlobals(globals), args)
	case "baseline":
		cli.RunBaseline(toCLIGlobals(globals), args)
	case "design":
		cli.RunDesign(toCLIGlobals(globals), args)
	case "web":
		cli.RunWeb(toCLIGlobals(globals), args)
	default:
		fmt.Fprintf(os.Stderr, "admin: unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

// parseGlobalFlags parses global flags from the argument slice.
// Environment variable fallback is applied when a flag is not explicitly set.
func parseGlobalFlags(args []string) *GlobalConfig {
	fs := flag.NewFlagSet("admin-globals", flag.ContinueOnError)
	fs.Usage = func() {}

	designDir := fs.String("design-dir", envOr("OTS_DESIGN_DIR", "./design"), "")
	configPath := fs.String("config", envOr("OTS_MONITOR_CONFIG", "./config/monitor.yaml"), "")
	dbPath := fs.String("db", "", "")
	apiAddr := fs.String("api-addr", envOr("OTS_API_ADDR", "localhost:8091"), "")
	plantPorts := fs.String("plant-ports", envOr("OTS_HEALTH_PORTS", defaultPlantPorts), "")

	// Parse ignoring errors -- unknown flags (the command name, subcommands)
	// are passed through to the command handlers.
	_ = fs.Parse(args)

	return &GlobalConfig{
		DesignDir:  *designDir,
		ConfigPath: *configPath,
		DBPath:     *dbPath,
		APIAddr:    *apiAddr,
		PlantPorts: *plantPorts,
	}
}

// envOr returns the value of the environment variable name, or fallback if unset.
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// findCommand locates the first non-flag argument in args.
// Global flags (--flag or -flag) and their values are skipped.
func findCommand(args []string) string {
	skip := false
	globalFlagsTakingValue := map[string]bool{
		"design-dir": true, "config": true, "db": true,
		"api-addr": true, "plant-ports": true,
	}

	for _, a := range args {
		if skip {
			skip = false
			continue
		}
		if strings.HasPrefix(a, "--") || strings.HasPrefix(a, "-") {
			name := strings.TrimLeft(a, "-")
			if idx := strings.IndexByte(name, '='); idx >= 0 {
				// --flag=value form: no skip needed
				continue
			}
			if globalFlagsTakingValue[name] {
				skip = true
			}
			continue
		}
		return a
	}
	return ""
}

// commandArgs returns the arguments slice starting after the command name.
func commandArgs(args []string, cmd string) []string {
	for i, a := range args {
		if a == cmd {
			return args[i+1:]
		}
	}
	return nil
}

// toCLIGlobals converts a GlobalConfig to the cli package's Globals type.
func toCLIGlobals(g *GlobalConfig) cli.Globals {
	return cli.Globals{
		DesignDir:  g.DesignDir,
		ConfigPath: g.ConfigPath,
		DBPath:     g.DBPath,
		APIAddr:    g.APIAddr,
		PlantPorts: g.PlantPorts,
	}
}

// printUsage prints the command reference to stderr.
func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: admin [global-flags] <command> [command-flags]

Commands:
  health                      Platform health summary (ports, API, database)
  db status                   Event store statistics
  db validate                 Database integrity check
  db prune [--older-than N]   Delete events older than N days
  db export [--format csv|json] [--output PATH] [--device ID]
            [--after TIME] [--before TIME]
                              Export events to CSV or JSON
  config view [PATH]          Display parsed monitoring configuration
  config validate [PATH]      Validate monitoring configuration file
  baseline status             Per-device baseline learning status
  baseline reset [--device ID] [--force]
                              Trigger baseline re-learning
  design validate <path> [--verbose] [--cross-refs-only]
                              Validate a design YAML file or environment directory
  design list                 List all design layer elements (devices, networks, environments)
  web [--addr ADDR]           Start admin web server (default :8095)

Global Flags:
  --design-dir PATH    Design layer directory (env: OTS_DESIGN_DIR, default: ./design)
  --config PATH        Monitor config file (env: OTS_MONITOR_CONFIG, default: ./config/monitor.yaml)
  --db PATH            Event database file (default: from config event_db_path)
  --api-addr ADDR      Monitoring API address (env: OTS_API_ADDR, default: localhost:8091)
  --plant-ports LIST   Comma-separated Modbus ports to health-check (env: OTS_HEALTH_PORTS)`)
}
