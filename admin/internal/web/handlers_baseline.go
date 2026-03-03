package web

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"

	"github.com/rustybrownlee/ot-simulator/admin/internal/apiclient"
)

// baselineRow is one row in the baseline status table.
type baselineRow struct {
	DeviceID        string
	Status          string
	SampleCount     int
	RequiredSamples int
	RegisterCount   int
}

// baselinePageData is the template data for the Baseline Status (/baseline) page.
type baselinePageData struct {
	Title       string
	ActivePage  string
	APIAddr     string
	Rows        []baselineRow
	Established int
	Learning    int
	Total       int
	APIError    string
}

// baselineHandler renders the full Baseline Status (/baseline) page.
func (s *Server) baselineHandler(w http.ResponseWriter, r *http.Request) {
	data := s.buildBaselineData()
	s.render(w, "baseline.html", data)
}

// baselineTableHandler returns the HTMX partial for the baseline table.
func (s *Server) baselineTableHandler(w http.ResponseWriter, r *http.Request) {
	data := s.buildBaselineData()
	s.renderPartial(w, "baseline_table_content", data)
}

// buildBaselineData assembles baseline status from the monitoring API.
// When the API is unreachable, APIError is set and Rows is empty (graceful degradation).
func (s *Server) buildBaselineData() baselinePageData {
	data := baselinePageData{Title: "Baseline Status", ActivePage: "baseline", APIAddr: s.globals.APIAddr}
	client := apiclient.New(s.globals.APIAddr)
	baselines, err := client.GetBaselines()
	if err != nil {
		data.APIError = fmt.Sprintf("Monitoring API unreachable at %s: %v", s.globals.APIAddr, err)
		return data
	}

	ids := make([]string, 0, len(baselines))
	for id := range baselines {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		b := baselines[id]
		data.Rows = append(data.Rows, baselineRow{
			DeviceID:        id,
			Status:          b.Status,
			SampleCount:     b.SampleCount,
			RequiredSamples: b.RequiredSamples,
			RegisterCount:   b.RegisterCount,
		})
		if b.Status == "established" {
			data.Established++
		} else {
			data.Learning++
		}
	}
	data.Total = len(baselines)
	return data
}

// baselineResetHandler handles POST /baseline/reset?device=<id>.
// Resets a single device's baseline and returns the updated table partial.
func (s *Server) baselineResetHandler(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device")
	if deviceID == "" {
		http.Error(w, "device parameter required", http.StatusBadRequest)
		return
	}
	client := apiclient.New(s.globals.APIAddr)
	_, err := client.ResetDeviceBaseline(deviceID)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<div class="alert alert-danger">Reset failed: %s</div>`,
			template.HTMLEscapeString(err.Error()))
		return
	}
	data := s.buildBaselineData()
	s.renderPartial(w, "baseline_table_content", data)
}

// baselineResetAllHandler handles POST /baseline/reset-all.
// Resets all device baselines and returns the updated table partial.
func (s *Server) baselineResetAllHandler(w http.ResponseWriter, r *http.Request) {
	client := apiclient.New(s.globals.APIAddr)
	_, err := client.ResetAllBaselines()
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<div class="alert alert-danger">Reset all failed: %s</div>`,
			template.HTMLEscapeString(err.Error()))
		return
	}
	data := s.buildBaselineData()
	s.renderPartial(w, "baseline_table_content", data)
}
