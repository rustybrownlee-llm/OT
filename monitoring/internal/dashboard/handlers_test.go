package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/dashboard"
)

func TestAssetsHandler_GroupsByEnvironment(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/assets", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Asset{
			{ID: "a:5020:1", Endpoint: "a:5020", Status: "online", EnvironmentName: "water-mfg", ConfigDesc: "wt-plc-01"},
			{ID: "b:5040:1", Endpoint: "b:5040", Status: "online", EnvironmentName: "pipeline", ConfigDesc: "ps-plc-01"},
		})
	})
	mux.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Alert{})
	})
	mux.HandleFunc("/api/baselines", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.BaselineStatus{})
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{"status": "healthy"})
	})

	dash, _ := newTestDashboard(t, mux)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/assets", nil)
	dash.Routes().ServeHTTP(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "water-mfg") {
		t.Error("assets page should show water-mfg environment group")
	}
	if !strings.Contains(body, "pipeline") {
		t.Error("assets page should show pipeline environment group")
	}
}

func TestAssetsHandler_ConnectionTypeLabel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/assets", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Asset{
			{ID: "a:5020:1", Endpoint: "a:5020", Status: "online", EnvironmentName: "water-mfg", ConfigDesc: "direct-device"},
			{ID: "a:5030:2", Endpoint: "a:5030", Status: "online", EnvironmentName: "water-mfg",
				ConfigDesc: "serial-device", ViaGateway: "a:5030:255"},
		})
	})
	mux.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Alert{})
	})
	mux.HandleFunc("/api/baselines", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.BaselineStatus{})
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{"status": "healthy"})
	})

	dash, _ := newTestDashboard(t, mux)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/partials/assets-table", nil)
	dash.Routes().ServeHTTP(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "Direct Ethernet") {
		t.Error("direct device should show 'Direct Ethernet'")
	}
	if !strings.Contains(body, "Serial via Gateway") {
		t.Error("serial device should show 'Serial via Gateway'")
	}
}

func TestAssetDetailHandler_EnrichedRegisters(t *testing.T) {
	// Asset matches test-plc-01 placement in testdata environment.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/assets", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Asset{
			{
				ID:              "10.10.1.1:5020:1",
				Endpoint:        "10.10.1.1:5020",
				Status:          "online",
				EnvironmentName: "test-env",
				ConfigDesc:      "test-plc-01 (Test PLC 1)",
				HoldingRegCount: 2,
				CoilCount:       1,
			},
		})
	})
	mux.HandleFunc("/api/assets/", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, &dashboard.RegisterResponse{
			ID:               "10.10.1.1:5020:1",
			HoldingRegisters: []uint16{32767, 0},
			Coils:            []bool{true},
		})
	})
	mux.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Alert{})
	})
	mux.HandleFunc("/api/baselines", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.BaselineStatus{})
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{"status": "healthy"})
	})

	dash, _ := newTestDashboard(t, mux)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/assets/10.10.1.1:5020:1", nil)
	dash.Routes().ServeHTTP(w, r)

	body := w.Body.String()
	// Enriched registers should show name from design library.
	if !strings.Contains(body, "flow_rate") {
		t.Error("enriched register should show 'flow_rate' name from design library")
	}
	// pump_speed is writable -- should show orange badge.
	if !strings.Contains(body, "Writable") {
		t.Error("writable register should show 'Writable' badge")
	}
}

func TestAssetRegistersPartial_NoLayout(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/partials/asset-registers/10.10.1.1:5020:1", nil)
	dash.Routes().ServeHTTP(w, r)

	if strings.Contains(w.Body.String(), "<!DOCTYPE html>") {
		t.Error("asset-registers partial should not contain full HTML layout")
	}
}

func TestAlertsHandler_FiltersPassedToAPI(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeTestJSON(w, []dashboard.Alert{})
	})
	mux.HandleFunc("/api/assets", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Asset{})
	})
	mux.HandleFunc("/api/baselines", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.BaselineStatus{})
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{"status": "healthy"})
	})

	dash, _ := newTestDashboard(t, mux)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/partials/alerts-table?severity=critical&device=dev-1", nil)
	dash.Routes().ServeHTTP(w, r)

	if !strings.Contains(gotQuery, "severity=critical") {
		t.Errorf("alert API should receive severity filter, got query: %s", gotQuery)
	}
	if !strings.Contains(gotQuery, "device=dev-1") {
		t.Errorf("alert API should receive device filter, got query: %s", gotQuery)
	}
}

func TestDeviceDetailHandler_LiveAssetsSection(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/assets", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Asset{
			{
				ID:              "10.10.1.1:5020:1",
				Endpoint:        "10.10.1.1:5020",
				Status:          "online",
				EnvironmentName: "test-env",
				ConfigDesc:      "test-plc-01 (Test PLC 1)",
			},
		})
	})
	mux.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.Alert{})
	})
	mux.HandleFunc("/api/baselines", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, []dashboard.BaselineStatus{})
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{"status": "healthy"})
	})

	dash, _ := newTestDashboard(t, mux)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/design/devices/test-plc", nil)
	dash.Routes().ServeHTTP(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "Live Assets") {
		t.Error("device detail page should show 'Live Assets' section when assets match")
	}
	if !strings.Contains(body, "test-plc-01") {
		t.Error("device detail 'Live Assets' should link to matching observed asset")
	}
}

func TestEnvDetailHandler_PortMap(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/design/environments/test-env", nil)
	dash.Routes().ServeHTTP(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "Port Map") {
		t.Error("environment detail should show Port Map section")
	}
	// test-env has one placement with modbus port 5020.
	if !strings.Contains(body, "5020") {
		t.Error("port map should include port 5020")
	}
}

func TestEnvDetailHandler_CrossLinks(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/design/environments/test-env", nil)
	dash.Routes().ServeHTTP(w, r)

	body := w.Body.String()
	// Should link to device atom.
	if !strings.Contains(body, "/design/devices/test-plc") {
		t.Error("environment detail should link to device atom")
	}
	// Should link to network atom.
	if !strings.Contains(body, "/design/networks/test-net") {
		t.Error("environment detail should link to network atom")
	}
}
