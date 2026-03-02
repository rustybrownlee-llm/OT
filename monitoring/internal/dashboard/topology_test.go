package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/dashboard"
)

// ---- Unit tests for topology layout engine ----

// TestBuildTopologyData_NilDef verifies graceful nil handling.
func TestBuildTopologyData_NilDef(t *testing.T) {
	td := dashboard.BuildTopologyData(nil, nil)
	if td != nil {
		t.Error("BuildTopologyData(nil, nil) should return nil")
	}
}

// TestBuildTopologyData_DefaultArchetype verifies FR-016: environments without
// an archetype field default to "legacy-flat".
func TestBuildTopologyData_DefaultArchetype(t *testing.T) {
	def := &dashboard.EnvironmentDef{
		Env: dashboard.EnvironmentMeta{
			ID:   "no-archetype",
			Name: "No Archetype Environment",
		},
	}
	td := dashboard.BuildTopologyData(def, nil)
	if td == nil {
		t.Fatal("BuildTopologyData returned nil for env with no archetype")
	}
	if td.Archetype != "legacy-flat" {
		t.Errorf("Archetype: got %q, want %q", td.Archetype, "legacy-flat")
	}
}

// TestBuildTopologyData_ModernSegmented verifies that a modern-segmented environment
// with level1/level2/level3 networks produces distinct Purdue levels.
func TestBuildTopologyData_ModernSegmented(t *testing.T) {
	year := 2020
	def := &dashboard.EnvironmentDef{
		Env: dashboard.EnvironmentMeta{
			ID:        "modern-test",
			Name:      "Modern Test",
			Archetype: "modern-segmented",
		},
		Networks: []dashboard.NetworkRef{
			{Ref: "wt-level3"},
			{Ref: "wt-level2"},
			{Ref: "wt-level1"},
		},
		Placements: []dashboard.Placement{
			{ID: "dev-l3", Device: "test-plc", Network: "wt-level3", IP: "10.0.3.1", ModbusPort: 5020, Installed: &year},
			{ID: "dev-l1", Device: "test-plc", Network: "wt-level1", IP: "10.0.1.1", ModbusPort: 5021},
		},
	}
	td := dashboard.BuildTopologyData(def, nil)
	if td == nil {
		t.Fatal("BuildTopologyData returned nil")
	}
	levelNames := make(map[string]bool)
	for _, lvl := range td.Levels {
		levelNames[lvl.Name] = true
	}
	if !levelNames["Level 3 - Site Operations"] {
		t.Error("modern-segmented should have 'Level 3 - Site Operations' level")
	}
	if !levelNames["Level 2 - Supervisory Control"] {
		t.Error("modern-segmented should have 'Level 2 - Supervisory Control' level")
	}
	if !levelNames["Level 1 - Basic Control"] {
		t.Error("modern-segmented should have 'Level 1 - Basic Control' level")
	}
}

// TestBuildTopologyData_LegacyFlat verifies that a legacy-flat environment
// with an unmanaged ethernet network produces a "Flat (Unclassified)" level.
// Uses test-flat-net (unmanaged, no VLAN, ethernet) from testdata.
func TestBuildTopologyData_LegacyFlat(t *testing.T) {
	year := 2015
	def := &dashboard.EnvironmentDef{
		Env: dashboard.EnvironmentMeta{
			ID:        "flat-test",
			Name:      "Flat Test",
			Archetype: "legacy-flat",
		},
		Networks: []dashboard.NetworkRef{
			{Ref: "test-flat-net"},
		},
		Placements: []dashboard.Placement{
			{ID: "flat-dev-01", Device: "test-plc", Network: "test-flat-net", IP: "192.168.10.10", ModbusPort: 5050, Installed: &year},
		},
	}
	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}
	td := dashboard.BuildTopologyData(def, lib)
	if td == nil {
		t.Fatal("BuildTopologyData returned nil")
	}
	foundFlat := false
	for _, lvl := range td.Levels {
		if lvl.Name == "Flat (Unclassified)" {
			foundFlat = true
			break
		}
	}
	if !foundFlat {
		names := make([]string, 0, len(td.Levels))
		for _, lvl := range td.Levels {
			names = append(names, lvl.Name)
		}
		t.Errorf("legacy-flat environment should produce 'Flat (Unclassified)' level, got: %v", names)
	}
}

// TestBuildTopologyData_SerialBackbone verifies that a network containing "serial"
// in its ID is classified as "Serial Backbone".
func TestBuildTopologyData_SerialBackbone(t *testing.T) {
	def := &dashboard.EnvironmentDef{
		Env: dashboard.EnvironmentMeta{
			ID:        "serial-test",
			Name:      "Serial Test",
			Archetype: "legacy-flat",
		},
		Networks: []dashboard.NetworkRef{
			{Ref: "ps-serial-bus"},
		},
		Placements: []dashboard.Placement{
			{ID: "serial-dev", Device: "test-plc", Network: "ps-serial-bus", SerialAddress: 1, Gateway: "ps-gw-01"},
		},
	}
	td := dashboard.BuildTopologyData(def, nil)
	if td == nil {
		t.Fatal("BuildTopologyData returned nil")
	}
	foundSerial := false
	for _, lvl := range td.Levels {
		if lvl.Name == "Serial Backbone" {
			foundSerial = true
			break
		}
	}
	if !foundSerial {
		t.Error("network with 'serial' in ID should produce 'Serial Backbone' level")
	}
}

// TestBuildTopologyData_WANNetwork verifies that a network containing "wan"
// in its ID is classified as "External/WAN".
func TestBuildTopologyData_WANNetwork(t *testing.T) {
	def := &dashboard.EnvironmentDef{
		Env: dashboard.EnvironmentMeta{
			ID:        "wan-test",
			Name:      "WAN Test",
			Archetype: "hybrid",
		},
		Networks: []dashboard.NetworkRef{
			{Ref: "ww-wan"},
		},
		Placements: []dashboard.Placement{},
	}
	td := dashboard.BuildTopologyData(def, nil)
	if td == nil {
		t.Fatal("BuildTopologyData returned nil")
	}
	foundWAN := false
	for _, lvl := range td.Levels {
		if lvl.Name == "External/WAN" {
			foundWAN = true
			break
		}
	}
	if !foundWAN {
		t.Error("network with 'wan' in ID should produce 'External/WAN' level")
	}
}

// TestBuildTopologyData_EnforcedBoundary verifies that enforced boundaries are
// translated correctly to TopologyBoundary with the correct CSS class.
func TestBuildTopologyData_EnforcedBoundary(t *testing.T) {
	installed := 2018
	def := &dashboard.EnvironmentDef{
		Env: dashboard.EnvironmentMeta{
			ID:        "boundary-test",
			Archetype: "hybrid",
		},
		Networks: []dashboard.NetworkRef{
			{Ref: "ww-level3"},
			{Ref: "ww-flat"},
		},
		Boundaries: []dashboard.BoundaryDef{
			{
				Between:        []string{"ww-level3", "ww-flat"},
				State:          "enforced",
				Infrastructure: "managed-switch",
				Installed:      &installed,
			},
		},
	}
	td := dashboard.BuildTopologyData(def, nil)
	if td == nil {
		t.Fatal("BuildTopologyData returned nil")
	}
	if len(td.Boundaries) != 1 {
		t.Fatalf("Boundaries: got %d, want 1", len(td.Boundaries))
	}
	b := td.Boundaries[0]
	if b.State != "enforced" {
		t.Errorf("Boundary.State: got %q, want %q", b.State, "enforced")
	}
	if b.CSSClass != "boundary-enforced" {
		t.Errorf("Boundary.CSSClass: got %q, want %q", b.CSSClass, "boundary-enforced")
	}
	if b.Infrastructure != "managed-switch" {
		t.Errorf("Boundary.Infrastructure: got %q, want %q", b.Infrastructure, "managed-switch")
	}
}

// TestBuildTopologyData_IntendedBoundary exercises the "intended" boundary state.
// No current environment uses intended boundaries, but the CSS and template must handle it.
// SOW-019.0 work tracking item: "Add unit test with mock environment data that exercises
// `intended` boundary state."
func TestBuildTopologyData_IntendedBoundary(t *testing.T) {
	def := &dashboard.EnvironmentDef{
		Env: dashboard.EnvironmentMeta{
			ID:        "intended-boundary-test",
			Archetype: "hybrid",
		},
		Networks: []dashboard.NetworkRef{
			{Ref: "wt-level3"},
			{Ref: "mfg-flat"},
		},
		Boundaries: []dashboard.BoundaryDef{
			{
				Between:        []string{"wt-level3", "mfg-flat"},
				State:          "intended",
				Infrastructure: "planned-firewall",
			},
		},
	}
	td := dashboard.BuildTopologyData(def, nil)
	if td == nil {
		t.Fatal("BuildTopologyData returned nil for intended boundary test")
	}
	if len(td.Boundaries) != 1 {
		t.Fatalf("Boundaries: got %d, want 1", len(td.Boundaries))
	}
	b := td.Boundaries[0]
	if b.State != "intended" {
		t.Errorf("Boundary.State: got %q, want %q", b.State, "intended")
	}
	if b.CSSClass != "boundary-intended" {
		t.Errorf("Boundary.CSSClass: got %q, want %q", b.CSSClass, "boundary-intended")
	}
}

// TestBuildTopologyData_AbsentBoundary exercises the "absent" boundary state with mock data.
// Per SOW-019.0 SC-8: the boundary-absent CSS class renders as a red dashed line with
// "NO BOUNDARY" label. Verified via unit test with mock data.
func TestBuildTopologyData_AbsentBoundary(t *testing.T) {
	def := &dashboard.EnvironmentDef{
		Env: dashboard.EnvironmentMeta{
			ID:        "absent-boundary-test",
			Archetype: "hybrid",
		},
		Networks: []dashboard.NetworkRef{
			{Ref: "wt-level2"},
			{Ref: "mfg-flat"},
		},
		Boundaries: []dashboard.BoundaryDef{
			{
				Between: []string{"wt-level2", "mfg-flat"},
				State:   "absent",
			},
		},
	}
	td := dashboard.BuildTopologyData(def, nil)
	if td == nil {
		t.Fatal("BuildTopologyData returned nil for absent boundary test")
	}
	if len(td.Boundaries) != 1 {
		t.Fatalf("Boundaries: got %d, want 1", len(td.Boundaries))
	}
	b := td.Boundaries[0]
	if b.State != "absent" {
		t.Errorf("Boundary.State: got %q, want %q", b.State, "absent")
	}
	if b.CSSClass != "boundary-absent" {
		t.Errorf("Boundary.CSSClass: got %q, want %q", b.CSSClass, "boundary-absent")
	}
}

// TestEraClass verifies decade-based era class assignment.
func TestEraClass(t *testing.T) {
	tests := []struct {
		year      int
		wantClass string
	}{
		{1990, "era-1990s"},
		{1997, "era-1990s"},
		{1999, "era-1990s"},
		{2000, "era-2000s"},
		{2008, "era-2000s"},
		{2009, "era-2000s"},
		{2010, "era-2010s"},
		{2013, "era-2010s"},
		{2019, "era-2010s"},
		{2020, "era-2020s"},
		{2022, "era-2020s"},
	}
	for _, tc := range tests {
		year := tc.year
		got := dashboard.EraClass(&year)
		if got != tc.wantClass {
			t.Errorf("EraClass(%d): got %q, want %q", tc.year, got, tc.wantClass)
		}
	}
}

// TestEraClass_Nil verifies that nil installed returns empty string.
func TestEraClass_Nil(t *testing.T) {
	got := dashboard.EraClass(nil)
	if got != "" {
		t.Errorf("EraClass(nil): got %q, want empty string", got)
	}
}

// TestBuildTopologyData_Bridges verifies that placement bridge definitions are
// extracted into TopologyBridge entries.
func TestBuildTopologyData_Bridges(t *testing.T) {
	def := &dashboard.EnvironmentDef{
		Env: dashboard.EnvironmentMeta{
			ID:        "bridge-test",
			Archetype: "hybrid",
		},
		Networks: []dashboard.NetworkRef{
			{Ref: "ww-flat"},
			{Ref: "ww-serial-bus"},
		},
		Placements: []dashboard.Placement{
			{
				ID:      "ww-gw-01",
				Device:  "moxa-nport-5150",
				Network: "ww-flat",
				IP:      "192.168.10.20",
				ModbusPort: 5063,
				Bridges: []dashboard.BridgeDef{
					{FromNetwork: "ww-flat", ToNetwork: "ww-serial-bus"},
				},
			},
		},
	}
	td := dashboard.BuildTopologyData(def, nil)
	if td == nil {
		t.Fatal("BuildTopologyData returned nil")
	}
	if len(td.Bridges) != 1 {
		t.Fatalf("Bridges: got %d, want 1", len(td.Bridges))
	}
	br := td.Bridges[0]
	if br.DeviceID != "ww-gw-01" {
		t.Errorf("Bridge.DeviceID: got %q, want %q", br.DeviceID, "ww-gw-01")
	}
	if br.FromNetwork != "ww-flat" {
		t.Errorf("Bridge.FromNetwork: got %q, want %q", br.FromNetwork, "ww-flat")
	}
	if br.ToNetwork != "ww-serial-bus" {
		t.Errorf("Bridge.ToNetwork: got %q, want %q", br.ToNetwork, "ww-serial-bus")
	}
}

// TestLevelSort_WANFirst verifies WAN network appears before Purdue levels in sorted output,
// and that a flat unmanaged network appears after all Purdue levels.
// Uses testdata networks: test-flat-net (unmanaged, qualifies as flat in hybrid),
// test-level3 (managed, level3 in ID), and ps-wan-link (wan in ID -> External/WAN).
func TestLevelSort_WANFirst(t *testing.T) {
	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}
	def := &dashboard.EnvironmentDef{
		Env: dashboard.EnvironmentMeta{
			ID:        "sort-test",
			Archetype: "hybrid",
		},
		Networks: []dashboard.NetworkRef{
			{Ref: "test-flat-net"}, // unmanaged ethernet in hybrid -> Flat (Unclassified)
			{Ref: "test-level3"},   // level3 in ID -> Level 3
			{Ref: "ps-wan-link"},   // wan in ID -> External/WAN (atom not in testdata, but ID rule fires)
		},
	}
	td := dashboard.BuildTopologyData(def, lib)
	if td == nil {
		t.Fatal("BuildTopologyData returned nil")
	}
	if len(td.Levels) == 0 {
		t.Fatal("no levels produced")
	}
	// First level should be External/WAN (ps-wan-link matches "wan" ID rule).
	first := td.Levels[0]
	if first.Name != "External/WAN" {
		t.Errorf("first level: got %q, want %q", first.Name, "External/WAN")
	}
	// test-flat-net should be classified as Flat (Unclassified) in hybrid context.
	foundFlat := false
	for _, lvl := range td.Levels {
		if lvl.Name == "Flat (Unclassified)" {
			foundFlat = true
		}
	}
	if !foundFlat {
		t.Error("hybrid environment with unmanaged ethernet should produce 'Flat (Unclassified)' level")
	}
}

// ---- Handler route tests for topology ----

// TestTopologyHandler_Returns200 verifies GET /topology returns 200.
func TestTopologyHandler_Returns200(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/topology", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("GET /topology: status %d, want 200", w.Result().StatusCode)
	}
	if !strings.Contains(w.Body.String(), "Topology") {
		t.Error("topology page should contain 'Topology'")
	}
}

// TestTopologyHandler_ShowsEnvSelector verifies the environment selector is rendered.
func TestTopologyHandler_ShowsEnvSelector(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/topology", nil)
	dash.Routes().ServeHTTP(w, r)

	body := w.Body.String()
	// testdata has test-env, test-hybrid, and test-flat environments
	if !strings.Contains(body, "test-hybrid") {
		t.Error("topology page should show test-hybrid in environment selector")
	}
	if !strings.Contains(body, "test-flat") {
		t.Error("topology page should show test-flat in environment selector")
	}
}

// TestTopologyEnvHandler_ValidEnv verifies GET /topology/{env-id} returns 200 for known env.
func TestTopologyEnvHandler_ValidEnv(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/topology/test-hybrid", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("GET /topology/test-hybrid: status %d, want 200", w.Result().StatusCode)
	}
}

// TestTopologyEnvHandler_NotFound verifies GET /topology/{env-id} returns 404 for unknown env.
func TestTopologyEnvHandler_NotFound(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/topology/nonexistent-env", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("GET /topology/nonexistent-env: status %d, want 404", w.Result().StatusCode)
	}
}

// TestTopologyPartialHandler_ValidEnv verifies HTMX partial for known environment.
func TestTopologyPartialHandler_ValidEnv(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/partials/topology-view/test-hybrid", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("GET /partials/topology-view/test-hybrid: status %d, want 200", w.Result().StatusCode)
	}
	// Partial should not contain the full HTML layout.
	if strings.Contains(w.Body.String(), "<!DOCTYPE html>") {
		t.Error("topology-view partial should not contain full HTML layout")
	}
}

// TestTopologyPartialHandler_NotFound verifies partial returns 404 for unknown env.
func TestTopologyPartialHandler_NotFound(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/partials/topology-view/no-such-env", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("GET /partials/topology-view/no-such-env: status %d, want 404", w.Result().StatusCode)
	}
}

// TestTopologyPage_ContainsNavLink verifies the nav bar includes the Topology link.
func TestTopologyPage_ContainsNavLink(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/topology", nil)
	dash.Routes().ServeHTTP(w, r)

	if !strings.Contains(w.Body.String(), `href="/topology"`) {
		t.Error("topology page should contain nav link to /topology")
	}
}

// TestTopologyPartial_ShowsHybridContent verifies hybrid environment partial content.
func TestTopologyPartial_ShowsHybridContent(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/partials/topology-view/test-hybrid", nil)
	dash.Routes().ServeHTTP(w, r)

	body := w.Body.String()
	// Should show enforced boundary indicator.
	if !strings.Contains(body, "boundary-enforced") {
		t.Error("hybrid environment partial should contain boundary-enforced CSS class")
	}
	// Should show era markers.
	if !strings.Contains(body, "era-2010s") || !strings.Contains(body, "era-1990s") {
		t.Error("hybrid environment partial should contain era markers for 2010s and 1990s devices")
	}
}
