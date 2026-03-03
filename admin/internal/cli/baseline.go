package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rustybrownlee/ot-simulator/admin/internal/apiclient"
)

// RunBaseline dispatches to the correct baseline subcommand: status, reset.
func RunBaseline(g Globals, args []string) {
	if len(args) == 0 {
		printBaselineUsage()
		os.Exit(1)
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "status":
		runBaselineStatus(g, rest)
	case "reset":
		runBaselineReset(g, rest)
	default:
		fmt.Fprintf(os.Stderr, "admin baseline: unknown subcommand %q\n\n", sub)
		printBaselineUsage()
		os.Exit(1)
	}
}

// runBaselineStatus displays the per-device baseline learning state.
func runBaselineStatus(g Globals, _ []string) {
	client := apiclient.New(g.APIAddr)

	baselines, err := client.GetBaselines()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"admin baseline status: monitoring API unreachable at %s: %v\n", g.APIAddr, err)
		os.Exit(1)
	}

	if len(baselines) == 0 {
		fmt.Println("No baseline data available. Monitoring may not have started polling yet.")
		return
	}

	printBaselineTable(baselines)
}

// printBaselineTable renders baseline status as an aligned table.
func printBaselineTable(baselines map[string]*apiclient.BaselineEntry) {
	tp := NewTablePrinter(os.Stdout,
		"Device ID", "Status", "Samples", "Required", "Registers")

	// Sort by device ID for stable output.
	ids := make([]string, 0, len(baselines))
	for id := range baselines {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		b := baselines[id]
		tp.AddRow(
			id,
			b.Status,
			fmt.Sprintf("%d", b.SampleCount),
			fmt.Sprintf("%d", b.RequiredSamples),
			fmt.Sprintf("%d", b.RegisterCount),
		)
	}
	tp.Print()
}

// runBaselineReset triggers baseline re-learning for all devices or a single device.
func runBaselineReset(g Globals, args []string) {
	fs := flag.NewFlagSet("baseline-reset", flag.ExitOnError)
	deviceID := fs.String("device", "", "reset baseline for a single device ID")
	force := fs.Bool("force", false, "skip confirmation prompt (required for full reset)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	client := apiclient.New(g.APIAddr)

	if *deviceID != "" {
		runResetSingleDevice(client, *deviceID)
		return
	}

	runResetAllDevices(client, *force)
}

// runResetSingleDevice resets one device's baseline.
// Single-device reset does not require --force because the detection gap
// is limited to one device, not the full plant.
func runResetSingleDevice(client *apiclient.Client, deviceID string) {
	resp, err := client.ResetDeviceBaseline(deviceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin baseline reset: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Baseline reset for device %q: %s\n", deviceID, resp.Message)
}

// runResetAllDevices resets all device baselines.
// Requires --force or interactive confirmation because the full reset suspends
// anomaly detection for all devices during the re-learning period.
func runResetAllDevices(client *apiclient.Client, force bool) {
	fmt.Fprintln(os.Stderr,
		"WARNING: Baseline reset triggers a learning period. Anomaly detection alerts")
	fmt.Fprintln(os.Stderr,
		"         are suppressed during this window. This is operationally equivalent")
	fmt.Fprintln(os.Stderr,
		"         to disabling the IDS for all monitored devices.")

	if !force {
		fmt.Print("Type 'yes' to confirm full baseline reset: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		if strings.TrimSpace(scanner.Text()) != "yes" {
			fmt.Println("Aborted.")
			os.Exit(0)
		}
	}

	resp, err := client.ResetAllBaselines()
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin baseline reset: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Full baseline reset: %s (%d devices)\n", resp.Message, resp.Devices)
}

// printBaselineUsage prints baseline subcommand usage to stderr.
func printBaselineUsage() {
	fmt.Fprintln(os.Stderr, `Usage: admin baseline <subcommand> [flags]

Subcommands:
  status                          Display per-device baseline learning status
  reset [--device ID] [--force]   Trigger baseline re-learning
                                  Full reset requires --force or confirmation prompt`)
}
