// Command plant is the entry point for the OT Simulator plant process.
// It loads a design layer environment definition, initializes structured logging,
// and coordinates graceful shutdown on SIGINT or SIGTERM.
//
// Usage:
//
//	plant [--environment <path>] [--log-level <level>] [--health]
//
// Flags:
//
//	--environment  Path to a design layer environment directory containing environment.yaml
//	               (default: ../design/environments/greenfield-water-mfg, relative to plant/)
//	--log-level    Log verbosity: debug | info | warn | error (default: info)
//	--health       Perform a health check: TCP connect to all four Modbus ports and exit 0/1.
//	               Used as the Docker container HEALTHCHECK command.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rustybrownlee/ot-simulator/plant/internal/config"
	"github.com/rustybrownlee/ot-simulator/plant/internal/logging"
	"github.com/rustybrownlee/ot-simulator/plant/internal/process"
	mbserver "github.com/rustybrownlee/ot-simulator/plant/internal/protocol/modbus"
)

const (
	shutdownTimeout    = 30 * time.Second
	defaultEnvironment = "../design/environments/greenfield-water-mfg"
	defaultLogLevel    = "info"
	healthCheckTimeout = 2 * time.Second
)

// modbusPorts lists every Modbus TCP port the plant binary must serve.
// The health check verifies all four sides: water treatment (5020-5022) and
// manufacturing floor (5030). Checking only water treatment would silently pass
// if the manufacturing side failed to bind -- obscuring the core educational contrast.
var modbusPorts = []string{"5020", "5021", "5022", "5030"}

func main() {
	var health bool
	var envPath string
	var logLevel string

	flag.BoolVar(&health, "health", false,
		"perform a container health check and exit 0 (healthy) or 1 (unhealthy)")
	flag.StringVar(&envPath, "environment", defaultEnvironment,
		"path to design layer environment directory containing environment.yaml")
	flag.StringVar(&logLevel, "log-level", defaultLogLevel,
		"log verbosity: debug | info | warn | error")
	flag.Parse()

	if health {
		if err := healthCheck(); err != nil {
			fmt.Fprintf(os.Stderr, "plant: health check failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	resolved, logger, err := initialize(envPath)
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

// healthCheck attempts a TCP connection to each Modbus port with a 2s timeout.
// Returns an error describing the first port that is unreachable.
func healthCheck() error {
	for _, port := range modbusPorts {
		addr := net.JoinHostPort("localhost", port)
		conn, err := net.DialTimeout("tcp", addr, healthCheckTimeout)
		if err != nil {
			return fmt.Errorf("port %s not reachable: %w", port, err)
		}
		conn.Close()
	}
	return nil
}

// initialize loads the design layer environment and constructs the logger.
func initialize(envPath string) (*config.ResolvedEnvironment, logging.Logger, error) {
	resolved, err := config.LoadEnvironment(envPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load environment %q: %w", envPath, err)
	}

	logger := logging.NewLogger()

	return resolved, logger, nil
}

// run starts subsystems, blocks until an OS signal is received, then performs graceful shutdown.
func run(resolved *config.ResolvedEnvironment, logger logging.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sm, err := mbserver.NewServerManager(resolved, logger)
	if err != nil {
		return fmt.Errorf("create modbus server manager: %w", err)
	}

	if err := sm.Start(); err != nil {
		return fmt.Errorf("start modbus servers: %w", err)
	}

	engine := buildEngine(sm.Stores(), logger)
	go engine.Run(ctx)

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

	return shutdown(sm, logger)
}

// buildEngine creates the SimulationEngine and registers a ProcessModel for each
// placement store. The register map variant determines which process model to instantiate.
// Unknown variants are logged and skipped; the engine will tick only known models.
func buildEngine(stores []mbserver.PlacementStore, logger logging.Logger) *process.SimulationEngine {
	engine := process.NewSimulationEngine(logger)

	for _, ps := range stores {
		m := buildModel(ps, logger)
		if m != nil {
			engine.AddModel(m)
		}
	}

	return engine
}

// buildModel selects and constructs the appropriate ProcessModel for a placement.
// Returns nil for unknown variants (they have no process model in this SOW).
func buildModel(ps mbserver.PlacementStore, logger logging.Logger) process.ProcessModel {
	switch ps.RegisterMapVariant {
	case "water-intake":
		return process.NewIntakeModel(ps.Store, nil)
	case "water-treatment":
		return process.NewTreatmentModel(ps.Store, nil)
	case "water-distribution":
		return process.NewDistributionModel(ps.Store, nil)
	case "mfg-line-a":
		return process.NewLineAModel(ps.Store, nil)
	case "mfg-cooling":
		return process.NewCoolingModel(ps.Store, nil)
	case "serial-gateway":
		return process.NewGatewayModel(ps.Store, nil)
	default:
		logger.Debug("no process model for variant, simulation skipped",
			"placement", ps.PlacementID,
			"variant", ps.RegisterMapVariant,
		)
		return nil
	}
}

// shutdown performs ordered cleanup with a 30-second deadline.
// The simulation engine goroutine is stopped via context cancellation before this
// function is called (run() defers cancel(), which propagates to engine.Run(ctx)).
func shutdown(sm *mbserver.ServerManager, logger logging.Logger) error {
	logger.Info("graceful shutdown initiated", "timeout_seconds", int(shutdownTimeout.Seconds()))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		sm.Stop()
	}()

	select {
	case <-done:
		logger.Info("graceful shutdown complete")
		return nil
	case <-shutdownCtx.Done():
		return fmt.Errorf("shutdown exceeded %s deadline", shutdownTimeout)
	}
}
