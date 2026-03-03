package validate

import (
	"fmt"
	"strings"
	"time"
)

// validEquipmentTypes lists valid equipment type values (OT domain corrected per RD-1).
// Note: "gateway" is NOT a valid equipment type -- gateways belong in network_context.
var validEquipmentTypes = map[string]bool{
	"tank":             true,
	"basin":            true,
	"pump":             true,
	"valve":            true,
	"blower":           true,
	"analyzer_station": true,
	"chromatograph":    true,
	"uv_system":        true,
	"clarifier":        true,
	"screen":           true,
	"compressor":       true,
	"meter":            true,
	"heat_exchanger":   true,
}

// isaInstrumentPrefixes lists ISA-5.1 standard instrument prefixes.
var isaInstrumentPrefixes = map[string]bool{
	"LT": true, "FT": true, "PT": true, "TT": true, "AT": true,
	"AIT": true, "PDT": true, "ST": true, "VT": true, "BT": true,
	"IT": true, "HS": true, "ZS": true, "ZSO": true, "ZSC": true, "FE": true,
	// Water treatment additions (SOW-023.0):
	"SC":  true, // Speed Controller -- writable VFD speed setpoint (ISA-5.1 suffix "C" = controller output)
	"FIC": true, // Flow Indicating Controller -- writable flow setpoint (ISA-5.1 suffix "IC" = indicating controller)
	"RT":  true, // Radiation Transmitter -- UV lamp intensity from photodiode (not analytical; ISA-5.1 suffix "T")
	"LS":  true, // Level Switch -- discrete float switch or level alarm contact (ISA-5.1 suffix "S")
	// Pipeline and wastewater additions (SOW-024.0):
	"ZT":  true, // Position Transmitter -- analog valve position feedback 0-100% (ISA-5.1 Z=position, T=transmitter)
	"FQ":  true, // Flow Quantity / Totalizer -- accumulated volume (ISA-5.1 F=flow, Q=quantity/integrate); custody transfer and NPDES daily reporting
	"AIC": true, // Analyzer Indicating Controller -- writable analytical setpoint (ISA-5.1 A=analysis, IC=indicating controller); e.g. dissolved oxygen setpoint
	"KIC": true, // Time Indicating Controller -- writable timer setpoint (ISA-5.1 K=time, IC=indicating controller); e.g. sludge pump cycle time
}

// isaEquipmentDesignators lists ISA-5.1 equipment designators.
var isaEquipmentDesignators = map[string]bool{
	"P": true, "V": true, "B": true, "C": true, "SV": true, "GC": true, "FM": true,
}

// isaProjectExtensions lists project-defined extensions (not ISA-5.1 standard).
var isaProjectExtensions = map[string]bool{
	"run": true, // Binary coil run-status (project extension, not standard ISA-5.1)
}

// validNetworkContextTypes lists valid network_context type values (RD-4).
var validNetworkContextTypes = map[string]bool{
	"wan_link":         true,
	"internet_gateway": true,
	"wireless_bridge":  true,
}

// checkFlowDirection applies PRO-004: flow_direction must be valid or empty.
func checkFlowDirection(processFile, dir string, r *ValidationResult) {
	if dir == "" || dir == "horizontal" || dir == "vertical" {
		return
	}
	r.Add(ValidationError{
		File:  processFile,
		Field: "process.flow_direction",
		Message: fmt.Sprintf(
			"value %q is not valid; must be \"horizontal\", \"vertical\", or omitted (defaults to \"horizontal\")",
			dir,
		),
		Severity: SeverityError, RuleID: "PRO-004",
	})
}

// checkEquipmentType applies PRO-008: equipment type must be a valid enum value.
func checkEquipmentType(processFile, field, equipType string, r *ValidationResult) {
	if equipType == "" {
		r.Add(ValidationError{
			File:     processFile,
			Field:    field,
			Message:  "required field is missing or empty",
			Severity: SeverityError, RuleID: "PRO-008",
		})
		return
	}
	if !validEquipmentTypes[equipType] {
		r.Add(ValidationError{
			File:  processFile,
			Field: field,
			Message: fmt.Sprintf(
				"value %q is not a valid equipment type; valid types: tank, basin, pump, valve, "+
					"blower, analyzer_station, chromatograph, uv_system, clarifier, screen, "+
					"compressor, meter, heat_exchanger (note: gateways belong in network_context per RD-1)",
				equipType,
			),
			Severity: SeverityError, RuleID: "PRO-008",
		})
	}
}

// checkEraYear applies PRO-018: era must be >= 1970 and <= current year (warning).
func checkEraYear(processFile, field string, era int, r *ValidationResult) {
	maxYear := time.Now().Year()
	if era < 1970 || era > maxYear {
		r.Add(ValidationError{
			File:  processFile,
			Field: field,
			Message: fmt.Sprintf(
				"era year %d is outside valid range 1970-%d", era, maxYear,
			),
			Severity: SeverityWarning, RuleID: "PRO-018",
		})
	}
}

// checkISAType applies PRO-010: isa_type should be a recognized prefix (warning if not).
func checkISAType(processFile, field, isaType string, r *ValidationResult) {
	if isaType == "" {
		return
	}
	if isaInstrumentPrefixes[isaType] || isaEquipmentDesignators[isaType] || isaProjectExtensions[isaType] {
		return
	}
	r.Add(ValidationError{
		File:  processFile,
		Field: field,
		Message: fmt.Sprintf(
			"isa_type %q is not a recognized prefix; "+
				"ISA-5.1 instrument prefixes: AIC, AIT, AT, BT, FE, FIC, FQ, FT, HS, IT, KIC, LS, LT, PDT, PT, RT, SC, ST, TT, VT, ZS, ZSC, ZSO, ZT; "+
				"equipment designators: P, V, B, C, SV, GC, FM; "+
				"project extension: run (not ISA-5.1 standard)",
			isaType,
		),
		Severity: SeverityWarning, RuleID: "PRO-010",
	})
}

// checkRegisterType applies PRO-012: register type must be "holding" or "coil".
func checkRegisterType(processFile, field, regType string, r *ValidationResult) {
	if regType == "holding" || regType == "coil" {
		return
	}
	r.Add(ValidationError{
		File:  processFile,
		Field: field,
		Message: fmt.Sprintf(
			"value %q is not valid; must be \"holding\" or \"coil\"", regType,
		),
		Severity: SeverityError, RuleID: "PRO-012",
	})
}

// checkRegisterBounds applies PRO-013: address must be within device capacity.
func checkRegisterBounds(
	processFile, iField string,
	inst InstrumentDoc,
	deviceDocs map[string]*RawDocument,
	r *ValidationResult,
) {
	devDoc := deviceDocs[inst.Placement]
	if devDoc == nil || devDoc.Registers == nil {
		return
	}
	addr := inst.Register.Address
	if addr < 0 {
		r.Add(ValidationError{
			File:     processFile,
			Field:    iField + ".register.address",
			Message:  fmt.Sprintf("address %d must be >= 0", addr),
			Severity: SeverityError, RuleID: "PRO-013",
		})
		return
	}
	switch inst.Register.Type {
	case "holding":
		if devDoc.Registers.MaxHolding != nil && addr >= *devDoc.Registers.MaxHolding {
			r.Add(ValidationError{
				File:  processFile,
				Field: iField + ".register.address",
				Message: fmt.Sprintf(
					"holding register address %d exceeds device capacity %d for placement %q",
					addr, *devDoc.Registers.MaxHolding, inst.Placement,
				),
				Severity: SeverityError, RuleID: "PRO-013",
			})
		}
	case "coil":
		if devDoc.Registers.MaxCoils != nil && addr >= *devDoc.Registers.MaxCoils {
			r.Add(ValidationError{
				File:  processFile,
				Field: iField + ".register.address",
				Message: fmt.Sprintf(
					"coil address %d exceeds device capacity %d for placement %q",
					addr, *devDoc.Registers.MaxCoils, inst.Placement,
				),
				Severity: SeverityError, RuleID: "PRO-013",
			})
		}
	}
}

// checkTagPrefixConsistency applies PRO-019: tag prefix and isa_type should match.
func checkTagPrefixConsistency(processFile, iField, tag, isaType string, r *ValidationResult) {
	prefix := extractTagPrefix(tag)
	if prefix == "" {
		return
	}
	// Only warn when the prefix is a recognized ISA prefix (not project extensions).
	if !isaInstrumentPrefixes[prefix] && !isaEquipmentDesignators[prefix] {
		return
	}
	if !strings.EqualFold(prefix, isaType) {
		r.Add(ValidationError{
			File:  processFile,
			Field: iField + ".tag",
			Message: fmt.Sprintf(
				"tag prefix %q does not match isa_type %q; P&ID convention requires tag and type to align "+
					"(e.g., tag \"FT-101\" should have isa_type \"FT\")",
				prefix, isaType,
			),
			Severity: SeverityWarning, RuleID: "PRO-019",
		})
	}
}

// extractTagPrefix extracts the alphabetic prefix from an ISA tag (e.g., "FT" from "FT-101").
func extractTagPrefix(tag string) string {
	for i, ch := range tag {
		if ch == '-' || (ch >= '0' && ch <= '9') {
			return tag[:i]
		}
	}
	return tag
}

// checkScaleConsistency applies PRO-020: process scale vs device atom scale_min/scale_max.
func checkScaleConsistency(
	processFile, iField string,
	inst InstrumentDoc,
	deviceDocs map[string]*RawDocument,
	envDoc *RawDocument,
	r *ValidationResult,
) {
	devDoc := deviceDocs[inst.Placement]
	if devDoc == nil {
		return
	}
	if inst.Register.Type != "holding" || devDoc.Variants == nil {
		return
	}
	variant := findVariantForPlacement(inst.Placement, envDoc, devDoc)
	if variant == nil {
		return
	}
	for _, hr := range variant.Holding {
		if hr.Address == nil || *hr.Address != inst.Register.Address {
			continue
		}
		if hr.ScaleMin == nil || hr.ScaleMax == nil {
			return
		}
		procMin := inst.Scale[0]
		procMax := inst.Scale[1]
		if procMin != *hr.ScaleMin || procMax != *hr.ScaleMax {
			r.Add(ValidationError{
				File:  processFile,
				Field: iField + ".scale",
				Message: fmt.Sprintf(
					"process scale [%.4g, %.4g] differs from device atom scale [%.4g, %.4g] "+
						"for register address %d on placement %q; "+
						"mismatched scale produces wrong engineering unit conversions",
					procMin, procMax, *hr.ScaleMin, *hr.ScaleMax,
					inst.Register.Address, inst.Placement,
				),
				Severity: SeverityWarning, RuleID: "PRO-020",
			})
		}
		return
	}
}

// findVariantForPlacement returns the VariantDoc used by the given placement.
// Returns nil if the placement or variant cannot be resolved.
func findVariantForPlacement(
	placementID string,
	envDoc *RawDocument,
	devDoc *RawDocument,
) *VariantDoc {
	if envDoc == nil || devDoc == nil || devDoc.Variants == nil {
		return nil
	}
	for _, p := range envDoc.Placements {
		if p.ID != placementID || p.RegisterMapVariant == "" {
			continue
		}
		v, ok := devDoc.Variants[p.RegisterMapVariant]
		if !ok {
			return nil
		}
		return &v
	}
	return nil
}

// checkConnectionType applies PRO-015: connection type must be "pipe" or "duct".
func checkConnectionType(processFile, field, connType string, r *ValidationResult) {
	if connType == "pipe" || connType == "duct" {
		return
	}
	r.Add(ValidationError{
		File:  processFile,
		Field: field,
		Message: fmt.Sprintf(
			"value %q is not valid; must be \"pipe\" or \"duct\" "+
				"(serial bus and wireless links belong in network_context per RD-1)",
			connType,
		),
		Severity: SeverityError, RuleID: "PRO-015",
	})
}
