package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/alert"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/api"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/baseline"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/inventory"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/poller"
)

// --- helpers ---

func makeTestInventory() *inventory.Inventory {
	return inventory.NewInventory()
}

func makePollState() *poller.PollState {
	return &poller.PollState{}
}

func makeEmptyEngine() *baseline.Engine {
	return baseline.NewEngine(150, 300, 1000)
}

func makeEmptyAlertStore() *alert.Store {
	return alert.NewStore(1000)
}

func makeOnlineAsset(id, endpoint string, unitID uint8) *inventory.Asset {
	return &inventory.Asset{
		ID:              id,
		Endpoint:        endpoint,
		UnitID:          unitID,
		Status:          inventory.StatusOnline,
		HoldingRegCount: 5,
		CoilCount:       4,
		Addressing:      "zero-based",
		Protocol:        "modbus-tcp",
		FirstSeen:       time.Now(),
		LastSeen:        time.Now(),
	}
}

func makeRouter(inv *inventory.Inventory, state *poller.PollState,
	engine *baseline.Engine, store *alert.Store) http.Handler {
	return api.NewRouter(inv, state, 2, engine, store)
}

func doRequest(t *testing.T, router http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func doRequestWithBody(t *testing.T, router http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// testBase is a fixed timestamp for deterministic alert creation.
var testBase = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func insertAlert(store *alert.Store, ruleID, deviceID, severity string, active bool, ts time.Time) *alert.Alert {
	id := alert.AlertID(ruleID, deviceID, 0)
	a := store.GetOrCreate(id, func() *alert.Alert {
		return &alert.Alert{
			ID:              id,
			Timestamp:       ts,
			Severity:        severity,
			RuleID:          ruleID,
			DeviceID:        deviceID,
			Description:     "test",
			RegisterAddress: -1,
			ExpectedValue:   "N/A",
			ActualValue:     "N/A",
			Active:          active,
		}
	})
	if !active {
		store.Clear(id, ts.Add(time.Minute))
	}
	return a
}

// --- existing health / assets tests (updated for new NewRouter signature) ---

func TestHealth_EmptyInventory_Unhealthy(t *testing.T) {
	inv := makeTestInventory()
	router := makeRouter(inv, makePollState(), makeEmptyEngine(), makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/health")

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	status, _ := resp["status"].(string)
	if status != "unhealthy" {
		t.Errorf("status: got %q, want %q", status, "unhealthy")
	}
}

func TestHealth_AllOnline_Healthy(t *testing.T) {
	inv := makeTestInventory()
	inv.Upsert(makeOnlineAsset("10.10.30.10:5020:1", "10.10.30.10:5020", 1))
	inv.Upsert(makeOnlineAsset("10.10.30.10:5021:1", "10.10.30.10:5021", 1))

	router := makeRouter(inv, makePollState(), makeEmptyEngine(), makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/health")
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	status, _ := resp["status"].(string)
	if status != "healthy" {
		t.Errorf("status: got %q, want %q", status, "healthy")
	}
}

func TestHealth_SomeOffline_Degraded(t *testing.T) {
	inv := makeTestInventory()
	inv.Upsert(makeOnlineAsset("10.10.30.10:5020:1", "10.10.30.10:5020", 1))

	offline := makeOnlineAsset("10.10.30.10:5021:1", "10.10.30.10:5021", 1)
	offline.Status = inventory.StatusOffline
	inv.Upsert(offline)

	router := makeRouter(inv, makePollState(), makeEmptyEngine(), makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/health")

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	status, _ := resp["status"].(string)
	if status != "degraded" {
		t.Errorf("status: got %q, want %q", status, "degraded")
	}
}

func TestHealth_CorrectStructure(t *testing.T) {
	inv := makeTestInventory()
	inv.Upsert(makeOnlineAsset("10.10.30.10:5020:1", "10.10.30.10:5020", 1))

	router := makeRouter(inv, makePollState(), makeEmptyEngine(), makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/health")

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	requiredFields := []string{"status", "timestamp", "endpoints", "last_poll_cycle",
		"poll_interval_seconds", "devices_online", "devices_offline"}
	for _, f := range requiredFields {
		if _, ok := resp[f]; !ok {
			t.Errorf("response missing field %q", f)
		}
	}
}

func TestAssets_EmptyInventory_ReturnsEmptyArray(t *testing.T) {
	inv := makeTestInventory()
	router := makeRouter(inv, makePollState(), makeEmptyEngine(), makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/assets")
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp []interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty array, got %d elements", len(resp))
	}
}

func TestAssets_ReturnsDiscoveredDevices(t *testing.T) {
	inv := makeTestInventory()
	inv.Upsert(makeOnlineAsset("10.10.30.10:5020:1", "10.10.30.10:5020", 1))
	inv.Upsert(makeOnlineAsset("10.10.30.10:5021:1", "10.10.30.10:5021", 1))

	router := makeRouter(inv, makePollState(), makeEmptyEngine(), makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/assets")
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("expected 2 assets, got %d", len(resp))
	}

	requiredFields := []string{"id", "endpoint", "unit_id", "addressing",
		"holding_register_count", "coil_count", "status"}
	for _, f := range requiredFields {
		if _, ok := resp[0][f]; !ok {
			t.Errorf("asset missing field %q", f)
		}
	}
}

func TestAssetRegisters_NotFound(t *testing.T) {
	inv := makeTestInventory()
	router := makeRouter(inv, makePollState(), makeEmptyEngine(), makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/assets/nonexistent:9999:1/registers")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestAssetRegisters_ReturnsValues(t *testing.T) {
	inv := makeTestInventory()
	a := makeOnlineAsset("10.10.30.10:5020:1", "10.10.30.10:5020", 1)
	inv.Upsert(a)
	inv.UpdateRegisters(a.ID, []uint16{100, 200, 300}, []bool{true, false}, time.Now())

	router := makeRouter(inv, makePollState(), makeEmptyEngine(), makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/assets/10.10.30.10:5020:1/registers")
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := resp["holding_registers"]; !ok {
		t.Error("response missing 'holding_registers'")
	}
	if _, ok := resp["coils"]; !ok {
		t.Error("response missing 'coils'")
	}
	if resp["id"] != "10.10.30.10:5020:1" {
		t.Errorf("id: got %v, want %q", resp["id"], "10.10.30.10:5020:1")
	}
}

func TestCORSHeader_PresentOnAllResponses(t *testing.T) {
	inv := makeTestInventory()
	router := makeRouter(inv, makePollState(), makeEmptyEngine(), makeEmptyAlertStore())

	paths := []string{"/api/health", "/api/assets"}
	for _, path := range paths {
		rr := doRequest(t, router, http.MethodGet, path)
		cors := rr.Header().Get("Access-Control-Allow-Origin")
		if cors != "*" {
			t.Errorf("path %q: Access-Control-Allow-Origin: got %q, want %q", path, cors, "*")
		}
	}
}

func TestCORSHeader_OnRegisterEndpoint(t *testing.T) {
	inv := makeTestInventory()
	inv.Upsert(makeOnlineAsset("10.10.30.10:5020:1", "10.10.30.10:5020", 1))

	router := makeRouter(inv, makePollState(), makeEmptyEngine(), makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/assets/10.10.30.10:5020:1/registers")
	cors := rr.Header().Get("Access-Control-Allow-Origin")
	if cors != "*" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want %q", cors, "*")
	}
}

func TestContentType_JSON(t *testing.T) {
	inv := makeTestInventory()
	router := makeRouter(inv, makePollState(), makeEmptyEngine(), makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/health")
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

// --- alert endpoint tests ---

func TestAlerts_EmptyStore_ReturnsEmptyArray(t *testing.T) {
	store := makeEmptyAlertStore()
	router := makeRouter(makeTestInventory(), makePollState(), makeEmptyEngine(), store)

	rr := doRequest(t, router, http.MethodGet, "/api/alerts")
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp []interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty array, got %d elements", len(resp))
	}
}

func TestAlerts_ReturnsAlertsWithCorrectStructure(t *testing.T) {
	store := makeEmptyAlertStore()
	insertAlert(store, "value_out_of_range", "dev1", "warning", true, testBase)

	router := makeRouter(makeTestInventory(), makePollState(), makeEmptyEngine(), store)

	rr := doRequest(t, router, http.MethodGet, "/api/alerts")
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(resp))
	}

	requiredFields := []string{"id", "timestamp", "severity", "rule_id", "device_id",
		"description", "register_address", "expected_value", "actual_value",
		"acknowledged", "active"}
	for _, f := range requiredFields {
		if _, ok := resp[0][f]; !ok {
			t.Errorf("alert missing field %q", f)
		}
	}
}

func TestAlerts_FilterBySeverity(t *testing.T) {
	store := makeEmptyAlertStore()
	insertAlert(store, "rule", "dev1", "high", true, testBase)
	insertAlert(store, "rule", "dev2", "warning", true, testBase)
	insertAlert(store, "rule", "dev3", "critical", true, testBase)

	router := makeRouter(makeTestInventory(), makePollState(), makeEmptyEngine(), store)

	rr := doRequest(t, router, http.MethodGet, "/api/alerts?severity=high")
	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("?severity=high: got %d alerts, want 1", len(resp))
	}
	if resp[0]["severity"] != "high" {
		t.Errorf("severity: got %v, want %q", resp[0]["severity"], "high")
	}
}

func TestAlerts_FilterByDevice(t *testing.T) {
	store := makeEmptyAlertStore()
	insertAlert(store, "rule", "device-A", "warning", true, testBase)
	insertAlert(store, "rule2", "device-B", "warning", true, testBase)

	router := makeRouter(makeTestInventory(), makePollState(), makeEmptyEngine(), store)

	rr := doRequest(t, router, http.MethodGet, "/api/alerts?device=device-A")
	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("?device=device-A: got %d alerts, want 1", len(resp))
	}
	if resp[0]["device_id"] != "device-A" {
		t.Errorf("device_id: got %v, want %q", resp[0]["device_id"], "device-A")
	}
}

func TestAlerts_FilterByActive(t *testing.T) {
	store := makeEmptyAlertStore()
	insertAlert(store, "rule", "dev1", "warning", true, testBase)
	insertAlert(store, "rule", "dev2", "warning", false, testBase.Add(time.Second)) // cleared

	router := makeRouter(makeTestInventory(), makePollState(), makeEmptyEngine(), store)

	rr := doRequest(t, router, http.MethodGet, "/api/alerts?active=true")
	var active []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&active); err != nil {
		t.Fatalf("decode active: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("?active=true: got %d, want 1", len(active))
	}

	rr = doRequest(t, router, http.MethodGet, "/api/alerts?active=false")
	var inactive []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&inactive); err != nil {
		t.Fatalf("decode inactive: %v", err)
	}
	if len(inactive) != 1 {
		t.Errorf("?active=false: got %d, want 1", len(inactive))
	}
}

func TestAlertByID_NotFound(t *testing.T) {
	store := makeEmptyAlertStore()
	router := makeRouter(makeTestInventory(), makePollState(), makeEmptyEngine(), store)

	rr := doRequest(t, router, http.MethodGet, "/api/alerts/nonexistent1234")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestAlertByID_ReturnsAlert(t *testing.T) {
	store := makeEmptyAlertStore()
	a := insertAlert(store, "value_out_of_range", "dev1", "warning", true, testBase)

	router := makeRouter(makeTestInventory(), makePollState(), makeEmptyEngine(), store)

	rr := doRequest(t, router, http.MethodGet, "/api/alerts/"+a.ID)
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["id"] != a.ID {
		t.Errorf("id: got %v, want %q", resp["id"], a.ID)
	}
}

func TestAcknowledge_ReturnsUpdatedAlert(t *testing.T) {
	store := makeEmptyAlertStore()
	a := insertAlert(store, "value_out_of_range", "dev1", "warning", true, testBase)

	router := makeRouter(makeTestInventory(), makePollState(), makeEmptyEngine(), store)

	rr := doRequestWithBody(t, router, http.MethodPost, "/api/alerts/"+a.ID+"/acknowledge", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["acknowledged"] != true {
		t.Errorf("acknowledged: got %v, want true", resp["acknowledged"])
	}
	// Alert must remain active.
	if resp["active"] != true {
		t.Errorf("active: got %v, want true (acknowledge should not clear alert)", resp["active"])
	}
}

func TestAcknowledge_NotFound(t *testing.T) {
	store := makeEmptyAlertStore()
	router := makeRouter(makeTestInventory(), makePollState(), makeEmptyEngine(), store)

	rr := doRequestWithBody(t, router, http.MethodPost, "/api/alerts/nonexistent1234/acknowledge", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// --- baseline endpoint tests ---

func TestBaselines_ReturnsPerDeviceStatus(t *testing.T) {
	engine := baseline.NewEngine(5, 300, 100)
	engine.SetKnownDevices([]string{"dev1"})

	snap := baseline.DeviceSnapshot{
		DeviceID:   "dev1",
		Timestamp:  testBase,
		Holding:    []uint16{100, 200},
		Online:     true,
		ResponseMs: 5.0,
	}
	engine.RecordCycle([]baseline.DeviceSnapshot{snap})

	router := makeRouter(makeTestInventory(), makePollState(), engine, makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/baselines")
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, ok := resp["dev1"]; !ok {
		t.Fatal("baselines: expected entry for dev1")
	}
}

func TestBaselines_ShowsLearningStatus(t *testing.T) {
	engine := baseline.NewEngine(50, 300, 100) // 50 cycles required
	engine.SetKnownDevices([]string{"dev1"})

	// Feed only 1 cycle -- still in learning.
	snap := baseline.DeviceSnapshot{
		DeviceID:   "dev1",
		Timestamp:  testBase,
		Holding:    []uint16{100},
		Online:     true,
		ResponseMs: 5.0,
	}
	engine.RecordCycle([]baseline.DeviceSnapshot{snap})

	router := makeRouter(makeTestInventory(), makePollState(), engine, makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/baselines")
	var resp map[string]map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	entry := resp["dev1"]
	if entry["status"] != "learning" {
		t.Errorf("status: got %v, want %q", entry["status"], "learning")
	}

	registers, _ := entry["registers"].([]interface{})
	if len(registers) != 0 {
		t.Errorf("registers: expected empty during learning, got %d", len(registers))
	}
}

func TestBaselines_ShowsEstablishedWithStatistics(t *testing.T) {
	const learningCycles = 5
	engine := baseline.NewEngine(learningCycles, 300, 100)
	engine.SetKnownDevices([]string{"dev1"})

	for i := 0; i < learningCycles; i++ {
		snap := baseline.DeviceSnapshot{
			DeviceID:   "dev1",
			Timestamp:  testBase.Add(time.Duration(i) * 2 * time.Second),
			Holding:    []uint16{1000},
			Online:     true,
			ResponseMs: 5.0,
		}
		engine.RecordCycle([]baseline.DeviceSnapshot{snap})
	}

	router := makeRouter(makeTestInventory(), makePollState(), engine, makeEmptyAlertStore())

	rr := doRequest(t, router, http.MethodGet, "/api/baselines")
	var resp map[string]map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	entry := resp["dev1"]
	if entry["status"] != "established" {
		t.Errorf("status: got %v, want %q", entry["status"], "established")
	}

	registers, _ := entry["registers"].([]interface{})
	if len(registers) == 0 {
		t.Error("registers: expected non-empty for established baseline")
	}
}

func TestBaselines_CORSOnAlertEndpoints(t *testing.T) {
	store := makeEmptyAlertStore()
	router := makeRouter(makeTestInventory(), makePollState(), makeEmptyEngine(), store)

	paths := []string{"/api/alerts", "/api/baselines"}
	for _, path := range paths {
		rr := doRequest(t, router, http.MethodGet, path)
		cors := rr.Header().Get("Access-Control-Allow-Origin")
		if cors != "*" {
			t.Errorf("path %q: Access-Control-Allow-Origin: got %q, want %q", path, cors, "*")
		}
	}
}

func TestBaselines_ContentTypeJSON(t *testing.T) {
	store := makeEmptyAlertStore()
	router := makeRouter(makeTestInventory(), makePollState(), makeEmptyEngine(), store)

	rr := doRequest(t, router, http.MethodGet, "/api/alerts")
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}
