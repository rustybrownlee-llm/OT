package validate

import (
	"fmt"
	"strings"
)

// validDeviceCategories lists all accepted device.category values.
var validDeviceCategories = map[string]bool{
	"plc": true, "gateway": true, "hmi": true,
	"sensor": true, "relay": true, "safety-controller": true,
}

// validFloatByteOrders lists all accepted registers.float_byte_order values.
var validFloatByteOrders = map[string]bool{
	"big-endian": true, "little-endian": true,
	"big-endian-byte-swap": true, "little-endian-byte-swap": true,
}

// measurementSuffixes are substrings that suggest a sensor measurement register.
var measurementSuffixes = []string{
	"_flow", "_pressure", "_temp", "_level", "_ph",
	"_turbidity", "_current", "_alarm", "_status",
}

// ValidateDevice loads and validates a device atom YAML file.
// It applies all DEV-001 through DEV-019 rules and returns a ValidationResult.
func ValidateDevice(path string) *ValidationResult {
	result := &ValidationResult{}

	doc, err := LoadFile(path)
	if err != nil {
		result.Add(ValidationError{
			File: path, Message: err.Error(),
			Severity: SeverityError, RuleID: "DEV-000",
		})
		return result
	}

	checkDeviceSchemaVersion(path, doc, result)
	if doc.Device == nil {
		result.Add(ValidationError{
			File:     path,
			Field:    "device",
			Message:  "device: required top-level section is missing",
			Severity: SeverityError,
			RuleID:   "DEV-002",
		})
		return result
	}

	checkDeviceRequiredFields(path, doc.Device, result)
	checkConnectivity(path, doc, result)
	checkRegisterCapabilities(path, doc.Registers, result)
	checkVariants(path, doc, result)
	return result
}

func checkDeviceSchemaVersion(path string, doc *RawDocument, r *ValidationResult) {
	if doc.SchemaVersion != "0.1" {
		r.Add(ValidationError{
			File:  path,
			Field: "schema_version",
			Message: fmt.Sprintf(
				"missing or unsupported (got %q, expected \"0.1\")", doc.SchemaVersion,
			),
			Severity: SeverityError,
			RuleID:   "DEV-001",
		})
	}
}

func checkDeviceRequiredFields(path string, d *DeviceDoc, r *ValidationResult) {
	if d.ID == "" {
		r.Add(ValidationError{
			File: path, Field: "device.id",
			Message:  "required field is missing or empty",
			Severity: SeverityError, RuleID: "DEV-002",
		})
	}
	if d.Vendor == "" {
		r.Add(ValidationError{
			File: path, Field: "device.vendor",
			Message:  "required field is missing or empty",
			Severity: SeverityError, RuleID: "DEV-003",
		})
	}
	if d.Model == "" {
		r.Add(ValidationError{
			File: path, Field: "device.model",
			Message:  "required field is missing or empty",
			Severity: SeverityError, RuleID: "DEV-004",
		})
	}
	if !validDeviceCategories[d.Category] {
		r.Add(ValidationError{
			File: path, Field: "device.category",
			Message: fmt.Sprintf(
				"invalid value %q (must be one of: plc, gateway, hmi, sensor, relay, safety-controller)",
				d.Category,
			),
			Severity: SeverityError, RuleID: "DEV-005",
		})
	}
}

func checkConnectivity(path string, doc *RawDocument, r *ValidationResult) {
	if doc.Connectivity == nil {
		r.Add(ValidationError{
			File: path, Field: "connectivity.ports",
			Message:  "at least one port with a protocol is required",
			Severity: SeverityError, RuleID: "DEV-006",
		})
		return
	}

	hasProtocol := false
	for _, p := range doc.Connectivity.Ports {
		if len(p.Protocols) > 0 {
			hasProtocol = true
			break
		}
	}
	if !hasProtocol {
		r.Add(ValidationError{
			File: path, Field: "connectivity.ports",
			Message:  "at least one port with a protocol is required",
			Severity: SeverityError, RuleID: "DEV-006",
		})
	}

	hasEthernet := connectivityHasEthernet(doc.Connectivity)
	checkResponseDelayWarning(path, doc.Connectivity, hasEthernet, r)
}

// connectivityHasEthernet returns true if any port has type "ethernet".
func connectivityHasEthernet(c *ConnectivityDoc) bool {
	for _, p := range c.Ports {
		if p.Type == "ethernet" {
			return true
		}
	}
	return false
}

func checkResponseDelayWarning(path string, c *ConnectivityDoc, hasEthernet bool, r *ValidationResult) {
	if c.ResponseDelayMs == nil {
		return
	}
	v := *c.ResponseDelayMs
	if hasEthernet {
		if v < 2 || v > 2000 {
			r.Add(ValidationError{
				File:  path,
				Field: "connectivity.response_delay_ms",
				Message: fmt.Sprintf(
					"value %d is outside expected range 2-2000ms for ethernet devices", v,
				),
				Severity: SeverityWarning, RuleID: "DEV-017",
			})
		}
	} else {
		if v < 10 || v > 2000 {
			r.Add(ValidationError{
				File:  path,
				Field: "connectivity.response_delay_ms",
				Message: fmt.Sprintf(
					"value %d is outside expected range 10-2000ms for serial devices", v,
				),
				Severity: SeverityWarning, RuleID: "DEV-017",
			})
		}
	}
}

func checkRegisterCapabilities(path string, reg *RegistersDoc, r *ValidationResult) {
	if reg == nil {
		r.Add(ValidationError{
			File: path, Field: "registers.addressing",
			Message:  "invalid value \"\" (must be \"zero-based\" or \"one-based\")",
			Severity: SeverityError, RuleID: "DEV-007",
		})
		r.Add(ValidationError{
			File: path, Field: "registers.max_holding",
			Message:  "required field, must be >= 0",
			Severity: SeverityError, RuleID: "DEV-008",
		})
		r.Add(ValidationError{
			File: path, Field: "registers.max_coils",
			Message:  "required field, must be >= 0",
			Severity: SeverityError, RuleID: "DEV-009",
		})
		return
	}

	if reg.Addressing != "zero-based" && reg.Addressing != "one-based" {
		r.Add(ValidationError{
			File: path, Field: "registers.addressing",
			Message: fmt.Sprintf(
				"invalid value %q (must be \"zero-based\" or \"one-based\")", reg.Addressing,
			),
			Severity: SeverityError, RuleID: "DEV-007",
		})
	}
	if reg.MaxHolding == nil || *reg.MaxHolding < 0 {
		r.Add(ValidationError{
			File: path, Field: "registers.max_holding",
			Message:  "required field, must be >= 0",
			Severity: SeverityError, RuleID: "DEV-008",
		})
	}
	if reg.MaxCoils == nil || *reg.MaxCoils < 0 {
		r.Add(ValidationError{
			File: path, Field: "registers.max_coils",
			Message:  "required field, must be >= 0",
			Severity: SeverityError, RuleID: "DEV-009",
		})
	}

	if !validFloatByteOrders[reg.FloatByteOrder] {
		r.Add(ValidationError{
			File: path, Field: "registers.float_byte_order",
			Message: fmt.Sprintf(
				"invalid value %q (must be one of: big-endian, little-endian, big-endian-byte-swap, little-endian-byte-swap)",
				reg.FloatByteOrder,
			),
			Severity: SeverityError, RuleID: "DEV-018",
		})
	}
}

func checkVariants(path string, doc *RawDocument, r *ValidationResult) {
	if doc.Registers == nil {
		return
	}
	reg := doc.Registers
	vendor := ""
	if doc.Device != nil {
		vendor = doc.Device.Vendor
	}

	checkFloatByteOrderVendorWarning(path, reg, vendor, r)

	for variantName, variant := range doc.Variants {
		checkVariantHolding(path, variantName, variant.Holding, reg, r)
		checkVariantCoils(path, variantName, variant.Coils, reg, r)
	}
}

func checkFloatByteOrderVendorWarning(path string, reg *RegistersDoc, vendor string, r *ValidationResult) {
	if !validFloatByteOrders[reg.FloatByteOrder] {
		return // Already reported as error in DEV-018.
	}
	vendorLower := strings.ToLower(vendor)
	if (strings.Contains(vendorLower, "schneider") || strings.Contains(vendorLower, "modicon")) &&
		reg.FloatByteOrder == "big-endian" {
		r.Add(ValidationError{
			File: path, Field: "registers.float_byte_order",
			Message: fmt.Sprintf(
				"vendor %q typically uses \"big-endian-byte-swap\" (CDAB word order), got \"big-endian\"",
				vendor,
			),
			Severity: SeverityWarning, RuleID: "DEV-018",
		})
	}
	if strings.Contains(vendorLower, "allen-bradley") && reg.FloatByteOrder != "big-endian" {
		r.Add(ValidationError{
			File: path, Field: "registers.float_byte_order",
			Message: fmt.Sprintf(
				"vendor %q typically uses \"big-endian\", got %q",
				vendor, reg.FloatByteOrder,
			),
			Severity: SeverityWarning, RuleID: "DEV-018",
		})
	}
}

func checkVariantHolding(
	path, variantName string,
	registers []HoldingRegisterDoc,
	reg *RegistersDoc,
	r *ValidationResult,
) {
	names := make(map[string]bool)
	addresses := make(map[int]bool)

	for i, hr := range registers {
		field := fmt.Sprintf("register_map_variants.%s.holding[%d]", variantName, i)

		if hr.Address == nil {
			r.Add(ValidationError{
				File: path, Field: field + ".address",
				Message:  "required field is missing",
				Severity: SeverityError, RuleID: "DEV-010",
			})
			continue
		}
		addr := *hr.Address

		// DEV-016: addressing offset validation.
		if reg.MaxHolding != nil {
			checkHoldingAddressBounds(path, field, addr, reg, r)
		}

		// DEV-014: unique names.
		if names[hr.Name] {
			r.Add(ValidationError{
				File: path, Field: fmt.Sprintf("register_map_variants.%s.holding", variantName),
				Message:  fmt.Sprintf("duplicate register name %q", hr.Name),
				Severity: SeverityError, RuleID: "DEV-014",
			})
		}
		names[hr.Name] = true

		// DEV-015: unique addresses.
		if addresses[addr] {
			r.Add(ValidationError{
				File: path, Field: fmt.Sprintf("register_map_variants.%s.holding", variantName),
				Message:  fmt.Sprintf("duplicate register address %d", addr),
				Severity: SeverityError, RuleID: "DEV-015",
			})
		}
		addresses[addr] = true

		// DEV-012: scale_min < scale_max.
		if hr.ScaleMin != nil && hr.ScaleMax != nil && *hr.ScaleMin >= *hr.ScaleMax {
			r.Add(ValidationError{
				File: path, Field: field,
				Message: fmt.Sprintf(
					"scale_min (%.6g) must be less than scale_max (%.6g)", *hr.ScaleMin, *hr.ScaleMax,
				),
				Severity: SeverityError, RuleID: "DEV-012",
			})
		}

		// DEV-013: unit non-empty.
		if hr.Unit == "" {
			r.Add(ValidationError{
				File: path, Field: field + ".unit",
				Message:  "required field for holding registers",
				Severity: SeverityError, RuleID: "DEV-013",
			})
		}

		// DEV-019: writable sensor measurement registers (warning).
		if hr.Writable != nil && *hr.Writable {
			checkWritableMeasurementWarning(path, field, hr.Name, variantName, r)
		}
	}
}

func checkHoldingAddressBounds(path, field string, addr int, reg *RegistersDoc, r *ValidationResult) {
	maxH := *reg.MaxHolding
	if reg.Addressing == "one-based" {
		if addr == 0 {
			r.Add(ValidationError{
				File: path, Field: field + ".address",
				Message:  "address 0 is invalid for one-based addressing",
				Severity: SeverityError, RuleID: "DEV-016",
			})
			return
		}
		// Valid range: 1 through maxH.
		if addr > maxH {
			r.Add(ValidationError{
				File: path, Field: field + ".address",
				Message: fmt.Sprintf(
					"address %d exceeds max address %d for one-based addressing", addr, maxH,
				),
				Severity: SeverityError, RuleID: "DEV-016",
			})
		}
	} else {
		// zero-based: valid range 0 through maxH-1.
		if addr < 0 || addr >= maxH {
			r.Add(ValidationError{
				File: path, Field: field + ".address",
				Message: fmt.Sprintf(
					"address %d exceeds max address %d for zero-based addressing", addr, maxH-1,
				),
				Severity: SeverityError, RuleID: "DEV-010",
			})
		}
	}
}

func checkWritableMeasurementWarning(path, field, name, variantName string, r *ValidationResult) {
	nameLower := strings.ToLower(name)
	for _, suffix := range measurementSuffixes {
		if strings.Contains(nameLower, suffix) {
			r.Add(ValidationError{
				File: path, Field: field,
				Message: fmt.Sprintf(
					"register %q appears to be a sensor measurement but is marked writable", name,
				),
				Severity: SeverityWarning, RuleID: "DEV-019",
			})
			return
		}
	}
}

func checkVariantCoils(
	path, variantName string,
	coils []CoilDoc,
	reg *RegistersDoc,
	r *ValidationResult,
) {
	names := make(map[string]bool)
	addresses := make(map[int]bool)

	for i, c := range coils {
		field := fmt.Sprintf("register_map_variants.%s.coils[%d]", variantName, i)

		if c.Address == nil {
			r.Add(ValidationError{
				File: path, Field: field + ".address",
				Message:  "required field is missing",
				Severity: SeverityError, RuleID: "DEV-011",
			})
			continue
		}
		addr := *c.Address

		// DEV-011: coil address within max_coils.
		if reg.MaxCoils != nil {
			checkCoilAddressBounds(path, field, addr, reg, r)
		}

		// DEV-014: unique names.
		if names[c.Name] {
			r.Add(ValidationError{
				File: path, Field: fmt.Sprintf("register_map_variants.%s.coils", variantName),
				Message:  fmt.Sprintf("duplicate register name %q", c.Name),
				Severity: SeverityError, RuleID: "DEV-014",
			})
		}
		names[c.Name] = true

		// DEV-015: unique addresses.
		if addresses[addr] {
			r.Add(ValidationError{
				File: path, Field: fmt.Sprintf("register_map_variants.%s.coils", variantName),
				Message:  fmt.Sprintf("duplicate register address %d", addr),
				Severity: SeverityError, RuleID: "DEV-015",
			})
		}
		addresses[addr] = true
	}
}

func checkCoilAddressBounds(path, field string, addr int, reg *RegistersDoc, r *ValidationResult) {
	maxC := *reg.MaxCoils
	if reg.Addressing == "one-based" {
		if addr == 0 {
			r.Add(ValidationError{
				File: path, Field: field + ".address",
				Message:  "address 0 is invalid for one-based addressing",
				Severity: SeverityError, RuleID: "DEV-016",
			})
			return
		}
		if addr > maxC {
			r.Add(ValidationError{
				File: path, Field: field + ".address",
				Message: fmt.Sprintf(
					"address %d exceeds max_coils (%d)", addr, maxC,
				),
				Severity: SeverityError, RuleID: "DEV-011",
			})
		}
	} else {
		if addr < 0 || addr >= maxC {
			r.Add(ValidationError{
				File: path, Field: field + ".address",
				Message: fmt.Sprintf(
					"address %d exceeds max_coils (%d)", addr, maxC,
				),
				Severity: SeverityError, RuleID: "DEV-011",
			})
		}
	}
}
