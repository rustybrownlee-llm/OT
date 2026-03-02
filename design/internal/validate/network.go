package validate

import (
	"fmt"
	"net"
)

// validNetworkTypes lists all accepted network.type values.
var validNetworkTypes = map[string]bool{
	"ethernet":      true,
	"serial-rs485":  true,
	"serial-rs232":  true,
	"serial-rs422":  true,
}

// serialNetworkTypes lists network types that must not have a subnet.
var serialNetworkTypes = map[string]bool{
	"serial-rs485": true,
	"serial-rs232": true,
	"serial-rs422": true,
}

// ValidateNetwork loads and validates a network atom YAML file.
// It applies all NET-001 through NET-006 rules and returns a ValidationResult.
func ValidateNetwork(path string) *ValidationResult {
	result := &ValidationResult{}

	doc, err := LoadFile(path)
	if err != nil {
		result.Add(ValidationError{
			File: path, Message: err.Error(),
			Severity: SeverityError, RuleID: "NET-000",
		})
		return result
	}

	checkNetworkSchemaVersion(path, doc, result)
	if doc.Network == nil {
		result.Add(ValidationError{
			File:     path,
			Field:    "network",
			Message:  "network: required top-level section is missing",
			Severity: SeverityError,
			RuleID:   "NET-002",
		})
		return result
	}

	checkNetworkRequiredFields(path, doc.Network, result)
	checkNetworkProperties(path, doc.Network, doc.Properties, result)
	return result
}

func checkNetworkSchemaVersion(path string, doc *RawDocument, r *ValidationResult) {
	if doc.SchemaVersion != "0.1" {
		r.Add(ValidationError{
			File:  path,
			Field: "schema_version",
			Message: fmt.Sprintf(
				"missing or unsupported (got %q, expected \"0.1\")", doc.SchemaVersion,
			),
			Severity: SeverityError,
			RuleID:   "NET-001",
		})
	}
}

func checkNetworkRequiredFields(path string, n *NetworkDoc, r *ValidationResult) {
	if n.ID == "" {
		r.Add(ValidationError{
			File: path, Field: "network.id",
			Message:  "required field is missing or empty",
			Severity: SeverityError, RuleID: "NET-002",
		})
	}

	if !validNetworkTypes[n.Type] {
		r.Add(ValidationError{
			File: path, Field: "network.type",
			Message: fmt.Sprintf(
				"invalid value %q (must be one of: ethernet, serial-rs485, serial-rs232, serial-rs422)",
				n.Type,
			),
			Severity: SeverityError, RuleID: "NET-003",
		})
	}
}

func checkNetworkProperties(path string, n *NetworkDoc, props *NetworkPropertiesDoc, r *ValidationResult) {
	isSerial := serialNetworkTypes[n.Type]
	isEthernet := n.Type == "ethernet"

	var subnet string
	if props != nil {
		subnet = props.Subnet
	}

	if isEthernet {
		if subnet == "" {
			r.Add(ValidationError{
				File: path, Field: "properties.subnet",
				Message:  `required for ethernet networks, must be valid CIDR (e.g., "10.10.10.0/24")`,
				Severity: SeverityError, RuleID: "NET-004",
			})
		} else if _, _, err := net.ParseCIDR(subnet); err != nil {
			r.Add(ValidationError{
				File: path, Field: "properties.subnet",
				Message: fmt.Sprintf(
					"invalid CIDR notation %q: must be valid CIDR (e.g., \"10.10.10.0/24\")", subnet,
				),
				Severity: SeverityError, RuleID: "NET-004",
			})
		}
	}

	if isSerial && subnet != "" {
		r.Add(ValidationError{
			File: path, Field: "properties.subnet",
			Message:  "serial networks must not have a subnet field",
			Severity: SeverityError, RuleID: "NET-005",
		})
	}

	// NET-006: span_capable must be boolean (YAML unmarshalling to *bool handles this).
	// If the value is present, the *bool field will be non-nil.
	// If it is not a boolean in the YAML, gopkg.in/yaml.v3 returns an unmarshal error,
	// which is caught at LoadFile time. No additional check is needed here.
}
