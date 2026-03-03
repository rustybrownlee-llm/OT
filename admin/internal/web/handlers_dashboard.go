package web

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rustybrownlee/ot-simulator/admin/internal/apiclient"
	"github.com/rustybrownlee/ot-simulator/admin/internal/configparse"
	"github.com/rustybrownlee/ot-simulator/admin/internal/dbutil"
)

// healthCardGroup holds one group of health status entries for the dashboard.
type healthCardGroup struct {
	Label   string
	Entries []healthEntry
	Online  int
	Offline int
}

// healthEntry represents one item in a health card group.
type healthEntry struct {
	Label   string
	Status  string // "online" | "offline" | "unknown"
	Detail  string
}

// dashboardData is the template data for the dashboard (/) page.
type dashboardData struct {
	Title      string
	ActivePage string
	DeviceGroup   healthCardGroup
	ServiceGroup  healthCardGroup
	TotalEvents   int64
	DBSizeMB      float64
	NewestEventAge string
	BaselineSummary string
	Error         string
}

// dashboardHandler renders the full dashboard (/) page.
func (s *Server) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	data := s.buildDashboardData()
	s.render(w, "dashboard.html", data)
}

// healthCardsHandler returns the HTMX partial for the health cards section.
func (s *Server) healthCardsHandler(w http.ResponseWriter, r *http.Request) {
	data := s.buildDashboardData()
	s.renderPartial(w, "health_cards_content", data)
}

// buildDashboardData assembles all data needed by the dashboard page.
// PROTOTYPE-DEBT: [td-admin-112] Port probing runs on every HTMX refresh (every 10s).
func (s *Server) buildDashboardData() dashboardData {
	data := dashboardData{
		Title:      "Dashboard",
		ActivePage: "dashboard",
	}
	data.DeviceGroup = s.buildDeviceHealthGroup()
	data.ServiceGroup = s.buildServiceHealthGroup()
	s.fillQuickStats(&data)
	return data
}

// buildDeviceHealthGroup probes plant Modbus TCP ports and builds the device card group.
func (s *Server) buildDeviceHealthGroup() healthCardGroup {
	ports := parsePorts(s.globals.PlantPorts)
	results := probePortsParallel(ports)

	g := healthCardGroup{Label: "Device Availability"}
	for _, r := range results {
		pd := s.design.lookupPort(r.port)
		label := fmt.Sprintf("Device %d", r.port)
		if pd != nil {
			label = fmt.Sprintf("%s (%s)", pd.PlaceID, pd.Category)
		}
		status := "offline"
		detail := "TCP connection refused"
		if r.online {
			status = "online"
			detail = fmt.Sprintf("%.0fms", float64(r.elapsed.Milliseconds()))
			g.Online++
		} else {
			g.Offline++
		}
		g.Entries = append(g.Entries, healthEntry{Label: label, Status: status, Detail: detail})
	}
	return g
}

// buildServiceHealthGroup checks the monitoring API and event DB status.
func (s *Server) buildServiceHealthGroup() healthCardGroup {
	g := healthCardGroup{Label: "Platform Services"}

	apiStatus, apiDetail := probeAPIService(s.globals.APIAddr)
	if apiStatus == "online" {
		g.Online++
	} else {
		g.Offline++
	}
	g.Entries = append(g.Entries, healthEntry{Label: "Monitoring API", Status: apiStatus, Detail: apiDetail})

	dbPath := effectiveDBPath(s.globals)
	dbStatus, dbDetail := probeDBService(dbPath)
	if dbStatus == "online" {
		g.Online++
	} else {
		g.Offline++
	}
	g.Entries = append(g.Entries, healthEntry{Label: "Event Database", Status: dbStatus, Detail: dbDetail})
	return g
}

// fillQuickStats populates DB stats and baseline summary in-place on dashboardData.
func (s *Server) fillQuickStats(data *dashboardData) {
	dbPath := effectiveDBPath(s.globals)
	retention := retentionDays(s.globals)
	stats, err := dbutil.Stats(dbPath, retention)
	if err == nil {
		data.TotalEvents = stats.EventCount
		data.DBSizeMB = float64(stats.FileSizeBytes) / (1024 * 1024)
		if stats.NewestEvent != nil {
			age := time.Since(*stats.NewestEvent)
			data.NewestEventAge = formatAge(age)
		} else {
			data.NewestEventAge = "no events"
		}
	}

	client := apiclient.New(s.globals.APIAddr)
	baselines, err := client.GetBaselines()
	if err == nil {
		established, learning := 0, 0
		for _, b := range baselines {
			if b.Status == "established" {
				established++
			} else {
				learning++
			}
		}
		data.BaselineSummary = fmt.Sprintf("%d established / %d learning", established, learning)
	} else {
		data.BaselineSummary = "monitoring API unreachable"
	}
}

// probeAPIService checks the monitoring API health endpoint.
func probeAPIService(apiAddr string) (string, string) {
	url := fmt.Sprintf("http://%s/api/health", apiAddr)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "offline", "unreachable"
	}
	defer resp.Body.Close()
	health, err := apiclient.ParseHealthResponse(resp.Body)
	if err != nil {
		return "online", fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return "online", fmt.Sprintf("%s, %d devices online", health.Status, health.DevicesOnline)
}

// probeDBService checks whether the event database file exists and is accessible.
func probeDBService(dbPath string) (string, string) {
	if dbPath == "" {
		return "unknown", "no database path configured"
	}
	stats, err := dbutil.Stats(dbPath, configparse.DefaultEventRetentionDays)
	if err != nil {
		return "offline", "not found or inaccessible"
	}
	sizeMB := float64(stats.FileSizeBytes) / (1024 * 1024)
	return "online", fmt.Sprintf("%.1f MB, %d events", sizeMB, stats.EventCount)
}

// portProbeResult holds the TCP probe outcome for one port.
type portProbeResult struct {
	port    int
	online  bool
	elapsed time.Duration
}

// probePortsParallel checks all ports concurrently with a 2-second timeout.
func probePortsParallel(ports []int) []portProbeResult {
	results := make([]portProbeResult, len(ports))
	var wg sync.WaitGroup
	for i, p := range ports {
		wg.Add(1)
		go func(idx, port int) {
			defer wg.Done()
			addr := fmt.Sprintf("localhost:%d", port)
			start := time.Now()
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			elapsed := time.Since(start)
			if conn != nil {
				conn.Close()
			}
			results[idx] = portProbeResult{port: port, online: err == nil, elapsed: elapsed}
		}(i, p)
	}
	wg.Wait()
	return results
}

// parsePorts converts a comma-separated port list string to a slice of ints.
func parsePorts(portList string) []int {
	var ports []int
	for _, s := range strings.Split(portList, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		p, err := strconv.Atoi(s)
		if err == nil && p > 0 {
			ports = append(ports, p)
		}
	}
	return ports
}

// formatAge formats a duration as a human-readable age string.
func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%.0fs ago", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%.0fm ago", d.Minutes())
	default:
		return fmt.Sprintf("%.1fh ago", d.Hours())
	}
}
