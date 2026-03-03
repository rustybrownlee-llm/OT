package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// HealthResponse holds the parsed body of GET /api/health.
type HealthResponse struct {
	Status          string            `json:"status"`
	Timestamp       time.Time         `json:"timestamp"`
	Endpoints       map[string]string `json:"endpoints"`
	LastPollCycle   time.Time         `json:"last_poll_cycle"`
	PollIntervalSec int               `json:"poll_interval_seconds"`
	DevicesOnline   int               `json:"devices_online"`
	DevicesOffline  int               `json:"devices_offline"`
}

// GetHealth calls GET /api/health on the monitoring API.
func (c *Client) GetHealth() (*HealthResponse, error) {
	var h HealthResponse
	if err := c.get("/api/health", &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// ParseHealthResponse decodes a health response body from an io.Reader.
// Useful when the caller already holds the http.Response and needs to parse it
// without constructing a full Client.
func ParseHealthResponse(r io.Reader) (*HealthResponse, error) {
	var h HealthResponse
	if err := json.NewDecoder(r).Decode(&h); err != nil {
		return nil, fmt.Errorf("parsing health response: %w", err)
	}
	return &h, nil
}
