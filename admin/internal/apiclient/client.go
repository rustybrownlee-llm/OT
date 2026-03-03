// Package apiclient provides an HTTP client for the monitoring module API.
// All methods call the monitoring API over HTTP and return typed response values.
// The admin module does not import monitoring Go packages; all communication
// is over the network per ADR-005.
package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls the monitoring HTTP API on behalf of admin CLI commands.
// Base address is host:port (e.g., "localhost:8091").
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a Client targeting the monitoring API at addr (host:port).
func New(addr string) *Client {
	return &Client{
		baseURL: "http://" + addr,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// get performs an HTTP GET to path and decodes the JSON response into dest.
func (c *Client) get(path string, dest any) error {
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", path, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return nil
}

// post performs an HTTP POST to path with no request body.
// Returns the raw response body bytes on success.
func (c *Client) post(path string) ([]byte, error) {
	resp, err := c.http.Post(c.baseURL+path, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from POST %s: %w", path, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("POST %s: HTTP %d: %s", path, resp.StatusCode, string(body))
	}
	return body, nil
}
