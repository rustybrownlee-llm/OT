// Command ot-design provides design layer validation and scaffolding for the OT simulator.
//
// Usage:
//
//	ot-design validate <path>           Validate a device atom, network atom, or environment
//	ot-design scaffold --device         Write a device atom skeleton to stdout
//	ot-design scaffold --network        Write a network atom skeleton to stdout
//	ot-design scaffold --environment    Write an environment skeleton to stdout
//	ot-design --help                    Print usage information
//
// Exit codes:
//
//	0 - All validations passed (or scaffold output written successfully)
//	1 - One or more validation errors found
//	2 - Usage error (bad arguments, file not found, unparseable YAML, design root not found)
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rustybrownlee/ot-simulator/design/internal/scaffold"
	"github.com/rustybrownlee/ot-simulator/design/internal/validate"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "validate":
		os.Exit(runValidate(os.Args[2:]))
	case "scaffold":
		os.Exit(runScaffold(os.Args[2:]))
	case "--help", "-h", "help":
		printUsage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "ot-design: unknown subcommand %q\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

// runValidate validates the target path and returns an exit code.
func runValidate(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "ot-design validate: path argument is required")
		fmt.Fprintln(os.Stderr, "Usage: ot-design validate <path>")
		return 2
	}

	target := args[0]
	info, err := os.Stat(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ot-design validate: %v\n", err)
		return 2
	}

	var result *validate.ValidationResult

	if info.IsDir() {
		result = validateDirectory(target)
	} else {
		result = validateFile(target)
	}

	if result == nil {
		return 2
	}

	fmt.Println(result.String(target))
	if result.HasErrors() {
		return 1
	}
	return 0
}

// validateDirectory validates an environment directory.
func validateDirectory(dirPath string) *validate.ValidationResult {
	envFile := filepath.Join(dirPath, "environment.yaml")
	if _, err := os.Stat(envFile); err != nil {
		fmt.Fprintf(os.Stderr,
			"ot-design validate: directory %q does not contain environment.yaml\n", dirPath,
		)
		return nil
	}
	return validate.ValidateEnvironment(dirPath)
}

// validateFile auto-detects the file type and dispatches to the correct validator.
func validateFile(path string) *validate.ValidationResult {
	doc, err := validate.LoadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ot-design validate: %v\n", err)
		return nil
	}

	switch validate.DetectFileType(doc) {
	case validate.FileTypeDevice:
		return validate.ValidateDevice(path)
	case validate.FileTypeNetwork:
		return validate.ValidateNetwork(path)
	case validate.FileTypeEnvironment:
		return validate.ValidateEnvironment(filepath.Dir(path))
	default:
		fmt.Fprintf(os.Stderr,
			"ot-design validate: unrecognized YAML file %q: expected top-level 'device:', 'network:', or 'environment:' key\n",
			path,
		)
		return nil
	}
}

// runScaffold writes a skeleton YAML to stdout and returns an exit code.
func runScaffold(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "ot-design scaffold: flag required (--device, --network, or --environment)")
		fmt.Fprintln(os.Stderr, "Usage: ot-design scaffold --device|--network|--environment")
		return 2
	}

	switch args[0] {
	case "--device":
		fmt.Print(scaffold.Device())
		return 0
	case "--network":
		fmt.Print(scaffold.Network())
		return 0
	case "--environment":
		fmt.Print(scaffold.Environment())
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Save this output to: design/environments/<your-env-id>/environment.yaml")
		fmt.Fprintln(os.Stderr, "Example: ot-design scaffold --environment > design/environments/my-env/environment.yaml")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "ot-design scaffold: unknown flag %q (expected --device, --network, or --environment)\n", args[0])
		return 2
	}
}

// printUsage writes usage information to stdout.
func printUsage() {
	fmt.Print(`ot-design - Design layer validation and scaffolding for the OT simulator

Usage:
  ot-design validate <path>           Validate a device atom, network atom, or environment
  ot-design scaffold --device         Write a device atom skeleton to stdout
  ot-design scaffold --network        Write a network atom skeleton to stdout
  ot-design scaffold --environment    Write an environment skeleton to stdout
  ot-design --help                    Print this usage information

Validate targets:
  Device atom file:    ot-design validate design/devices/compactlogix-l33er.yaml
  Network atom file:   ot-design validate design/networks/wt-level1.yaml
  Environment dir:     ot-design validate design/environments/greenfield-water-mfg/

The tool auto-detects the file type by inspecting the top-level YAML key.
For environment directories, cross-references to devices and networks are resolved
by walking up the directory tree to find the design root (the directory containing
devices/, networks/, and environments/ subdirectories).

Exit codes:
  0 - Validation passed or scaffold written
  1 - One or more validation errors found
  2 - Usage error (bad arguments, file not found, unparseable YAML)

Scaffold output:
  Redirect scaffold output to a file, then edit the placeholder values:
    ot-design scaffold --device > design/devices/my-new-device.yaml
    ot-design scaffold --network > design/networks/my-new-network.yaml
    ot-design scaffold --environment > design/environments/my-new-env/environment.yaml

Reference: ADR-009 - Design Layer and Composable Environments (schema v0.1)
`)
}
