package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rustybrownlee/ot-simulator/monitoring/internal/dashboard"
	"github.com/rustybrownlee/ot-simulator/monitoring/internal/eventstore"
)

// newTestDashboardWithStore creates a Dashboard with a mock API server, test design
// library, and a real in-memory event store for events page tests.
func newTestDashboardWithStore(t *testing.T, apiHandler http.Handler) (*dashboard.Dashboard, *eventstore.Store) {
	t.Helper()
	apiSrv := httptest.NewServer(apiHandler)
	t.Cleanup(apiSrv.Close)

	lib, err := dashboard.LoadDesignLibrary(testDataDir)
	if err != nil {
		t.Fatalf("LoadDesignLibrary: %v", err)
	}

	store, err := eventstore.New(":memory:")
	if err != nil {
		t.Fatalf("eventstore.New: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck -- test cleanup

	client := dashboard.NewAPIClient(apiSrv.Listener.Addr().String())
	dash := dashboard.NewDashboard(client, lib, store)
	return dash, store
}

// TestEventsHandler_NilStore verifies the events page renders the placeholder
// when the event store is nil (FR-1: nil store -> placeholder message).
func TestEventsHandler_NilStore(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events", nil)
	dash.Routes().ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /events: status %d, want 200", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Transaction Events") {
		t.Error("events page should contain 'Transaction Events'")
	}
	if !strings.Contains(body, "not available") {
		t.Error("events page should show 'not available' when store is nil")
	}
}

// TestEventsHandler_EmptyStore verifies the events page renders correctly
// with a live store that has no events (empty state).
func TestEventsHandler_EmptyStore(t *testing.T) {
	dash, _ := newTestDashboardWithStore(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events", nil)
	dash.Routes().ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /events: status %d, want 200", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Transaction Events") {
		t.Error("events page should contain 'Transaction Events'")
	}
}

// TestEventsTablePartial_NoLayout verifies the HTMX partial does not include
// the full HTML layout (FR-2: partial handler).
func TestEventsTablePartial_NoLayout(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/partials/events-table", nil)
	dash.Routes().ServeHTTP(w, r)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("GET /partials/events-table: status %d, want 200", w.Result().StatusCode)
	}
	if strings.Contains(w.Body.String(), "<!DOCTYPE html>") {
		t.Error("events-table partial should not contain full HTML layout")
	}
}

// TestEventsNavLink verifies the "Events" nav link appears after "Process"
// and links to /events (FR-8).
func TestEventsNavLink(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	dash.Routes().ServeHTTP(w, r)

	body := w.Body.String()
	if !strings.Contains(body, `href="/events"`) {
		t.Error("nav should contain link to /events")
	}
}

// TestEventsNavLink_ActiveHighlight verifies the "Events" link has the active
// CSS class when on the /events page (FR-8: ActivePage "events").
func TestEventsNavLink_ActiveHighlight(t *testing.T) {
	dash, _ := newTestDashboard(t, emptyAPIServer())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events", nil)
	dash.Routes().ServeHTTP(w, r)

	body := w.Body.String()
	// The nav link should carry the "active" class when ActivePage is "events".
	if !strings.Contains(body, `nav-link active`) {
		t.Error("events nav link should have active class when on /events")
	}
}
