package web

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rustybrownlee/ot-simulator/admin/internal/dbutil"
)

// dbPageData is the template data for the Event Database (/db) page.
type dbPageData struct {
	Title      string
	ActivePage string
	Stats      *dbutil.DBStatus
	StatsError string
}

// dbHandler renders the full Event Database (/db) page.
func (s *Server) dbHandler(w http.ResponseWriter, r *http.Request) {
	data := s.buildDBData()
	s.render(w, "db.html", data)
}

// dbStatsHandler returns the HTMX partial for the DB stats panel.
func (s *Server) dbStatsHandler(w http.ResponseWriter, r *http.Request) {
	data := s.buildDBData()
	s.renderPartial(w, "db_stats_content", data)
}

// buildDBData assembles the data for the DB page and stats partial.
func (s *Server) buildDBData() dbPageData {
	data := dbPageData{Title: "Event Database", ActivePage: "db"}
	dbPath := effectiveDBPath(s.globals)
	retention := retentionDays(s.globals)
	stats, err := dbutil.Stats(dbPath, retention)
	if err != nil {
		data.StatsError = fmt.Sprintf("database not accessible: %v", err)
		return data
	}
	data.Stats = stats
	return data
}

// dbPruneHandler handles POST /db/prune. Deletes events older than N days
// and returns an updated stats partial as the HTMX response.
func (s *Server) dbPruneHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	days, err := strconv.Atoi(r.FormValue("days"))
	if err != nil || days < 1 {
		days = retentionDays(s.globals)
	}

	dbPath := effectiveDBPath(s.globals)
	cutoff := time.Now().AddDate(0, 0, -days)

	n, pruneErr := dbutil.Prune(dbPath, cutoff)

	data := s.buildDBData()
	if pruneErr != nil {
		data.StatsError = fmt.Sprintf("prune failed: %v", pruneErr)
	} else {
		data.StatsError = fmt.Sprintf("Deleted %d events older than %d days.", n, days)
	}
	s.renderPartial(w, "db_stats_content", data)
}

// dbExportHandler handles GET /db/export. Returns a file download in CSV or JSON format.
// Supports query params: format (csv|json), device, after (RFC3339), before (RFC3339).
//
// PROTOTYPE-DEBT: [td-admin-113] No pagination. 100K row limit from dbutil.Export.
func (s *Server) dbExportHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	format := q.Get("format")
	if format != "json" {
		format = "csv"
	}

	opts := dbutil.ExportOptions{Format: format, DeviceID: q.Get("device")}

	if afterStr := q.Get("after"); afterStr != "" {
		t, err := time.Parse(time.RFC3339, afterStr)
		if err == nil {
			opts.After = &t
		}
	}
	if beforeStr := q.Get("before"); beforeStr != "" {
		t, err := time.Parse(time.RFC3339, beforeStr)
		if err == nil {
			opts.Before = &t
		}
	}

	filename := fmt.Sprintf("events-%s.%s", time.Now().Format("20060102-150405"), format)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	}

	opts.Output = w
	dbPath := effectiveDBPath(s.globals)
	_, _, err := dbutil.Export(dbPath, opts)
	if err != nil {
		// Cannot write headers at this point; log the error.
		fmt.Fprintf(w, "\n# export error: %v\n", err)
	}
}
