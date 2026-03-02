// Package dashboard implements the web UI served on port 8090.
// It consumes the monitoring API on port 8091 (localhost loopback) to display
// live asset data, register values, and anomaly alerts.
// Design library YAML files are read at startup for the Reference pages.
package dashboard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// APIClient wraps HTTP calls to the monitoring API on port 8091.
// The browser never calls port 8091 directly; all API access is server-side.
type APIClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewAPIClient creates an APIClient pointing at the given API address.
// apiAddr is a host:port string (e.g., "localhost:8091" or ":8091").
func NewAPIClient(apiAddr string) *APIClient {
	addr := apiAddr
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	return &APIClient{
		baseURL: "http://" + addr,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

// HealthResponse mirrors the JSON schema from GET /api/health.
type HealthResponse struct {
	Status          string            `json:"status"`
	Timestamp       time.Time         `json:"timestamp"`
	Endpoints       map[string]string `json:"endpoints"`
	LastPollCycle   time.Time         `json:"last_poll_cycle"`
	PollIntervalSec int               `json:"poll_interval_seconds"`
	DevicesOnline   int               `json:"devices_online"`
	DevicesOffline  int               `json:"devices_offline"`
}

// Asset mirrors the JSON schema from GET /api/assets array elements.
type Asset struct {
	ID              string    `json:"id"`
	Endpoint        string    `json:"endpoint"`
	UnitID          uint8     `json:"unit_id"`
	Addressing      string    `json:"addressing"`
	HoldingRegCount int       `json:"holding_register_count"`
	CoilCount       int       `json:"coil_count"`
	ResponseTimeMs  float64   `json:"response_time_ms"`
	Status          string    `json:"status"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	ViaGateway      string    `json:"via_gateway,omitempty"`
	EnvironmentName string    `json:"environment"`
	ConfigDesc      string    `json:"config_description"`
	Protocol        string    `json:"protocol"`
}

// RegisterResponse mirrors the JSON schema from GET /api/assets/{id}/registers.
type RegisterResponse struct {
	ID               string    `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	HoldingRegisters []uint16  `json:"holding_registers"`
	Coils            []bool    `json:"coils"`
}

// AlertFilters holds optional query parameters for GET /api/alerts.
type AlertFilters struct {
	Severity string
	Device   string
	Rule     string
}

// Alert mirrors the JSON schema for anomaly alert records from SOW-013.0.
type Alert struct {
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	Severity     string    `json:"severity"`
	DeviceID     string    `json:"device_id"`
	Rule         string    `json:"rule"`
	Description  string    `json:"description"`
	Acknowledged bool      `json:"acknowledged"`
	RegisterAddr int       `json:"register_addr,omitempty"`
	OldValue     uint16    `json:"old_value,omitempty"`
	NewValue     uint16    `json:"new_value,omitempty"`
}

// BaselineStatus mirrors the JSON schema from GET /api/baselines.
type BaselineStatus struct {
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
}

// GetHealth fetches the overall monitoring health status.
func (c *APIClient) GetHealth() (*HealthResponse, error) {
	var h HealthResponse
	if err := c.getJSON("/api/health", &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// GetAssets fetches the list of all discovered assets.
func (c *APIClient) GetAssets() ([]Asset, error) {
	var assets []Asset
	if err := c.getJSON("/api/assets", &assets); err != nil {
		return nil, err
	}
	return assets, nil
}

// GetAssetRegisters fetches the latest register snapshot for one asset.
func (c *APIClient) GetAssetRegisters(id string) (*RegisterResponse, error) {
	var reg RegisterResponse
	path := "/api/assets/" + url.PathEscape(id) + "/registers"
	if err := c.getJSON(path, &reg); err != nil {
		return nil, err
	}
	return &reg, nil
}

// GetAlerts fetches anomaly alerts with optional filters.
// Returns nil, nil when the alert API is not yet available (404 or connection refused).
func (c *APIClient) GetAlerts(filters AlertFilters) ([]Alert, error) {
	q := url.Values{}
	if filters.Severity != "" {
		q.Set("severity", filters.Severity)
	}
	if filters.Device != "" {
		q.Set("device", filters.Device)
	}
	if filters.Rule != "" {
		q.Set("rule", filters.Rule)
	}
	path := "/api/alerts"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	var alerts []Alert
	if err := c.getJSON(path, &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

// AcknowledgeAlert sends a POST to acknowledge an alert by ID.
func (c *APIClient) AcknowledgeAlert(id string) error {
	path := "/api/alerts/" + url.PathEscape(id) + "/acknowledge"
	resp, err := c.httpClient.Post(c.baseURL+path, "application/json", nil)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("POST %s: status %d", path, resp.StatusCode)
	}
	return nil
}

// GetBaselines fetches baseline status for all devices.
// Returns nil, nil when the baseline API is not yet available.
func (c *APIClient) GetBaselines() ([]BaselineStatus, error) {
	var baselines []BaselineStatus
	if err := c.getJSON("/api/baselines", &baselines); err != nil {
		return nil, err
	}
	return baselines, nil
}

// getJSON performs an HTTP GET and unmarshals the JSON response body into v.
func (c *APIClient) getJSON(path string, v any) error {
	resp, err := c.httpClient.Get(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response %s: %w", path, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", path, resp.StatusCode)
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode response %s: %w", path, err)
	}
	return nil
}
