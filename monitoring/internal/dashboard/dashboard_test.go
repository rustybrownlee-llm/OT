package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/dashboard"
)

// newTestDashboard creates a Dashboard with a mock API server and the test design library.
func newTestDashboard(t *testing.T, apiHandler http.Handler) (*dashboard.Dashboard, *httptest.Server) {
	t.Helper()
	apiSrv := httptest.NewServer(apiHandler)
	t.Cleanup(apiSrv.Close)

	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}

	client := dashboard.NewAPIClient(apiSrv.Listener.Addr().String())
	dash := dashboard.NewDashboard(client, lib, nil)
	return dash, apiSrv
}

// emptyAPIServer returns an HTTP handler that serves empty/default responses.
func emptyAPIServer() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{
			"status":           "healthy",
			"devices_online":   2,
			"devices_offline":  0,
			"poll_interval_seconds": 2,
		})
	})
	mux.HandleFunc("/api/assets", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Asset{
			{
				ID:              "10.10.1.1:5020:1",
				Endpoint:        "10.10.1.1:5020",
				UnitID:          1,
				Status:          "online",
				EnvironmentName: "water-mfg",
				ConfigDesc:      "test-plc-01 (Test PLC 1)",
				HoldingRegCount: 2,
				CoilCount:       1,
			},
		})
	})
	mux.HandleFunc("/api/assets/", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, &dashboard.RegisterResponse{
			ID:               "10.10.1.1:5020:1",
			HoldingRegisters: []uint16{100, 200},
			Coils:            []bool{true},
		})
	})
	mux.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Alert{})
	})
	mux.HandleFunc("/api/baselines", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.BaselineStatus{
			{DeviceID: "dev-1", Status: "established"},
		})
	})
	return mux
}

func TestDashboard_OverviewPage(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	dash.Routes().ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /: status %d, want 200", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "OT Security Monitor") {
		t.Error("overview page should contain brand title")
	}
	if !strings.Contains(body, "Overview") {
		t.Error("overview page should contain 'Overview'")
	}
}

func TestDashboard_OverviewCardsPartial(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/partials/overview-cards", nil)
	dash.Routes().ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /partials/overview-cards: status %d, want 200", resp.StatusCode)
	}
	body := w.Body.String()
	// Partial should not contain the full layout.
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("overview-cards partial should not contain full HTML layout")
	}
	// Should contain card content.
	if !strings.Contains(body, "Devices Online") {
		t.Error("overview-cards partial should contain 'Devices Online'")
	}
}

func TestDashboard_AssetsPage(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/assets", nil)
	dash.Routes().ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /assets: status %d, want 200", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Asset Inventory") {
		t.Error("assets page should contain 'Asset Inventory'")
	}
}

func TestDashboard_AssetsTablePartial(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/partials/assets-table", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("GET /partials/assets-table: status %d, want 200", w.Result().StatusCode)
	}
	if strings.Contains(w.Body.String(), "<!DOCTYPE html>") {
		t.Error("assets-table partial should not contain full HTML layout")
	}
}

func TestDashboard_AssetDetailPage(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/assets/10.10.1.1:5020:1", nil)
	dash.Routes().ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /assets/{id}: status %d, want 200", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Holding Registers") {
		t.Error("asset detail should show register section")
	}
}

func TestDashboard_AssetDetail_NotFound(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/assets/nonexistent-id", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("GET /assets/nonexistent: status %d, want 404", w.Result().StatusCode)
	}
}

func TestDashboard_AlertsPage(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	dash.Routes().ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /alerts: status %d, want 200", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Anomaly Alerts") {
		t.Error("alerts page should contain 'Anomaly Alerts'")
	}
}

func TestDashboard_AlertsUnavailable(t *testing.T) {
	// API server returns 404 for /api/alerts.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{"status": "healthy"})
	})
	mux.HandleFunc("/api/assets", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Asset{})
	})
	mux.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/baselines", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	dash, _ := newTestDashboard(t, mux)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("alerts page with unavailable API: status %d, want 200", w.Result().StatusCode)
	}
	if !strings.Contains(w.Body.String(), "not yet available") {
		t.Error("alerts page should show 'not yet available' when alert API returns 404")
	}
}

func TestDashboard_DesignDevicesPage(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/design/devices", nil)
	dash.Routes().ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /design/devices: status %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(w.Body.String(), "Device Library") {
		t.Error("design devices page should contain 'Device Library'")
	}
}

func TestDashboard_DesignDeviceDetail(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/design/devices/test-plc", nil)
	dash.Routes().ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /design/devices/test-plc: status %d, want 200", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "TestCorp") {
		t.Error("device detail should show vendor")
	}
	if !strings.Contains(body, "Writable") {
		t.Error("device detail should show writable badge for writable registers")
	}
}

func TestDashboard_DesignDeviceDetail_NotFound(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/design/devices/nonexistent", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("GET /design/devices/nonexistent: status %d, want 404", w.Result().StatusCode)
	}
}

func TestDashboard_DesignNetworksPage(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/design/networks", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("GET /design/networks: status %d, want 200", w.Result().StatusCode)
	}
	if !strings.Contains(w.Body.String(), "Network Library") {
		t.Error("networks page should contain 'Network Library'")
	}
}

func TestDashboard_DesignNetworkDetail(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/design/networks/test-net", nil)
	dash.Routes().ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /design/networks/test-net: status %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(w.Body.String(), "Test Network") {
		t.Error("network detail should show network name")
	}
}

func TestDashboard_DesignEnvironmentsPage(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/design/environments", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("GET /design/environments: status %d, want 200", w.Result().StatusCode)
	}
	if !strings.Contains(w.Body.String(), "Environment Library") {
		t.Error("environments page should contain 'Environment Library'")
	}
}

func TestDashboard_DesignEnvironmentDetail(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/design/environments/test-env", nil)
	dash.Routes().ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /design/environments/test-env: status %d, want 200", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Test Environment") {
		t.Error("environment detail should show environment name")
	}
	if !strings.Contains(body, "Port Map") {
		t.Error("environment detail should show Port Map section")
	}
}

func TestDashboard_LibraryUnavailable(t *testing.T) {
	// Create dashboard with empty library (simulates missing /design mount).
	apiSrv := httptest.NewServer(emptyAPIServer())
	defer apiSrv.Close()

	emptyLib, _ := dashboard.LoadDesignLibrary("/nonexistent/path")
	client := dashboard.NewAPIClient(apiSrv.Listener.Addr().String())
	dash := dashboard.NewDashboard(client, emptyLib, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/design/devices", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("design page with no library: status %d, want 200", w.Result().StatusCode)
	}
	if !strings.Contains(w.Body.String(), "not available") {
		t.Error("design page should show 'not available' when library is empty")
	}
}
