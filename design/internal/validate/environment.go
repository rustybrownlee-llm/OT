package validate

import (
	"fmt"
	"path/filepath"
)

// ValidateEnvironment loads and validates an environment directory.
// It applies all ENV-001 through ENV-019 rules and returns a ValidationResult.
// The dirPath argument is the path to the environment directory containing environment.yaml.
func ValidateEnvironment(dirPath string) *ValidationResult {
	result := &ValidationResult{}

	envFile := filepath.Join(dirPath, "environment.yaml")
	doc, err := LoadFile(envFile)
	if err != nil {
		result.Add(ValidationError{
			File: envFile, Message: err.Error(),
			Severity: SeverityError, RuleID: "ENV-000",
		})
		return result
	}

	// ENV-001: schema_version.
	if doc.SchemaVersion != "0.1" {
		result.Add(ValidationError{
			File:  envFile,
			Field: "schema_version",
			Message: fmt.Sprintf(
				"missing or unsupported (got %q, expected \"0.1\")", doc.SchemaVersion,
			),
			Severity: SeverityError, RuleID: "ENV-001",
		})
	}

	// ENV-002: environment.id required.
	if doc.Environment == nil || doc.Environment.ID == "" {
		result.Add(ValidationError{
			File: envFile, Field: "environment.id",
			Message:  "required field is missing or empty",
			Severity: SeverityError, RuleID: "ENV-002",
		})
		return result
	}

	// Find design root for cross-reference resolution.
	designRoot, err := FindDesignRoot(dirPath)
	if err != nil {
		result.Add(ValidationError{
			File:     envFile,
			Message:  fmt.Sprintf("cannot find design root: %v", err),
			Severity: SeverityError, RuleID: "ENV-000",
		})
		return result
	}

	// Load all referenced networks (ENV-003).
	networkDocs := loadNetworkRefs(envFile, designRoot, doc.Networks, result)

	// Load all referenced device docs (ENV-004).
	deviceDocs := loadDeviceRefs(envFile, designRoot, doc.Placements, result)

	// Check placements.
	checkPlacements(envFile, doc, networkDocs, deviceDocs, result)
	return result
}

// loadNetworkRefs loads and validates all network references in the environment.
// Returns a map of network ID -> loaded RawDocument (nil if failed to load).
func loadNetworkRefs(
	envFile, designRoot string,
	refs []NetworkRefDoc,
	r *ValidationResult,
) map[string]*RawDocument {
	docs := make(map[string]*RawDocument)
	for i, ref := range refs {
		if ref.Ref == "" {
			r.Add(ValidationError{
				File:     envFile,
				Field:    fmt.Sprintf("networks[%d].ref", i),
				Message:  "ref: required field is missing or empty",
				Severity: SeverityError, RuleID: "ENV-003",
			})
			continue
		}
		netPath := filepath.Join(designRoot, "networks", ref.Ref+".yaml")
		netDoc, err := LoadFile(netPath)
		if err != nil {
			r.Add(ValidationError{
				File:  envFile,
				Field: fmt.Sprintf("networks[%d].ref", i),
				Message: fmt.Sprintf(
					"network %q not found (expected %s)", ref.Ref, netPath,
				),
				Severity: SeverityError, RuleID: "ENV-003",
			})
			docs[ref.Ref] = nil
			continue
		}
		netResult := ValidateNetwork(netPath)
		for _, e := range netResult.Errors {
			r.Add(e)
		}
		docs[ref.Ref] = netDoc
	}
	return docs
}

// loadDeviceRefs loads and validates all device references in the placements.
// Returns a map of device ID -> loaded RawDocument (nil if failed to load).
func loadDeviceRefs(
	envFile, designRoot string,
	placements []PlacementDoc,
	r *ValidationResult,
) map[string]*RawDocument {
	docs := make(map[string]*RawDocument)
	seen := make(map[string]bool)
	for i, p := range placements {
		if p.Device == "" || seen[p.Device] {
			continue
		}
		seen[p.Device] = true
		devPath := filepath.Join(designRoot, "devices", p.Device+".yaml")
		devDoc, err := LoadFile(devPath)
		if err != nil {
			r.Add(ValidationError{
				File:  envFile,
				Field: fmt.Sprintf("placements[%d].device", i),
				Message: fmt.Sprintf(
					"device %q not found (expected %s)", p.Device, devPath,
				),
				Severity: SeverityError, RuleID: "ENV-004",
			})
			docs[p.Device] = nil
			continue
		}
		devResult := ValidateDevice(devPath)
		for _, e := range devResult.Errors {
			r.Add(e)
		}
		docs[p.Device] = devDoc
	}
	return docs
}

// checkPlacements runs all ENV-005 through ENV-019 placement checks.
func checkPlacements(
	envFile string,
	doc *RawDocument,
	networkDocs map[string]*RawDocument,
	deviceDocs map[string]*RawDocument,
	r *ValidationResult,
) {
	placements := doc.Placements
	envNetworkSet := buildEnvNetworkSet(doc.Networks)
	placementIDs := make(map[string]int)         // id -> first-seen index
	networkIPs := make(map[string]map[string]string) // netID -> ip -> placementID
	usedPorts := make(map[int]string)            // port -> placementID
	gwSerialAddrs := make(map[string]map[int]string) // gatewayID -> addr -> placementID

	for i, p := range placements {
		checkSinglePlacement(
			envFile, i, p,
			networkDocs, deviceDocs,
			envNetworkSet, placementIDs,
			networkIPs, usedPorts, gwSerialAddrs,
			r,
		)
	}

	// ENV-011, ENV-012: gateway back-references (second pass after all IDs are known).
	for i, p := range placements {
		if p.Gateway == "" {
			continue
		}
		if _, ok := placementIDs[p.Gateway]; !ok {
			r.Add(ValidationError{
				File:  envFile,
				Field: fmt.Sprintf("placements[%d].gateway", i),
				Message: fmt.Sprintf(
					"gateway placement %q not found in environment", p.Gateway,
				),
				Severity: SeverityError, RuleID: "ENV-011",
			})
			continue
		}
		checkGatewayBridge(envFile, i, p, placements, placementIDs, r)
	}
}

func buildEnvNetworkSet(refs []NetworkRefDoc) map[string]bool {
	set := make(map[string]bool, len(refs))
	for _, ref := range refs {
		set[ref.Ref] = true
	}
	return set
}
