package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/rustybrownlee/ot-simulator/admin/internal/configparse"
)

// RunConfig dispatches to the correct config subcommand: view, validate.
func RunConfig(g Globals, args []string) {
	if len(args) == 0 {
		printConfigUsage()
		os.Exit(1)
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "view":
		runConfigView(g, rest)
	case "validate":
		runConfigValidate(g, rest)
	default:
		fmt.Fprintf(os.Stderr, "admin config: unknown subcommand %q\n\n", sub)
		printConfigUsage()
		os.Exit(1)
	}
}

// runConfigView parses the monitoring config and prints a formatted summary.
func runConfigView(g Globals, args []string) {
	path := resolveConfigPath(g, args)

	cfg, err := configparse.ParseLenient(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin config view: %v\n", err)
		os.Exit(1)
	}

	printConfigSummary(cfg, path)
}

// runConfigValidate validates the monitoring config and reports field errors.
func runConfigValidate(g Globals, args []string) {
	path := resolveConfigPath(g, args)

	_, err := configparse.Parse(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Configuration is valid.")
}

// resolveConfigPath returns the config file path from args (positional first arg)
// or falls back to the global --config value.
func resolveConfigPath(g Globals, args []string) string {
	if len(args) > 0 && args[0] != "" {
		return args[0]
	}
	return g.ConfigPath
}

// printConfigSummary writes a formatted key-value display of the parsed config.
func printConfigSummary(cfg *configparse.Config, path string) {
	const kw = 28
	fmt.Printf("Configuration: %s\n\n", path)

	PrintKV(os.Stdout, kw, "log_level", cfg.LogLevel)
	PrintKV(os.Stdout, kw, "poll_interval_seconds", strconv.Itoa(cfg.PollIntervalSeconds))
	PrintKV(os.Stdout, kw, "gateway_request_delay_ms", strconv.Itoa(cfg.GatewayRequestDelayMs))
	PrintKV(os.Stdout, kw, "api_addr", cfg.APIAddr)
	PrintKV(os.Stdout, kw, "dashboard_addr", cfg.DashboardAddr)
	PrintKV(os.Stdout, kw, "baseline_learning_cycles", strconv.Itoa(cfg.BaselineLearningCycles))
	PrintKV(os.Stdout, kw, "ring_buffer_size", strconv.Itoa(cfg.RingBufferSize))
	PrintKV(os.Stdout, kw, "max_alerts", strconv.Itoa(cfg.MaxAlerts))
	PrintKV(os.Stdout, kw, "event_db_path", cfg.EventDBPath)
	PrintKV(os.Stdout, kw, "event_retention_days", strconv.Itoa(cfg.EventRetentionDays))

	fmt.Println()
	printSyslogSummary(cfg, kw)
	fmt.Println()
	printEnvironmentsSummary(cfg)
}

// printSyslogSummary prints syslog configuration fields.
func printSyslogSummary(cfg *configparse.Config, kw int) {
	fmt.Println("  Syslog:")
	PrintKV(os.Stdout, kw, "  enabled", strconv.FormatBool(cfg.Syslog.Enabled))
	if cfg.Syslog.Enabled {
		PrintKV(os.Stdout, kw, "  target", cfg.Syslog.Target)
		PrintKV(os.Stdout, kw, "  protocol", cfg.Syslog.Protocol)
		PrintKV(os.Stdout, kw, "  facility", cfg.Syslog.Facility)
		PrintKV(os.Stdout, kw, "  format", cfg.Syslog.Format)
	}
}

// printEnvironmentsSummary prints a summary of configured environments.
func printEnvironmentsSummary(cfg *configparse.Config) {
	fmt.Printf("  Environments (%d):\n", len(cfg.Environments))
	for _, env := range cfg.Environments {
		fmt.Printf("    %s (%s): %d endpoint(s)\n",
			env.Name, env.Address, len(env.Endpoints))
		for _, ep := range env.Endpoints {
			epType := "PLC"
			if ep.Gateway {
				epType = "Gateway"
			}
			fmt.Printf("      port %-5d  %s  unit_id=%-3d  %s\n",
				ep.Port, epType, ep.UnitID, ep.Description)
		}
	}
}

// printConfigUsage prints config subcommand usage to stderr.
func printConfigUsage() {
	fmt.Fprintln(os.Stderr, `Usage: admin config <subcommand> [path]

Subcommands:
  view [PATH]      Display parsed monitoring configuration with defaults applied
  validate [PATH]  Validate configuration file and report field-level errors

PATH defaults to the --config global flag or OTS_MONITOR_CONFIG environment variable.`)
}
