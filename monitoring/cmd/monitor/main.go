// Command monitor is the entry point for the OT Simulator monitoring process.
// It observes the plant environment externally -- via Modbus TCP polling and
// network packet capture -- without importing any plant packages directly.
//
// This separation enforces the monitoring architecture defined in ADR-005:
// monitoring tools must work within real OT constraints, interacting only
// over the network as actual security tools would.
//
// Usage:
//
//	monitor [--config <path>] [--addr <host:port>] [--health]
//
// Flags:
//
//	--config  Path to the monitoring YAML configuration file (default: config/monitor.yaml)
//	--addr    Override the bind address for the monitoring dashboard
//	--health  Perform a health check: verify config file is readable and exit 0/1.
//	          Used as the Docker container HEALTHCHECK command.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

const defaultConfig = "config/monitor.yaml"

func main() {
	var health bool
	var configPath string
	var addr string

	flag.BoolVar(&health, "health", false,
		"perform a container health check and exit 0 (healthy) or 1 (unhealthy)")
	flag.StringVar(&configPath, "config", defaultConfig, "path to monitoring YAML configuration file")
	flag.StringVar(&addr, "addr", "", "override bind address for the monitoring dashboard")
	flag.Parse()

	if health {
		if err := healthCheck(configPath); err != nil {
			fmt.Fprintf(os.Stderr, "monitor: health check failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := run(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "monitor: %v\n", err)
		os.Exit(1)
	}
}

// healthCheck verifies the config file is readable. Returns an error if it cannot be opened.
// This confirms the process has access to its required configuration, not that it has crashed.
// TD-012: Upgrade when monitoring has real endpoints to check.
func healthCheck(configPath string) error {
	f, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("config file %q not readable: %w", configPath, err)
	}
	f.Close()
	return nil
}

// run starts the monitoring process, then blocks until an OS signal triggers clean exit.
func run(configPath string) error {
	fmt.Fprintf(os.Stdout, "{\"level\":\"INFO\",\"msg\":\"monitor starting\",\"config\":%q}\n", configPath)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Fprintf(os.Stdout, "{\"level\":\"INFO\",\"msg\":\"monitor ready\"}\n")

	select {
	case sig := <-sigCh:
		fmt.Fprintf(os.Stdout, "{\"level\":\"INFO\",\"msg\":\"received shutdown signal\",\"signal\":%q}\n", sig.String())
	case <-ctx.Done():
		fmt.Fprintf(os.Stdout, "{\"level\":\"INFO\",\"msg\":\"context cancelled\"}\n")
	}

	fmt.Fprintf(os.Stdout, "{\"level\":\"INFO\",\"msg\":\"monitor exited cleanly\"}\n")
	return nil
}
