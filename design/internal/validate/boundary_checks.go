package validate

import (
	"fmt"
)

// checkBoundaries applies ENV-023 through ENV-030 to the boundaries section.
// It is called by ValidateEnvironment after placements are checked.
func checkBoundaries(envFile string, doc *RawDocument, r *ValidationResult) {
	archetype := ""
	envID := ""
	if doc.Environment != nil {
		archetype = doc.Environment.Archetype
		envID = doc.Environment.ID
	}

	envNetworkSet := buildEnvNetworkSet(doc.Networks)

	// ENV-026: hybrid archetype without any boundaries section.
	if archetype == "hybrid" && len(doc.Boundaries) == 0 {
		r.Add(ValidationError{
			File:  envFile,
			Field: "boundaries",
			Message: fmt.Sprintf(
				"environment %q has archetype \"hybrid\" but no boundaries section; "+
					"add a boundaries section to document cross-segment boundary states (ADR-010 D4)",
				envID,
			),
			Severity: SeverityWarning, RuleID: "ENV-026",
		})
	}

	for i, b := range doc.Boundaries {
		checkSingleBoundary(envFile, i, b, envID, archetype, envNetworkSet, r)
	}
}

// checkSingleBoundary applies ENV-023 through ENV-025 and ENV-027 through ENV-030
// to one boundary entry.
func checkSingleBoundary(
	envFile string,
	i int,
	b BoundaryDoc,
	envID string,
	archetype string,
	envNetworkSet map[string]bool,
	r *ValidationResult,
) {
	field := func(f string) string { return fmt.Sprintf("boundaries[%d].%s", i, f) }

	// ENV-023: between must have exactly 2 elements.
	if len(b.Between) != 2 {
		r.Add(ValidationError{
			File:  envFile,
			Field: field("between"),
			Message: fmt.Sprintf(
				"between must list exactly 2 network refs (got %d)", len(b.Between),
			),
			Severity: SeverityError, RuleID: "ENV-023",
		})
	} else {
		// ENV-024: each between element must reference a network in the environment.
		for _, ref := range b.Between {
			if !envNetworkSet[ref] {
				r.Add(ValidationError{
					File:  envFile,
					Field: field("between"),
					Message: fmt.Sprintf(
						"network %q is not listed in the environment's networks section", ref,
					),
					Severity: SeverityError, RuleID: "ENV-024",
				})
			}
		}
	}

	// ENV-025: state must be a valid value.
	checkBoundaryState(envFile, i, b.State, r)

	// ENV-027: modern-segmented with an absent boundary -- warn with reclassification guidance.
	if archetype == "modern-segmented" && b.State == "absent" {
		netA, netB := boundaryNetworkNames(b.Between)
		r.Add(ValidationError{
			File:  envFile,
			Field: field("state"),
			Message: fmt.Sprintf(
				"Environment %q is classified \"modern-segmented\" but has an \"absent\" boundary "+
					"between %q and %q. "+
					"Consider reclassifying as \"hybrid\", or remove the absent boundary entry if it is not intentional.",
				envID, netA, netB,
			),
			Severity: SeverityWarning, RuleID: "ENV-027",
		})
	}

	// ENV-028: boundary installed year range if present.
	if b.Installed != nil {
		checkInstalledYear(envFile, field("installed"), *b.Installed, r, "ENV-028")
	}

	// ENV-029 and ENV-030: infrastructure enum validation if present.
	if b.Infrastructure != "" {
		checkBoundaryInfrastructure(envFile, i, b.Infrastructure, r)
	}
}

// validBoundaryStates lists all valid boundary state values (ADR-010 D4).
var validBoundaryStates = map[string]bool{
	"enforced": true,
	"intended": true,
	"absent":   true,
}

// validBoundaryInfrastructure lists all valid infrastructure enum values.
var validBoundaryInfrastructure = map[string]bool{
	"managed-switch": true,
	"firewall":       true,
	"ids-sensor":     true,
	"vlan-only":      true,
	"other":          true,
}

// checkBoundaryState applies ENV-025: state must be enforced, intended, or absent.
func checkBoundaryState(envFile string, i int, state string, r *ValidationResult) {
	if state == "" || !validBoundaryStates[state] {
		r.Add(ValidationError{
			File:  envFile,
			Field: fmt.Sprintf("boundaries[%d].state", i),
			Message: fmt.Sprintf(
				"value %q is not valid; must be one of: enforced, intended, absent",
				state,
			),
			Severity: SeverityError, RuleID: "ENV-025",
		})
	}
}

// checkBoundaryInfrastructure applies ENV-029 (enum) and ENV-030 (other warning).
func checkBoundaryInfrastructure(envFile string, i int, infra string, r *ValidationResult) {
	if !validBoundaryInfrastructure[infra] {
		r.Add(ValidationError{
			File:  envFile,
			Field: fmt.Sprintf("boundaries[%d].infrastructure", i),
			Message: fmt.Sprintf(
				"value %q is not valid; must be one of: managed-switch, firewall, ids-sensor, vlan-only, other",
				infra,
			),
			Severity: SeverityError, RuleID: "ENV-029",
		})
		return
	}
	// ENV-030: other is valid but warrants specificity guidance.
	if infra == "other" {
		r.Add(ValidationError{
			File:  envFile,
			Field: fmt.Sprintf("boundaries[%d].infrastructure", i),
			Message: "infrastructure value \"other\" is imprecise; specify managed-switch, firewall, ids-sensor, or vlan-only if possible",
			Severity: SeverityWarning, RuleID: "ENV-030",
		})
	}
}

// boundaryNetworkNames extracts the two network names from a between slice.
// Returns placeholder strings if the slice does not have exactly 2 elements.
func boundaryNetworkNames(between []string) (string, string) {
	if len(between) == 2 {
		return between[0], between[1]
	}
	return "<unknown>", "<unknown>"
}
