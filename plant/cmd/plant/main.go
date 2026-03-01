// Command plant is the entry point for the OT Simulator plant process.
// It loads a design layer environment definition, initializes structured logging,
// and coordinates graceful shutdown on SIGINT or SIGTERM.
//
// Usage:
//
//	plant [--environment <path>] [--log-level <level>]
//
// Flags:
//
//	--environment  Path to a design layer environment directory containing environment.yaml
//	               (default: ../design/environments/greenfield-water-mfg, relative to plant/)
//	--log-level    Log verbosity: debug | info | warn | error (default: info)
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rustybrownlee/ot-simulator/plant/internal/config"
	"github.com/rustybrownlee/ot-simulator/plant/internal/logging"
)

const (
	shutdownTimeout    = 30 * time.Second
	defaultEnvironment = "../design/environments/greenfield-water-mfg"
	defaultLogLevel    = "info"
)

func main() {
	resolved, logger, err := initialize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "plant: initialization failed: %v\n", err)
		os.Exit(1)
	}

	logger.Info("plant simulator starting",
		"environment", resolved.Env.Environment.Name,
		"placements", len(resolved.Env.Placements),
	)

	if err := run(resolved, logger); err != nil {
		logger.Fatal("plant simulator exited with error", "error", err.Error())
	}
}

// initialize parses flags, loads the design layer environment, and constructs the logger.
func initialize() (*config.ResolvedEnvironment, logging.Logger, error) {
	var envPath string
	var logLevel string

	flag.StringVar(&envPath, "environment", defaultEnvironment,
		"path to design layer environment directory containing environment.yaml")
	flag.StringVar(&logLevel, "log-level", defaultLogLevel,
		"log verbosity: debug | info | warn | error")
	flag.Parse()

	resolved, err := config.LoadEnvironment(envPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load environment %q: %w", envPath, err)
	}

	logger := logging.NewLogger()

	return resolved, logger, nil
}

// run blocks until an OS signal is received, then performs graceful shutdown.
func run(resolved *config.ResolvedEnvironment, logger logging.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("plant simulator ready",
		"environment", resolved.Env.Environment.Name,
	)

	select {
	case sig := <-sigCh:
		logger.Info("received shutdown signal", "signal", sig.String())
	case <-ctx.Done():
		logger.Info("context cancelled")
	}

	return shutdown(logger)
}

// shutdown performs ordered cleanup with a 30-second deadline.
func shutdown(logger logging.Logger) error {
	logger.Info("graceful shutdown initiated", "timeout_seconds", int(shutdownTimeout.Seconds()))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Subsystem shutdown hooks will be added here in subsequent SOWs
		// as Modbus servers, HMI servers, and process engines are implemented.
	}()

	select {
	case <-done:
		logger.Info("graceful shutdown complete")
		return nil
	case <-shutdownCtx.Done():
		return fmt.Errorf("shutdown exceeded %s deadline", shutdownTimeout)
	}
}
