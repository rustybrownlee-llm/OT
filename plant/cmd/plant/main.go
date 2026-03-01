// Command plant is the entry point for the OT Simulator plant process.
// It loads the plant topology configuration, initializes structured logging,
// and coordinates graceful shutdown on SIGINT or SIGTERM.
//
// Usage:
//
//	plant [--config <path>] [--addr <host:port>]
//
// Flags:
//
//	--config  Path to the plant YAML configuration file (default: config/plant.yaml)
//	--addr    Override the default bind address for the plant HTTP interface
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
	shutdownTimeout = 30 * time.Second
	defaultConfig   = "config/plant.yaml"
)

func main() {
	cfg, logger, err := initialize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "plant: initialization failed: %v\n", err)
		os.Exit(1)
	}

	logger.Info("plant simulator starting",
		"facility", cfg.Plant.Name,
		"water_treatment_enabled", cfg.WaterTreatment.Enabled,
		"manufacturing_enabled", cfg.Manufacturing.Enabled,
	)

	if err := run(cfg, logger); err != nil {
		logger.Fatal("plant simulator exited with error", "error", err.Error())
	}
}

// initialize parses flags, loads configuration, and constructs the logger.
func initialize() (*config.Config, logging.Logger, error) {
	var configPath string
	var addr string

	flag.StringVar(&configPath, "config", defaultConfig, "path to plant YAML configuration file")
	flag.StringVar(&addr, "addr", "", "override bind address for the plant HTTP interface")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load config %q: %w", configPath, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validate config: %w", err)
	}

	logger := logging.NewLogger()

	return cfg, logger, nil
}

// run blocks until an OS signal is received, then performs graceful shutdown.
func run(cfg *config.Config, logger logging.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("plant simulator ready", "config", cfg.Plant.Name)

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
