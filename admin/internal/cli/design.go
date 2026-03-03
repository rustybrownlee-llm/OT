package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rustybrownlee/ot-simulator/admin/internal/schema"
	"gopkg.in/yaml.v3"
)

// RunDesign dispatches to the correct design subcommand: validate, list.
func RunDesign(g Globals, args []string) {
	if len(args) == 0 {
		printDesignUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "validate":
		runDesignValidate(g, args[1:])
	case "list":
		runDesignList(g, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "admin design: unknown subcommand %q\n\n", args[0])
		printDesignUsage()
		os.Exit(1)
	}
}

// runDesignValidate implements "admin design validate <path>".
func runDesignValidate(g Globals, args []string) {
	fs := flag.NewFlagSet("design-validate", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "show schema description for each error")
	crossRefsOnly := fs.Bool("cross-refs-only", false, "skip schema validation, run cross-reference checks only")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "admin design validate: path argument required")
		os.Exit(1)
	}

	targetPath := fs.Arg(0)
	info, err := os.Stat(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin design validate: %v\n", err)
		os.Exit(1)
	}

	schemasDir := filepath.Join(g.DesignDir, "schemas")
	schemas, err := schema.Load(schemasDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin design validate: loading schemas: %v\n", err)
		os.Exit(1)
	}

	var exitCode int
	if info.IsDir() {
		exitCode = validateEnvironmentDir(targetPath, g.DesignDir, schemas, *verbose, *crossRefsOnly)
	} else {
		exitCode = validateSingleFile(targetPath, g.DesignDir, schemas, *verbose, *crossRefsOnly)
	}
	os.Exit(exitCode)
}

// validateSingleFile validates a single YAML file against its inferred schema.
func validateSingleFile(filePath, designDir string, schemas *schema.SchemaSet, verbose, crossRefsOnly bool) int {
	if crossRefsOnly {
		fmt.Fprintln(os.Stderr, "admin design validate: --cross-refs-only requires an environment directory path")
		return 1
	}

	result, err := schema.ValidateFile(filePath, designDir, schemas)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin design validate: %v\n", err)
		return 1
	}

	if result.OK() {
		fmt.Printf("%s: OK\n", filePath)
		return 0
	}

	printValidationErrors(result.Errors, verbose)
	return 1
}

// validateEnvironmentDir validates an entire environment directory.
func validateEnvironmentDir(
	envDir, designDir string, schemas *schema.SchemaSet, verbose, crossRefsOnly bool,
) int {
	envName := filepath.Base(envDir)
	fmt.Printf("Validating environment: %s\n\n", envName)

	envFile := filepath.Join(envDir, "environment.yaml")
	procFile := filepath.Join(envDir, "process.yaml")

	total, passed := 0, 0
	hasErrors := false

	if !crossRefsOnly {
		total, passed, hasErrors = validateEnvSchemaFiles(envFile, procFile, designDir, schemas, verbose, total, passed, hasErrors)
	}

	// Cross-reference validation.
	crossResult, err := schema.ValidateCrossRefs(envDir, designDir)
	total++
	if err != nil {
		fmt.Printf("  %-32s FAIL\n", "Cross-references")
		fmt.Fprintf(os.Stderr, "    cross-reference check error: %v\n", err)
		hasErrors = true
	} else if crossResult.OK() {
		fmt.Printf("  %-32s OK\n", "Cross-references")
		passed++
	} else {
		fmt.Printf("  %-32s FAIL\n", "Cross-references")
		for _, e := range crossResult.Errors {
			fmt.Printf("    %s\n", e.String())
		}
		hasErrors = true
	}

	if !crossRefsOnly {
		total, passed, hasErrors = validateReferencedDevicesAndNetworks(envDir, designDir, schemas, verbose, total, passed, hasErrors)
	}

	fmt.Println()
	if hasErrors {
		fmt.Printf("  Result: %d/%d passed, %d errors\n", passed, total, total-passed)
		return 1
	}
	fmt.Printf("  Result: %d/%d passed, 0 errors\n", total, total)
	return 0
}

// validateEnvSchemaFiles validates environment.yaml and process.yaml against their schemas.
func validateEnvSchemaFiles(
	envFile, procFile, designDir string, schemas *schema.SchemaSet,
	verbose bool, total, passed int, hasErrors bool,
) (int, int, bool) {
	total++
	envResult, err := schema.ValidateFile(envFile, designDir, schemas)
	if err != nil || !envResult.OK() {
		fmt.Printf("  %-32s FAIL (schema)\n", "environment.yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "    %v\n", err)
		} else {
			printValidationErrors(envResult.Errors, verbose)
		}
		hasErrors = true
	} else {
		fmt.Printf("  %-32s OK (schema)\n", "environment.yaml")
		passed++
	}

	if _, statErr := os.Stat(procFile); statErr == nil {
		total++
		procResult, err := schema.ValidateFile(procFile, designDir, schemas)
		if err != nil || !procResult.OK() {
			fmt.Printf("  %-32s FAIL (schema)\n", "process.yaml")
			if err != nil {
				fmt.Fprintf(os.Stderr, "    %v\n", err)
			} else {
				printValidationErrors(procResult.Errors, verbose)
			}
			hasErrors = true
		} else {
			fmt.Printf("  %-32s OK (schema)\n", "process.yaml")
			passed++
		}
	}

	return total, passed, hasErrors
}

// validateReferencedDevicesAndNetworks validates unique device and network atoms
// referenced from the environment's placements and networks arrays.
func validateReferencedDevicesAndNetworks(
	envDir, designDir string, schemas *schema.SchemaSet,
	verbose bool, total, passed int, hasErrors bool,
) (int, int, bool) {
	devices, networks := collectEnvRefs(envDir)

	for _, deviceID := range devices {
		total++
		path := filepath.Join(designDir, "devices", deviceID+".yaml")
		label := fmt.Sprintf("Device: %s", deviceID)
		result, err := schema.ValidateFileWithSchema(path, schemas.DeviceAtom)
		if err != nil || !result.OK() {
			fmt.Printf("  %-32s FAIL (schema)\n", label)
			if err != nil {
				fmt.Fprintf(os.Stderr, "    %v\n", err)
			} else {
				printValidationErrors(result.Errors, verbose)
			}
			hasErrors = true
		} else {
			fmt.Printf("  %-32s OK (schema)\n", label)
			passed++
		}
	}

	for _, networkID := range networks {
		total++
		path := filepath.Join(designDir, "networks", networkID+".yaml")
		label := fmt.Sprintf("Network: %s", networkID)
		result, err := schema.ValidateFileWithSchema(path, schemas.NetworkAtom)
		if err != nil || !result.OK() {
			fmt.Printf("  %-32s FAIL (schema)\n", label)
			if err != nil {
				fmt.Fprintf(os.Stderr, "    %v\n", err)
			} else {
				printValidationErrors(result.Errors, verbose)
			}
			hasErrors = true
		} else {
			fmt.Printf("  %-32s OK (schema)\n", label)
			passed++
		}
	}

	return total, passed, hasErrors
}

// printValidationErrors formats and prints schema validation errors to stdout.
func printValidationErrors(errors []*schema.ValidationError, verbose bool) {
	for _, e := range errors {
		fmt.Printf("    %s\n", e.String())
		if verbose && e.Message != "" {
			fmt.Printf("      expected: %s\n", e.Message)
		}
	}
}

// collectEnvRefs parses environment.yaml to extract unique device and network IDs.
func collectEnvRefs(envDir string) (devices, networks []string) {
	type envSummary struct {
		Networks []struct {
			Ref string `yaml:"ref"`
		} `yaml:"networks"`
		Placements []struct {
			Device string `yaml:"device"`
		} `yaml:"placements"`
	}

	data, err := os.ReadFile(filepath.Join(envDir, "environment.yaml"))
	if err != nil {
		return nil, nil
	}
	var env envSummary
	if err := yaml.Unmarshal(data, &env); err != nil {
		return nil, nil
	}

	devSet := make(map[string]bool)
	netSet := make(map[string]bool)

	for _, p := range env.Placements {
		devSet[p.Device] = true
	}
	for _, n := range env.Networks {
		netSet[n.Ref] = true
	}

	for d := range devSet {
		devices = append(devices, d)
	}
	for n := range netSet {
		networks = append(networks, n)
	}
	sort.Strings(devices)
	sort.Strings(networks)
	return devices, networks
}

// runDesignList implements "admin design list".
func runDesignList(g Globals, args []string) {
	designDir := g.DesignDir
	fmt.Printf("Design Layer Elements (%s)\n", designDir)

	printDeviceList(designDir)
	printNetworkList(designDir)
	printEnvironmentList(designDir)
}

// printDeviceList reads all device atoms and prints a formatted table.
func printDeviceList(designDir string) {
	devicesDir := filepath.Join(designDir, "devices")
	entries, err := os.ReadDir(devicesDir)
	if err != nil {
		fmt.Printf("\nDevices (error reading %s: %v)\n", devicesDir, err)
		return
	}

	type deviceRow struct {
		ID       string
		Vendor   string
		Model    string
		Category string
	}

	var rows []deviceRow
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		row := parseDeviceRow(filepath.Join(devicesDir, e.Name()))
		rows = append(rows, row)
	}

	fmt.Printf("\nDevices (%d):\n", len(rows))
	tp := NewTablePrinter(os.Stdout, "ID", "Vendor", "Model", "Category")
	for _, r := range rows {
		tp.AddRow(r.ID, r.Vendor, r.Model, r.Category)
	}
	tp.Print()
}

// parseDeviceRow extracts display fields from a device atom YAML file.
func parseDeviceRow(path string) struct {
	ID       string
	Vendor   string
	Model    string
	Category string
} {
	type deviceAtomDisplay struct {
		Device struct {
			ID       string `yaml:"id"`
			Vendor   string `yaml:"vendor"`
			Model    string `yaml:"model"`
			Category string `yaml:"category"`
		} `yaml:"device"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return struct{ ID, Vendor, Model, Category string }{filepath.Base(path), "", "", ""}
	}
	var d deviceAtomDisplay
	_ = yaml.Unmarshal(data, &d)
	return struct{ ID, Vendor, Model, Category string }{d.Device.ID, d.Device.Vendor, d.Device.Model, d.Device.Category}
}

// printNetworkList reads all network atoms and prints a formatted table.
func printNetworkList(designDir string) {
	networksDir := filepath.Join(designDir, "networks")
	entries, err := os.ReadDir(networksDir)
	if err != nil {
		fmt.Printf("\nNetworks (error reading %s: %v)\n", networksDir, err)
		return
	}

	type netRow struct {
		ID     string
		Type   string
		Subnet string
	}

	var rows []netRow
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		row := parseNetworkRow(filepath.Join(networksDir, e.Name()))
		rows = append(rows, row)
	}

	fmt.Printf("\nNetworks (%d):\n", len(rows))
	tp := NewTablePrinter(os.Stdout, "ID", "Type", "Subnet")
	for _, r := range rows {
		tp.AddRow(r.ID, r.Type, r.Subnet)
	}
	tp.Print()
}

// parseNetworkRow extracts display fields from a network atom YAML file.
func parseNetworkRow(path string) struct {
	ID     string
	Type   string
	Subnet string
} {
	type networkAtomDisplay struct {
		Network struct {
			ID   string `yaml:"id"`
			Type string `yaml:"type"`
		} `yaml:"network"`
		Properties struct {
			Subnet string `yaml:"subnet"`
		} `yaml:"properties"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return struct{ ID, Type, Subnet string }{filepath.Base(path), "", "-"}
	}
	var n networkAtomDisplay
	_ = yaml.Unmarshal(data, &n)
	subnet := n.Properties.Subnet
	if subnet == "" {
		subnet = "-"
	}
	return struct{ ID, Type, Subnet string }{n.Network.ID, n.Network.Type, subnet}
}

// printEnvironmentList reads all environment directories and prints a formatted table.
func printEnvironmentList(designDir string) {
	envsDir := filepath.Join(designDir, "environments")
	entries, err := os.ReadDir(envsDir)
	if err != nil {
		fmt.Printf("\nEnvironments (error reading %s: %v)\n", envsDir, err)
		return
	}

	type envRow struct {
		ID         string
		Devices    string
		Networks   string
		HasProcess string
	}

	var rows []envRow
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		row := parseEnvironmentRow(filepath.Join(envsDir, e.Name()))
		rows = append(rows, row)
	}

	fmt.Printf("\nEnvironments (%d):\n", len(rows))
	tp := NewTablePrinter(os.Stdout, "ID", "Devices", "Networks", "Has Process")
	for _, r := range rows {
		tp.AddRow(r.ID, r.Devices, r.Networks, r.HasProcess)
	}
	tp.Print()
}

// parseEnvironmentRow extracts summary fields from an environment directory.
func parseEnvironmentRow(envDir string) struct {
	ID         string
	Devices    string
	Networks   string
	HasProcess string
} {
	type envSummary struct {
		Environment struct {
			ID string `yaml:"id"`
		} `yaml:"environment"`
		Networks []struct {
			Ref string `yaml:"ref"`
		} `yaml:"networks"`
		Placements []struct {
			Device string `yaml:"device"`
		} `yaml:"placements"`
	}

	envFile := filepath.Join(envDir, "environment.yaml")
	data, err := os.ReadFile(envFile)
	if err != nil {
		return struct{ ID, Devices, Networks, HasProcess string }{filepath.Base(envDir), "?", "?", "?"}
	}
	var env envSummary
	_ = yaml.Unmarshal(data, &env)

	devSet := make(map[string]bool)
	for _, p := range env.Placements {
		devSet[p.Device] = true
	}

	hasProcess := "no"
	if _, err := os.Stat(filepath.Join(envDir, "process.yaml")); err == nil {
		hasProcess = "yes"
	}

	return struct{ ID, Devices, Networks, HasProcess string }{
		ID:         env.Environment.ID,
		Devices:    fmt.Sprintf("%d", len(devSet)),
		Networks:   fmt.Sprintf("%d", len(env.Networks)),
		HasProcess: hasProcess,
	}
}

// printDesignUsage prints design subcommand usage to stderr.
func printDesignUsage() {
	fmt.Fprintln(os.Stderr, `Usage: admin design <subcommand> [flags]

Subcommands:
  validate <path> [--verbose] [--cross-refs-only]
                      Validate a single design YAML file or an entire environment directory.
                      --verbose: show schema descriptions for each error
                      --cross-refs-only: skip schema validation, check only cross-references
  list                List all design layer elements (devices, networks, environments)`)
}
