// Package api implements the HTTP API for the monitoring module.
// Routes are served on port 8091 (configurable via api_addr in monitor.yaml).
// All responses are JSON with CORS headers for consumption by the SOW-014.0 dashboard.
//
// Routes:
//
//	GET /api/health                       -- per-endpoint connectivity status
//	GET /api/assets                       -- discovered device inventory
//	GET /api/assets/{id}/registers        -- latest register values for a device
//	GET /api/alerts                       -- anomaly alerts with optional filters
//	GET /api/alerts/{id}                  -- single alert detail
//	POST /api/alerts/{id}/acknowledge     -- acknowledge an alert
//	GET /api/baselines                    -- per-device baseline status
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/alert"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/baseline"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/inventory"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/poller"
)

// healthResponse is the JSON schema for GET /api/health.
type healthResponse struct {
	Status          string            `json:"status"`
	Timestamp       time.Time         `json:"timestamp"`
	Endpoints       map[string]string `json:"endpoints"`
	LastPollCycle   time.Time         `json:"last_poll_cycle"`
	PollIntervalSec int               `json:"poll_interval_seconds"`
	DevicesOnline   int               `json:"devices_online"`
	DevicesOffline  int               `json:"devices_offline"`
}

// registerResponse is the JSON schema for GET /api/assets/{id}/registers.
type registerResponse struct {
	ID               string    `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	HoldingRegisters []uint16  `json:"holding_registers"`
	Coils            []bool    `json:"coils"`
}

// baselineRegisterStats is the per-register statistics payload in GET /api/baselines.
type baselineRegisterStats struct {
	Address float64 `json:"address"`
	Mean    float64 `json:"mean"`
	StdDev  float64 `json:"stddev"`
	Min     uint16  `json:"min"`
	Max     uint16  `json:"max"`
	Type    string  `json:"type"`
}

// baselineResponseTimeStats is the response time statistics in GET /api/baselines.
type baselineResponseTimeStats struct {
	MeanMs   float64 `json:"mean_ms"`
	StdDevMs float64 `json:"stddev_ms"`
}

// baselineEntry is one device's entry in the GET /api/baselines response.
type baselineEntry struct {
	DeviceID        string                    `json:"device_id"`
	Status          string                    `json:"status"`
	SampleCount     int                       `json:"sample_count"`
	RequiredSamples int                       `json:"required_samples"`
	Registers       []baselineRegisterStats   `json:"registers"`
	ResponseTime    baselineResponseTimeStats `json:"response_time"`
}

// NewRouter creates a chi router with all API routes registered.
// It requires the inventory, poll state, baseline engine and alert store.
func NewRouter(inv *inventory.Inventory, state *poller.PollState, pollIntervalSec int,
	engine *baseline.Engine, alertStore *alert.Store) http.Handler {

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Get("/api/health", healthHandler(inv, state, pollIntervalSec))
	r.Get("/api/assets", assetsHandler(inv))
	r.Get("/api/assets/{id}/registers", assetRegistersHandler(inv))

	// SOW-013.0: alert and baseline endpoints.
	r.Get("/api/alerts", alertsHandler(alertStore))
	r.Get("/api/alerts/{id}", alertByIDHandler(alertStore))
	r.Post("/api/alerts/{id}/acknowledge", acknowledgeAlertHandler(alertStore))
	r.Get("/api/baselines", baselinesHandler(engine))

	return r
}

// corsMiddleware adds CORS headers to every response for dashboard consumption.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// healthHandler returns JSON with per-endpoint connectivity status.
// Status is "healthy" (all online), "degraded" (some online), or "unhealthy" (none online).
func healthHandler(inv *inventory.Inventory, state *poller.PollState, pollIntervalSec int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assets := inv.List()

		endpoints := make(map[string]string)
		onlineCount, offlineCount := 0, 0

		for _, a := range assets {
			epStatus, exists := endpoints[a.Endpoint]
			if !exists {
				epStatus = a.Status
			}
			// An endpoint is "online" if any of its unit IDs responded.
			if a.Status == inventory.StatusOnline {
				epStatus = inventory.StatusOnline
			}
			endpoints[a.Endpoint] = epStatus

			if a.Status == inventory.StatusOnline {
				onlineCount++
			} else {
				offlineCount++
			}
		}

		overallStatus := overallHealth(onlineCount, offlineCount)

		resp := healthResponse{
			Status:          overallStatus,
			Timestamp:       time.Now().UTC(),
			Endpoints:       endpoints,
			LastPollCycle:   state.LastCycleTime().UTC(),
			PollIntervalSec: pollIntervalSec,
			DevicesOnline:   onlineCount,
			DevicesOffline:  offlineCount,
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// assetsHandler returns a JSON array of all discovered assets.
func assetsHandler(inv *inventory.Inventory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assets := inv.List()
		if assets == nil {
			assets = []*inventory.Asset{}
		}
		writeJSON(w, http.StatusOK, assets)
	}
}

// assetRegistersHandler returns the latest register snapshot for a single asset.
// Returns 404 if the asset ID is not found in the inventory.
func assetRegistersHandler(inv *inventory.Inventory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		a, ok := inv.Get(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "asset not found",
				"id":    id,
			})
			return
		}

		holding := a.LatestHolding()
		if holding == nil {
			holding = []uint16{}
		}
		coils := a.LatestCoils()
		if coils == nil {
			coils = []bool{}
		}

		resp := registerResponse{
			ID:               a.ID,
			Timestamp:        time.Now().UTC(),
			HoldingRegisters: holding,
			Coils:            coils,
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// alertsHandler returns a filtered list of alerts ordered by timestamp descending.
// Supports query parameters: severity, device, active.
func alertsHandler(store *alert.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filters := parseAlertFilters(r)
		alerts := store.List(filters...)
		if alerts == nil {
			alerts = []*alert.Alert{}
		}
		writeJSON(w, http.StatusOK, alerts)
	}
}

// alertByIDHandler returns a single alert by ID. Returns 404 if not found.
func alertByIDHandler(store *alert.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		a, ok := store.Get(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "alert not found",
				"id":    id,
			})
			return
		}
		writeJSON(w, http.StatusOK, a)
	}
}

// acknowledgeAlertHandler sets the acknowledged flag for an alert.
// Returns the updated alert, or 404 if the ID is not found.
func acknowledgeAlertHandler(store *alert.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		a, ok := store.Acknowledge(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "alert not found",
				"id":    id,
			})
			return
		}
		writeJSON(w, http.StatusOK, a)
	}
}

// baselinesHandler returns per-device baseline status and statistics.
// For established baselines, per-register statistics are included.
// For devices still in the learning period, registers is an empty array.
func baselinesHandler(engine *baseline.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		baselines := engine.GetBaselines()
		result := make(map[string]baselineEntry, len(baselines))

		for id, db := range baselines {
			entry := baselineEntry{
				DeviceID:        db.DeviceID,
				Status:          db.Status,
				SampleCount:     db.SampleCount,
				RequiredSamples: db.RequiredSamples,
				Registers:       []baselineRegisterStats{},
				ResponseTime: baselineResponseTimeStats{
					MeanMs:   db.ResponseTimeMean,
					StdDevMs: db.ResponseTimeStdDev,
				},
			}

			if db.Status == baseline.StatusEstablished {
				for i, rs := range db.RegisterStats {
					entry.Registers = append(entry.Registers, baselineRegisterStats{
						Address: float64(i),
						Mean:    rs.Mean,
						StdDev:  rs.StdDev,
						Min:     rs.Min,
						Max:     rs.Max,
						Type:    rs.Type,
					})
				}
			}

			result[id] = entry
		}

		writeJSON(w, http.StatusOK, result)
	}
}

// parseAlertFilters builds a slice of FilterFuncs from the request's query string.
// Supported parameters: severity, device, active.
func parseAlertFilters(r *http.Request) []alert.FilterFunc {
	var filters []alert.FilterFunc

	if sev := r.URL.Query().Get("severity"); sev != "" {
		filters = append(filters, alert.BySeverity(sev))
	}
	if dev := r.URL.Query().Get("device"); dev != "" {
		filters = append(filters, alert.ByDevice(dev))
	}
	if activeStr := r.URL.Query().Get("active"); activeStr != "" {
		if active, err := strconv.ParseBool(activeStr); err == nil {
			filters = append(filters, alert.ByActive(active))
		}
	}

	return filters
}

// overallHealth derives the aggregate health status from device counts.
func overallHealth(online, offline int) string {
	total := online + offline
	switch {
	case total == 0:
		return "unhealthy"
	case online == 0:
		return "unhealthy"
	case offline > 0:
		return "degraded"
	default:
		return "healthy"
	}
}

// writeJSON serializes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		// Cannot change status code after WriteHeader; log only.
		_ = err
	}
}
