package validate

import (
	"fmt"
	"path/filepath"
)

// ValidateProcess loads and validates a process schematic YAML file.
// It applies all PRO-001 through PRO-020 rules and returns a ValidationResult.
// The envDoc argument is the companion environment.yaml document for cross-reference
// resolution (placement IDs). Pass nil to skip cross-reference checks.
func ValidateProcess(processFile string, envDoc *RawDocument, designRoot string) *ValidationResult {
	result := &ValidationResult{}

	doc, err := LoadFile(processFile)
	if err != nil {
		result.Add(ValidationError{
			File: processFile, Message: err.Error(),
			Severity: SeverityError, RuleID: "PRO-000",
		})
		return result
	}

	// PRO-001 through PRO-005: header and required field validation.
	if !checkProcessHeader(processFile, doc, result) {
		return result
	}

	placementSet := buildPlacementSet(envDoc)
	deviceDocs := loadProcessDeviceDocs(processFile, envDoc, designRoot, result)

	equipmentIDs := make(map[string]bool)
	instrumentTags := make(map[string]bool)
	checkStages(processFile, doc.Stages, placementSet, deviceDocs, envDoc,
		equipmentIDs, instrumentTags, result)
	checkConnections(processFile, doc.Connections, equipmentIDs, result)
	checkNetworkContext(processFile, doc.NetworkContext, placementSet, result)

	return result
}

// checkProcessHeader applies PRO-001 through PRO-005.
// Returns false if validation should stop early (missing required ID or stages).
func checkProcessHeader(processFile string, doc *RawDocument, r *ValidationResult) bool {
	// PRO-001: schema_version.
	if doc.SchemaVersion != "0.1" {
		r.Add(ValidationError{
			File:  processFile,
			Field: "schema_version",
			Message: fmt.Sprintf(
				"missing or unsupported (got %q, expected \"0.1\")", doc.SchemaVersion,
			),
			Severity: SeverityError, RuleID: "PRO-001",
		})
	}

	// PRO-002: process.id required.
	if doc.Process == nil || doc.Process.ID == "" {
		r.Add(ValidationError{
			File:     processFile,
			Field:    "process.id",
			Message:  "required field is missing or empty",
			Severity: SeverityError, RuleID: "PRO-002",
		})
		return false
	}

	// PRO-003: process.name required.
	if doc.Process.Name == "" {
		r.Add(ValidationError{
			File:     processFile,
			Field:    "process.name",
			Message:  "required field is missing or empty",
			Severity: SeverityError, RuleID: "PRO-003",
		})
	}

	// PRO-004: flow_direction enum.
	checkFlowDirection(processFile, doc.Process.FlowDirection, r)

	// PRO-005: at least one stage required.
	if len(doc.Stages) == 0 {
		r.Add(ValidationError{
			File:     processFile,
			Field:    "stages",
			Message:  "at least one stage is required",
			Severity: SeverityError, RuleID: "PRO-005",
		})
		return false
	}

	return true
}

// buildPlacementSet builds a set of placement IDs from the companion environment.
func buildPlacementSet(envDoc *RawDocument) map[string]bool {
	set := make(map[string]bool)
	if envDoc == nil {
		return set
	}
	for _, p := range envDoc.Placements {
		if p.ID != "" {
			set[p.ID] = true
		}
	}
	return set
}

// loadProcessDeviceDocs loads device atom documents for each placement in the environment.
// Returns a map of placement ID -> device RawDocument (nil if load failed).
func loadProcessDeviceDocs(
	processFile string,
	envDoc *RawDocument,
	designRoot string,
	r *ValidationResult,
) map[string]*RawDocument {
	docs := make(map[string]*RawDocument)
	if envDoc == nil || designRoot == "" {
		return docs
	}
	placementDevice := make(map[string]string)
	for _, p := range envDoc.Placements {
		if p.ID != "" && p.Device != "" {
			placementDevice[p.ID] = p.Device
		}
	}
	loadedDevices := make(map[string]*RawDocument)
	for placementID, deviceID := range placementDevice {
		if loaded, ok := loadedDevices[deviceID]; ok {
			docs[placementID] = loaded
			continue
		}
		devPath := filepath.Join(designRoot, "devices", deviceID+".yaml")
		devDoc, err := LoadFile(devPath)
		if err != nil {
			// Device load failures are caught by environment validation; do not double-report.
			loadedDevices[deviceID] = nil
			docs[placementID] = nil
			continue
		}
		loadedDevices[deviceID] = devDoc
		docs[placementID] = devDoc
	}
	return docs
}

// checkStages applies PRO-006 through PRO-013 across all stages.
func checkStages(
	processFile string,
	stages []StageDoc,
	placementSet map[string]bool,
	deviceDocs map[string]*RawDocument,
	envDoc *RawDocument,
	equipmentIDs map[string]bool,
	instrumentTags map[string]bool,
	r *ValidationResult,
) {
	stageIDs := make(map[string]bool)
	for i, stage := range stages {
		// PRO-006: unique stage IDs.
		if stage.ID == "" {
			r.Add(ValidationError{
				File:     processFile,
				Field:    fmt.Sprintf("stages[%d].id", i),
				Message:  "required field is missing or empty",
				Severity: SeverityError, RuleID: "PRO-006",
			})
		} else if stageIDs[stage.ID] {
			r.Add(ValidationError{
				File:    processFile,
				Field:   fmt.Sprintf("stages[%d].id", i),
				Message: fmt.Sprintf("duplicate stage ID %q", stage.ID),
				Severity: SeverityError, RuleID: "PRO-006",
			})
		} else {
			stageIDs[stage.ID] = true
		}
		checkStageEquipment(processFile, i, stage.Equipment,
			placementSet, deviceDocs, envDoc, equipmentIDs, instrumentTags, r)
	}
}

// checkStageEquipment applies PRO-007 through PRO-013 for equipment in one stage.
func checkStageEquipment(
	processFile string,
	stageIdx int,
	equipment []EquipmentDoc,
	placementSet map[string]bool,
	deviceDocs map[string]*RawDocument,
	envDoc *RawDocument,
	equipmentIDs map[string]bool,
	instrumentTags map[string]bool,
	r *ValidationResult,
) {
	for j, eq := range equipment {
		eqField := fmt.Sprintf("stages[%d].equipment[%d]", stageIdx, j)

		// PRO-007: equipment IDs unique across entire process.
		if eq.ID == "" {
			r.Add(ValidationError{
				File:     processFile,
				Field:    eqField + ".id",
				Message:  "required field is missing or empty",
				Severity: SeverityError, RuleID: "PRO-007",
			})
		} else if equipmentIDs[eq.ID] {
			r.Add(ValidationError{
				File:    processFile,
				Field:   eqField + ".id",
				Message: fmt.Sprintf("duplicate equipment ID %q", eq.ID),
				Severity: SeverityError, RuleID: "PRO-007",
			})
		} else {
			equipmentIDs[eq.ID] = true
		}

		checkEquipmentType(processFile, eqField+".type", eq.Type, r) // PRO-008

		if eq.Era != nil {
			checkEraYear(processFile, eqField+".era", *eq.Era, r) // PRO-018
		}

		checkInstruments(processFile, stageIdx, j, eq.Instruments,
			placementSet, deviceDocs, envDoc, instrumentTags, r)
	}
}

// checkInstruments applies PRO-009 through PRO-013, PRO-019, and PRO-020.
func checkInstruments(
	processFile string,
	stageIdx, eqIdx int,
	instruments []InstrumentDoc,
	placementSet map[string]bool,
	deviceDocs map[string]*RawDocument,
	envDoc *RawDocument,
	instrumentTags map[string]bool,
	r *ValidationResult,
) {
	for k, inst := range instruments {
		iField := fmt.Sprintf("stages[%d].equipment[%d].instruments[%d]", stageIdx, eqIdx, k)

		// PRO-009: instrument tag uniqueness.
		if inst.Tag == "" {
			r.Add(ValidationError{
				File:     processFile,
				Field:    iField + ".tag",
				Message:  "required field is missing or empty",
				Severity: SeverityError, RuleID: "PRO-009",
			})
		} else if instrumentTags[inst.Tag] {
			r.Add(ValidationError{
				File:    processFile,
				Field:   iField + ".tag",
				Message: fmt.Sprintf("duplicate instrument tag %q", inst.Tag),
				Severity: SeverityError, RuleID: "PRO-009",
			})
		} else {
			instrumentTags[inst.Tag] = true
		}

		checkISAType(processFile, iField+".isa_type", inst.ISAType, r) // PRO-010
		checkPlacementRef(processFile, iField, inst.Placement, placementSet, r) // PRO-011
		checkRegisterType(processFile, iField+".register.type", inst.Register.Type, r) // PRO-012

		if inst.Placement != "" && len(placementSet) > 0 {
			checkRegisterBounds(processFile, iField, inst, deviceDocs, r) // PRO-013
		}

		// PRO-019: tag prefix vs isa_type consistency.
		if inst.Tag != "" && inst.ISAType != "" {
			checkTagPrefixConsistency(processFile, iField, inst.Tag, inst.ISAType, r)
		}

		// PRO-020: scale vs device atom scale_min/scale_max.
		if inst.Placement != "" && len(inst.Scale) == 2 {
			checkScaleConsistency(processFile, iField, inst, deviceDocs, envDoc, r)
		}
	}
}

// checkPlacementRef applies PRO-011: placement must reference a valid placement ID.
func checkPlacementRef(
	processFile, iField, placement string,
	placementSet map[string]bool,
	r *ValidationResult,
) {
	if placement == "" {
		r.Add(ValidationError{
			File:     processFile,
			Field:    iField + ".placement",
			Message:  "required field is missing or empty",
			Severity: SeverityError, RuleID: "PRO-011",
		})
		return
	}
	if len(placementSet) > 0 && !placementSet[placement] {
		r.Add(ValidationError{
			File:  processFile,
			Field: iField + ".placement",
			Message: fmt.Sprintf(
				"placement %q not found in companion environment.yaml", placement,
			),
			Severity: SeverityError, RuleID: "PRO-011",
		})
	}
}

// checkConnections applies PRO-014 and PRO-015 for all connections.
func checkConnections(
	processFile string,
	connections []ConnectionDoc,
	equipmentIDs map[string]bool,
	r *ValidationResult,
) {
	for i, conn := range connections {
		connField := fmt.Sprintf("connections[%d]", i)
		checkConnectionEndpoint(processFile, connField+".from", conn.From, equipmentIDs, r)
		checkConnectionEndpoint(processFile, connField+".to", conn.To, equipmentIDs, r)
		checkConnectionType(processFile, connField+".type", conn.Type, r) // PRO-015
	}
}

// checkConnectionEndpoint applies PRO-014 for one connection endpoint field.
func checkConnectionEndpoint(
	processFile, field, equipID string,
	equipmentIDs map[string]bool,
	r *ValidationResult,
) {
	if equipID == "" {
		r.Add(ValidationError{
			File:     processFile,
			Field:    field,
			Message:  "required field is missing or empty",
			Severity: SeverityError, RuleID: "PRO-014",
		})
		return
	}
	if !equipmentIDs[equipID] {
		r.Add(ValidationError{
			File:  processFile,
			Field: field,
			Message: fmt.Sprintf(
				"equipment ID %q not found in this process schematic", equipID,
			),
			Severity: SeverityError, RuleID: "PRO-014",
		})
	}
}

// checkNetworkContext applies PRO-016 and PRO-017 for all network_context entries.
func checkNetworkContext(
	processFile string,
	contexts []NetworkContextDoc,
	placementSet map[string]bool,
	r *ValidationResult,
) {
	for i, nc := range contexts {
		ncField := fmt.Sprintf("network_context[%d]", i)

		// PRO-016: placement reference (warning if present but not in environment).
		if nc.Placement != "" && len(placementSet) > 0 && !placementSet[nc.Placement] {
			r.Add(ValidationError{
				File:  processFile,
				Field: ncField + ".placement",
				Message: fmt.Sprintf(
					"placement %q not found in companion environment.yaml", nc.Placement,
				),
				Severity: SeverityWarning, RuleID: "PRO-016",
			})
		}

		// PRO-017: type enum.
		if !validNetworkContextTypes[nc.Type] {
			r.Add(ValidationError{
				File:  processFile,
				Field: ncField + ".type",
				Message: fmt.Sprintf(
					"value %q is not valid; must be one of: wan_link, internet_gateway, wireless_bridge",
					nc.Type,
				),
				Severity: SeverityError, RuleID: "PRO-017",
			})
		}
	}
}
