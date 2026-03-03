// Command monitor is the entry point for the OT Simulator monitoring process.
// It observes the plant environment externally -- via Modbus TCP polling --
// without importing any plant packages directly.
//
// This separation enforces the monitoring architecture defined in ADR-005:
// monitoring tools must work within real OT constraints, interacting only
// over the network as actual security tools would.
//
// Usage:
//
//	monitor [--config <path>] [--addr <host:port>] [--dashboard-addr <host:port>] [--health]
//
// Flags:
//
//	--config         Path to the monitoring YAML configuration file (default: /config/monitor.yaml)
//	--addr           Override the bind address for the monitoring API (overrides config api_addr)
//	--dashboard-addr Override the bind address for the dashboard web UI (overrides config dashboard_addr)
//	--design-dir     Override the design library directory path (default: /design)
//	--health         Perform a health check: HTTP GET /api/health, exit 0 (healthy/degraded) or 1.
//	                 Used as the Docker container HEALTHCHECK command.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/api"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/baseline"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/config"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/dashboard"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/discovery"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/inventory"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/poller"
)

const (
	defaultConfig    = "/config/monitor.yaml"
	defaultDesignDir = "/design"
)

// healthCheckTimeout is the HTTP timeout for the --health flag check.
const healthCheckTimeout = 3 * time.Second

func main() {
	var healthMode bool
	var configPath string
	var addrOverride string
	var dashboardAddrOverride string
	var designDirOverride string

	flag.BoolVar(&healthMode, "health", false,
		"perform a container health check and exit 0 (healthy/degraded) or 1 (unhealthy)")
	flag.StringVar(&configPath, "config", defaultConfig, "path to monitoring YAML configuration file")
	flag.StringVar(&addrOverride, "addr", "", "override bind address for the monitoring API")
	flag.StringVar(&dashboardAddrOverride, "dashboard-addr", "", "override bind address for the dashboard web UI")
	flag.StringVar(&designDirOverride, "design-dir", "", "override design library directory path")
	flag.Parse()

	if healthMode {
		if err := healthCheck(configPath, addrOverride); err != nil {
			fmt.Fprintf(os.Stderr, "monitor: health check failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := run(configPath, addrOverride, dashboardAddrOverride, designDirOverride); err != nil {
		fmt.Fprintf(os.Stderr, "monitor: %v\n", err)
		os.Exit(1)
	}
}

// healthCheck performs a real HTTP health check against the running monitor API.
func healthCheck(configPath, addrOverride string) error {
	apiAddr := resolveAPIAddr(configPath, addrOverride)
	if apiAddr == "" {
		apiAddr = ":8091"
	}
	host := apiAddr
	if len(host) > 0 && host[0] == ':' {
		host = "localhost" + host
	}
	url := "http://" + host + "/api/health"

	client := &http.Client{Timeout: healthCheckTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("decode health response: %w", err)
	}

	status, _ := body["status"].(string)
	if status == "unhealthy" {
		return fmt.Errorf("monitor status is unhealthy: no devices reachable")
	}
	return nil
}

// resolveAPIAddr extracts the API address from config, then applies CLI override.
func resolveAPIAddr(configPath, addrOverride string) string {
	if addrOverride != "" {
		return addrOverride
	}
	cfg, err := config.Parse(configPath)
	if err != nil {
		return ":8091"
	}
	return cfg.APIAddr
}

// run is the main application loop: parse config, open event store, start API
// and dashboard, discover devices, start polling, and wait for shutdown signal.
func run(configPath, addrOverride, dashboardAddrOverride, designDirOverride string) error {
	cfg, err := config.Parse(configPath)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// CLI flags override config file values.
	if addrOverride != "" {
		cfg.APIAddr = addrOverride
	}
	if dashboardAddrOverride != "" {
		cfg.DashboardAddr = dashboardAddrOverride
	}

	designDir := defaultDesignDir
	if designDirOverride != "" {
		designDir = designDirOverride
	}

	slog.Info("monitor starting",
		"config", configPath,
		"api_addr", cfg.APIAddr,
		"dashboard_addr", cfg.DashboardAddr,
		"poll_interval_seconds", cfg.PollIntervalSeconds,
		"environments", len(cfg.Environments))

	// Open the event store. Schema is applied on first open.
	store, err := eventstore.New(cfg.EventDBPath)
	if err != nil {
		return fmt.Errorf("open event store: %w", err)
	}
	slog.Info("event store opened",
		"path", cfg.EventDBPath,
		"retention_days", cfg.EventRetentionDays)

	// Load design library at startup (read-only after initialization).
	lib, err := dashboard.LoadDesignLibrary(designDir)
	if err != nil {
		store.Close() //nolint:errcheck -- best-effort close on startup failure
		return fmt.Errorf("load design library: %w", err)
	}

	inv := inventory.NewInventory()
	state := &poller.PollState{}

	// Baseline engine (SOW-013.0): learns device behavior and detects anomalies.
	engine := baseline.NewEngine(cfg.BaselineLearningCycles, cfg.RingBufferSize, cfg.MaxAlerts)

	// Start the HTTP API before discovery so /api/health is immediately available.
	router := api.NewRouter(inv, state, cfg.PollIntervalSeconds, engine, engine.AlertStore())
	srv := &http.Server{
		Addr:    cfg.APIAddr,
		Handler: router,
	}

	srvErr := make(chan error, 2)
	go func() {
		slog.Info("API server listening", "addr", cfg.APIAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			srvErr <- err
		}
	}()

	// Start the dashboard web server. The API client calls the API addr on loopback.
	apiClient := dashboard.NewAPIClient(cfg.APIAddr)
	dash := dashboard.NewDashboard(apiClient, lib)
	dashSrv := &http.Server{
		Addr:    cfg.DashboardAddr,
		Handler: dash.Routes(),
	}
	go func() {
		slog.Info("dashboard server listening", "addr", cfg.DashboardAddr)
		if err := dashSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			srvErr <- err
		}
	}()

	// Context for discovery and polling.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start pruning goroutine (hourly, runs until context is cancelled).
	go runPruner(ctx, store, cfg.EventRetentionDays)

	// Device discovery: probe all configured endpoints.
	slog.Info("starting device discovery")
	disc := discovery.New(cfg, inv)
	if err := disc.DiscoverAll(ctx); err != nil {
		slog.Warn("discovery returned error", "error", err)
	}

	discoveredCount := inv.Count()
	slog.Info("discovery complete", "devices_discovered", discoveredCount)

	// Register all discovered device IDs with the baseline engine before polling
	// starts. Any device that appears after this point triggers the "new device" rule.
	allAssets := inv.List()
	knownIDs := make([]string, 0, len(allAssets))
	for _, a := range allAssets {
		knownIDs = append(knownIDs, a.ID)
	}
	engine.SetKnownDevices(knownIDs)
	slog.Info("baseline engine initialised",
		"known_devices", len(knownIDs),
		"learning_cycles", cfg.BaselineLearningCycles,
		"max_alerts", cfg.MaxAlerts)

	// Start the polling loop in a background goroutine.
	p := poller.New(cfg, inv, state)
	p.SetCycleHook(engine.RecordCycle)
	p.SetEventHook(func(events []*eventstore.TransactionEvent) {
		if err := store.InsertBatch(context.Background(), events); err != nil {
			// [OT-REVIEW] FR-9: Availability over completeness. InsertBatch errors are
			// logged but never crash the monitor or block polling.
			slog.Warn("event store insert failed", "count", len(events), "error", err)
		}
	})

	go func() {
		if err := p.Run(ctx); err != nil {
			slog.Warn("poller exited with error", "error", err)
		}
	}()

	// Wait for shutdown signal or server error.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("received shutdown signal", "signal", sig.String())
	case err := <-srvErr:
		return fmt.Errorf("server error: %w", err)
	}

	slog.Info("shutting down")
	cancel()
	p.Shutdown()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("API server shutdown error", "error", err)
	}
	if err := dashSrv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("dashboard server shutdown error", "error", err)
	}

	// Close event store after poller has stopped to ensure no in-flight inserts.
	if err := store.Close(); err != nil {
		slog.Warn("event store close error", "error", err)
	}

	slog.Info("monitor exited cleanly")
	return nil
}

// runPruner deletes events older than the retention window on an hourly schedule.
// Runs until ctx is cancelled. Prune errors are logged at Warn level and the
// goroutine continues running to retry on the next tick.
//
// PROTOTYPE-DEBT: [td-main-030] Pruner goroutine uses background context for Prune
// calls. If the main context is cancelled during a prune, the operation may be
// interrupted. Not critical: incomplete prune will complete on next hourly tick.
func runPruner(ctx context.Context, store *eventstore.Store, retentionDays int) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	retention := time.Duration(retentionDays) * 24 * time.Hour

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-retention)
			n, err := store.Prune(ctx, cutoff)
			if err != nil {
				slog.Warn("event pruning failed", "error", err)
				continue
			}
			if n > 0 {
				slog.Info("pruned old events", "deleted", n, "retention_days", retentionDays)
			}
		}
	}
}
