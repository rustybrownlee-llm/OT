package dashboard_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/dashboard"
)

// newTestClient creates an APIClient pointed at a test HTTP server.
func newTestClient(srv *httptest.Server) *dashboard.APIClient {
	client := dashboard.NewAPIClient(srv.Listener.Addr().String())
	return client
}

func TestGetHealth_ParsesResponse(t *testing.T) {
	want := &dashboard.HealthResponse{
		Status:         "healthy",
		DevicesOnline:  3,
		DevicesOffline: 0,
		PollIntervalSec: 2,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		writeTestJSON(w, want)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	got, err := client.GetHealth()
	if err != nil {
		t.Fatalf("GetHealth: unexpected error: %v", err)
	}
	if got.Status != want.Status {
		t.Errorf("Status: got %q, want %q", got.Status, want.Status)
	}
	if got.DevicesOnline != want.DevicesOnline {
		t.Errorf("DevicesOnline: got %d, want %d", got.DevicesOnline, want.DevicesOnline)
	}
}

func TestGetAssets_ParsesArray(t *testing.T) {
	want := []dashboard.Asset{
		{ID: "10.10.1.1:5020:1", Endpoint: "10.10.1.1:5020", Status: "online", EnvironmentName: "water-mfg"},
		{ID: "10.10.1.1:5021:1", Endpoint: "10.10.1.1:5021", Status: "offline", EnvironmentName: "water-mfg"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, want)
	}))
	defer srv.Close()

	got, err := newTestClient(srv).GetAssets()
	if err != nil {
		t.Fatalf("GetAssets: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(assets): got %d, want 2", len(got))
	}
	if got[0].ID != want[0].ID {
		t.Errorf("assets[0].ID: got %q, want %q", got[0].ID, want[0].ID)
	}
}

func TestGetAssetRegisters_ParsesRegisters(t *testing.T) {
	want := &dashboard.RegisterResponse{
		ID:               "10.10.1.1:5020:1",
		Timestamp:        time.Now().UTC(),
		HoldingRegisters: []uint16{100, 200, 300},
		Coils:            []bool{true, false, true},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/registers") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		writeTestJSON(w, want)
	}))
	defer srv.Close()

	got, err := newTestClient(srv).GetAssetRegisters("10.10.1.1:5020:1")
	if err != nil {
		t.Fatalf("GetAssetRegisters: unexpected error: %v", err)
	}
	if len(got.HoldingRegisters) != 3 {
		t.Errorf("HoldingRegisters length: got %d, want 3", len(got.HoldingRegisters))
	}
	if got.HoldingRegisters[1] != 200 {
		t.Errorf("HoldingRegisters[1]: got %d, want 200", got.HoldingRegisters[1])
	}
}

func TestGetAlerts_WithoutFilters(t *testing.T) {
	want := []dashboard.Alert{
		{ID: "alert-1", Severity: "critical", DeviceID: "dev-1", Rule: "device_offline"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/alerts" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		writeTestJSON(w, want)
	}))
	defer srv.Close()

	got, err := newTestClient(srv).GetAlerts(dashboard.AlertFilters{})
	if err != nil {
		t.Fatalf("GetAlerts: unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "alert-1" {
		t.Errorf("unexpected alerts: %+v", got)
	}
}

func TestGetAlerts_WithFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("severity") != "critical" {
			t.Errorf("severity param: got %q, want %q", q.Get("severity"), "critical")
		}
		if q.Get("device") != "dev-1" {
			t.Errorf("device param: got %q, want %q", q.Get("device"), "dev-1")
		}
		writeTestJSON(w, []dashboard.Alert{})
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetAlerts(dashboard.AlertFilters{
		Severity: "critical",
		Device:   "dev-1",
	})
	if err != nil {
		t.Fatalf("GetAlerts with filters: unexpected error: %v", err)
	}
}

func TestAcknowledgeAlert_SendsPOST(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := newTestClient(srv).AcknowledgeAlert("alert-42")
	if err != nil {
		t.Fatalf("AcknowledgeAlert: unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %q, want POST", gotMethod)
	}
	if !strings.Contains(gotPath, "alert-42") {
		t.Errorf("path %q should contain alert ID", gotPath)
	}
}

func TestAPIClient_Unavailable(t *testing.T) {
	// Point at a port that is not listening.
	client := dashboard.NewAPIClient("localhost:19999")
	_, err := client.GetHealth()
	if err == nil {
		t.Fatal("GetHealth: expected error for unavailable API, got nil")
	}
}

func TestGetBaselines_ParsesArray(t *testing.T) {
	want := []dashboard.BaselineStatus{
		{DeviceID: "dev-1", Status: "established"},
		{DeviceID: "dev-2", Status: "learning"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, want)
	}))
	defer srv.Close()

	got, err := newTestClient(srv).GetBaselines()
	if err != nil {
		t.Fatalf("GetBaselines: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(baselines): got %d, want 2", len(got))
	}
}

// writeTestJSON writes v as JSON to w with status 200.
func writeTestJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}
