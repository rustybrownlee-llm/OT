package cli

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rustybrownlee/ot-simulator/admin/internal/apiclient"
)

// portType classifies a Modbus TCP port by the device category it serves.
// PLCs (Programmable Logic Controllers) are distinguished from serial-to-Ethernet
// Gateways (e.g., Moxa NPort) and cellular Modems (e.g., Cradlepoint).
// This teaches operators the topological difference between reaching a PLC
// directly versus through a serial gateway or cellular link.
//
// Port ranges follow the quickstart-example environment layout:
//   - 5020-5029: Water treatment PLCs (direct Ethernet-native PLCs)
//   - 5030-5039: Manufacturing gateways (Moxa NPort serial bridges)
//   - 5040-5049: Power systems PLCs (direct Ethernet-native)
//   - 5050-5059: Wastewater PLCs
//   - 5060-5065: Modems and vendor access (Cradlepoint cellular)
var portTypeRanges = []portTypeRange{
	{low: 5020, high: 5029, label: "PLC"},
	{low: 5030, high: 5039, label: "Gateway"},
	{low: 5040, high: 5049, label: "PLC"},
	{low: 5050, high: 5059, label: "PLC"},
	{low: 5060, high: 5065, label: "Modem"},
}

type portTypeRange struct {
	low, high int
	label     string
}

// portLabel returns the OT-aware device type label for a given port number.
// Returns "Device" as a generic fallback if the port is not in a known range.
func portLabel(port int) string {
	for _, r := range portTypeRanges {
		if port >= r.low && port <= r.high {
			return r.label
		}
	}
	return "Device"
}

// portResult holds the TCP probe outcome for one port.
type portResult struct {
	port    int
	label   string
	online  bool
	elapsed time.Duration
}

// RunHealth executes the "admin health" command.
// It probes all configured plant ports, the monitoring API, and the event DB,
// then prints a summary table.
// Exit code 0 = all services reachable; exit code 1 = any service unreachable.
// This follows OT convention: a degraded plant is not a "healthy" condition.
func RunHealth(g Globals, args []string) {
	ports := parsePorts(g.PlantPorts)
	results := probePortsParallel(ports)

	apiStatus, apiDetail := probeAPI(g.APIAddr)
	dbStatus, dbDetail := probeDB(g)

	printHealthTable(results, apiStatus, apiDetail, dbStatus, dbDetail)

	if !allOnline(results, apiStatus, dbStatus) {
		os.Exit(1)
	}
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

// probePortsParallel checks all ports concurrently with a 2-second timeout.
func probePortsParallel(ports []int) []portResult {
	results := make([]portResult, len(ports))
	var wg sync.WaitGroup

	for i, p := range ports {
		wg.Add(1)
		go func(idx, port int) {
			defer wg.Done()
			results[idx] = probePort(port)
		}(i, p)
	}

	wg.Wait()
	return results
}

// probePort attempts a TCP connection to localhost:port with a 2-second timeout.
func probePort(port int) portResult {
	addr := fmt.Sprintf("localhost:%d", port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	elapsed := time.Since(start)

	online := err == nil
	if conn != nil {
		conn.Close()
	}

	return portResult{
		port:    port,
		label:   portLabel(port),
		online:  online,
		elapsed: elapsed,
	}
}

// probeAPI calls the monitoring API health endpoint.
// Returns status string and detail string for the summary table.
func probeAPI(apiAddr string) (string, string) {
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

	detail := fmt.Sprintf("%s, %d devices online",
		health.Status,
		health.DevicesOnline,
	)
	return "online", detail
}

// probeDB checks whether the event database file exists and is readable.
func probeDB(g Globals) (string, string) {
	dbPath := effectiveDBPath(g)
	if dbPath == "" {
		return "unknown", "no database path configured"
	}

	info, err := os.Stat(dbPath)
	if err != nil {
		return "offline", fmt.Sprintf("not found: %s", dbPath)
	}

	sizeMB := float64(info.Size()) / (1024 * 1024)
	return "online", fmt.Sprintf("%.1f MB, %s", sizeMB, dbPath)
}

// printHealthTable renders the health summary to stdout.
func printHealthTable(results []portResult, apiStatus, apiDetail, dbStatus, dbDetail string) {
	tp := NewTablePrinter(os.Stdout, "Service", "Status", "Details")

	for _, r := range results {
		svc := fmt.Sprintf("%s %d", r.label, r.port)
		status := "offline"
		detail := "TCP connection refused"
		if r.online {
			status = "online"
			detail = fmt.Sprintf("%.0fms", float64(r.elapsed.Milliseconds()))
		}
		tp.AddRow(svc, status, detail)
	}

	tp.AddRow("Monitor API", apiStatus, apiDetail)
	tp.AddRow("Event DB", dbStatus, dbDetail)
	tp.Print()
}

// allOnline returns true only when all port probes and service checks passed.
func allOnline(results []portResult, apiStatus, dbStatus string) bool {
	for _, r := range results {
		if !r.online {
			return false
		}
	}
	return apiStatus == "online" && dbStatus == "online"
}

