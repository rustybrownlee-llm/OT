package apiclient

import (
	"encoding/json"
	"fmt"
)

// BaselineEntry holds per-device baseline status from GET /api/baselines.
type BaselineEntry struct {
	DeviceID        string  `json:"device_id"`
	Status          string  `json:"status"`
	SampleCount     int     `json:"sample_count"`
	RequiredSamples int     `json:"required_samples"`
	RegisterCount   int     `json:"-"` // derived from len(Registers)
	Registers       []any   `json:"registers"`
}

// BaselineResetResponse holds the parsed body of POST /api/baselines/reset.
type BaselineResetResponse struct {
	Message string `json:"message"`
	Devices int    `json:"devices"`
}

// GetBaselines calls GET /api/baselines on the monitoring API.
// Returns a map of device ID to BaselineEntry.
func (c *Client) GetBaselines() (map[string]*BaselineEntry, error) {
	var raw map[string]json.RawMessage
	if err := c.get("/api/baselines", &raw); err != nil {
		return nil, err
	}

	result := make(map[string]*BaselineEntry, len(raw))
	for id, msg := range raw {
		var entry BaselineEntry
		if err := json.Unmarshal(msg, &entry); err != nil {
			return nil, fmt.Errorf("parsing baseline entry for %q: %w", id, err)
		}
		entry.RegisterCount = len(entry.Registers)
		result[id] = &entry
	}
	return result, nil
}

// ResetAllBaselines calls POST /api/baselines/reset on the monitoring API.
func (c *Client) ResetAllBaselines() (*BaselineResetResponse, error) {
	body, err := c.post("/api/baselines/reset")
	if err != nil {
		return nil, err
	}

	var resp BaselineResetResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing reset response: %w", err)
	}
	return &resp, nil
}

// ResetDeviceBaseline calls POST /api/baselines/{deviceID}/reset on the monitoring API.
func (c *Client) ResetDeviceBaseline(deviceID string) (*BaselineResetResponse, error) {
	body, err := c.post("/api/baselines/" + deviceID + "/reset")
	if err != nil {
		return nil, err
	}

	var resp BaselineResetResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing reset response for device %q: %w", deviceID, err)
	}
	return &resp, nil
}
