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
//	monitor [--config <path>] [--addr <host:port>]
//
// Flags:
//
//	--config  Path to the monitoring YAML configuration file (default: config/monitor.yaml)
//	--addr    Override the bind address for the monitoring dashboard
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
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "monitor: %v\n", err)
		os.Exit(1)
	}
}

// run parses flags, then blocks until an OS signal triggers clean exit.
func run() error {
	var configPath string
	var addr string

	flag.StringVar(&configPath, "config", defaultConfig, "path to monitoring YAML configuration file")
	flag.StringVar(&addr, "addr", "", "override bind address for the monitoring dashboard")
	flag.Parse()

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
