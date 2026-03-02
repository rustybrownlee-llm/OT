package validate

import (
	"fmt"
	"net"
	"time"
)

// checkSinglePlacement applies ENV-005 through ENV-019 to one placement entry.
func checkSinglePlacement(
	envFile string,
	i int,
	p PlacementDoc,
	networkDocs map[string]*RawDocument,
	deviceDocs map[string]*RawDocument,
	envNetworkSet map[string]bool,
	placementIDs map[string]int,
	networkIPs map[string]map[string]string,
	usedPorts map[int]string,
	gwSerialAddrs map[string]map[int]string,
	r *ValidationResult,
) {
	field := func(f string) string { return fmt.Sprintf("placements[%d].%s", i, f) }

	// ENV-006: unique placement IDs.
	if first, dup := placementIDs[p.ID]; dup {
		r.Add(ValidationError{
			File: envFile, Field: fmt.Sprintf("placements[%d].id", i),
			Message: fmt.Sprintf(
				"duplicate placement ID %q (first seen at placements[%d])", p.ID, first,
			),
			Severity: SeverityError, RuleID: "ENV-006",
		})
	} else if p.ID != "" {
		placementIDs[p.ID] = i
	}

	// ENV-007: placement network must be listed in environment networks.
	if !envNetworkSet[p.Network] {
		r.Add(ValidationError{
			File: envFile, Field: field("network"),
			Message: fmt.Sprintf(
				"network %q not listed in environment networks", p.Network,
			),
			Severity: SeverityError, RuleID: "ENV-007",
		})
	}

	netDoc := networkDocs[p.Network]
	devDoc := deviceDocs[p.Device]

	// ENV-005: register_map_variant must exist in device.
	if devDoc != nil && p.RegisterMapVariant != "" {
		checkVariantExists(envFile, i, p, devDoc, r)
	}

	// ENV-016: gateway-category devices must declare bridges.
	if devDoc != nil && devDoc.Device != nil && devDoc.Device.Category == "gateway" {
		if len(p.Bridges) == 0 {
			r.Add(ValidationError{
				File: envFile, Field: fmt.Sprintf("placements[%d]", i),
				Message: fmt.Sprintf(
					"device %q is a gateway but has no bridges field", p.Device,
				),
				Severity: SeverityError, RuleID: "ENV-016",
			})
		}
	}

	// ENV-018: device port type must match network type.
	if devDoc != nil && netDoc != nil && netDoc.Network != nil {
		checkDevicePortMatchesNetwork(envFile, i, p, devDoc, netDoc, r)
	}

	if p.Gateway != "" {
		checkSerialPlacement(envFile, i, p, gwSerialAddrs, r, field)
	} else {
		checkEthernetPlacement(envFile, i, p, netDoc, networkIPs, usedPorts, r, field)
	}

	// ENV-014, ENV-015: additional_networks.
	checkAdditionalNetworks(envFile, i, p, envNetworkSet, networkDocs, networkIPs, r)

	// ENV-022: installed year must be within valid range if present.
	if p.Installed != nil {
		checkInstalledYear(envFile, fmt.Sprintf("placements[%d].installed", i), *p.Installed, r, "ENV-022")
	}
}

// checkInstalledYear validates an installation year is within [1960, time.Now().Year()+2].
// The lower bound 1960 accommodates early PLC history (Modicon 084 shipped 1968).
// The upper bound +2 accommodates staged equipment ordered but not yet operational.
func checkInstalledYear(envFile, field string, year int, r *ValidationResult, ruleID string) {
	maxYear := time.Now().Year() + 2
	if year < 1960 || year > maxYear {
		r.Add(ValidationError{
			File:  envFile,
			Field: field,
			Message: fmt.Sprintf(
				"year %d is outside valid range 1960-%d (PLCs first deployed ~1968; +2 years accommodates staged equipment)",
				year, maxYear,
			),
			Severity: SeverityError, RuleID: ruleID,
		})
	}
}

// checkSerialPlacement applies ENV-013, ENV-017, ENV-019 for gateway-accessed devices.
func checkSerialPlacement(
	envFile string,
	i int,
	p PlacementDoc,
	gwSerialAddrs map[string]map[int]string,
	r *ValidationResult,
	field func(string) string,
) {
	// ENV-017: serial devices must not have ip or modbus_port.
	if p.IP != "" || p.ModbusPort != nil {
		r.Add(ValidationError{
			File: envFile, Field: fmt.Sprintf("placements[%d]", i),
			Message:  "serial device accessed via gateway must not have an ip or modbus_port (serial devices are IP-invisible)",
			Severity: SeverityError, RuleID: "ENV-017",
		})
	}

	if p.SerialAddress == nil {
		return
	}
	addr := *p.SerialAddress

	// ENV-019: serial_address must be 1-247.
	checkSerialAddressRange(envFile, i, addr, r)

	// ENV-013: unique serial addresses per gateway.
	gwAddrs := gwSerialAddrs[p.Gateway]
	if gwAddrs == nil {
		gwAddrs = make(map[int]string)
		gwSerialAddrs[p.Gateway] = gwAddrs
	}
	if prev := gwAddrs[addr]; prev != "" {
		r.Add(ValidationError{
			File: envFile, Field: field("serial_address"),
			Message: fmt.Sprintf(
				"address %d already used by placement %q on gateway %q", addr, prev, p.Gateway,
			),
			Severity: SeverityError, RuleID: "ENV-013",
		})
	} else {
		gwAddrs[addr] = p.ID
	}
}

// checkEthernetPlacement applies ENV-008, ENV-009, ENV-010 for direct Ethernet devices.
func checkEthernetPlacement(
	envFile string,
	i int,
	p PlacementDoc,
	netDoc *RawDocument,
	networkIPs map[string]map[string]string,
	usedPorts map[int]string,
	r *ValidationResult,
	field func(string) string,
) {
	if netDoc == nil || netDoc.Network == nil || netDoc.Network.Type != "ethernet" {
		return
	}

	// ENV-008: IP within subnet.
	if netDoc.Properties != nil && netDoc.Properties.Subnet != "" {
		checkIPInSubnet(envFile, i, "ip", p.IP, netDoc.Properties.Subnet, p.Network, r)
	}

	// ENV-009: no duplicate IPs on same network.
	if p.IP != "" {
		if networkIPs[p.Network] == nil {
			networkIPs[p.Network] = make(map[string]string)
		}
		if prev := networkIPs[p.Network][p.IP]; prev != "" {
			r.Add(ValidationError{
				File: envFile, Field: field("ip"),
				Message: fmt.Sprintf(
					"duplicate IP %q on network %q (first seen at placement %q)",
					p.IP, p.Network, prev,
				),
				Severity: SeverityError, RuleID: "ENV-009",
			})
		} else {
			networkIPs[p.Network][p.IP] = p.ID
		}
	}

	// ENV-010: no modbus_port collision.
	if p.ModbusPort != nil && *p.ModbusPort != 0 {
		if prev := usedPorts[*p.ModbusPort]; prev != "" {
			r.Add(ValidationError{
				File: envFile, Field: field("modbus_port"),
				Message: fmt.Sprintf(
					"port %d already used by placement %q", *p.ModbusPort, prev,
				),
				Severity: SeverityError, RuleID: "ENV-010",
			})
		} else {
			usedPorts[*p.ModbusPort] = p.ID
		}
	}
}

// checkVariantExists verifies the placement's register_map_variant is in the device.
func checkVariantExists(
	envFile string, idx int,
	p PlacementDoc,
	devDoc *RawDocument,
	r *ValidationResult,
) {
	if devDoc.Variants == nil {
		r.Add(ValidationError{
			File:  envFile,
			Field: fmt.Sprintf("placements[%d].register_map_variant", idx),
			Message: fmt.Sprintf(
				"variant %q not found in device %q (device has no variants)",
				p.RegisterMapVariant, p.Device,
			),
			Severity: SeverityError, RuleID: "ENV-005",
		})
		return
	}
	if _, ok := devDoc.Variants[p.RegisterMapVariant]; !ok {
		available := make([]string, 0, len(devDoc.Variants))
		for k := range devDoc.Variants {
			available = append(available, k)
		}
		r.Add(ValidationError{
			File:  envFile,
			Field: fmt.Sprintf("placements[%d].register_map_variant", idx),
			Message: fmt.Sprintf(
				"variant %q not found in device %q (available: %v)",
				p.RegisterMapVariant, p.Device, available,
			),
			Severity: SeverityError, RuleID: "ENV-005",
		})
	}
}

// checkSerialAddressRange verifies a serial_address is in the Modbus-valid range 1-247.
func checkSerialAddressRange(envFile string, idx, addr int, r *ValidationResult) {
	if addr < 1 || addr > 247 {
		r.Add(ValidationError{
			File:  envFile,
			Field: fmt.Sprintf("placements[%d].serial_address", idx),
			Message: fmt.Sprintf(
				"address %d is outside valid range 1-247 (0 is broadcast, 248-255 are reserved)", addr,
			),
			Severity: SeverityError, RuleID: "ENV-019",
		})
	}
}

// checkIPInSubnet verifies an IP address is within the given CIDR subnet.
func checkIPInSubnet(
	envFile string,
	idx int,
	fieldSuffix, ipStr, subnet, networkID string,
	r *ValidationResult,
) {
	if ipStr == "" {
		r.Add(ValidationError{
			File:  envFile,
			Field: fmt.Sprintf("placements[%d].%s", idx, fieldSuffix),
			Message: fmt.Sprintf(
				"ip address is required for ethernet network %q", networkID,
			),
			Severity: SeverityError, RuleID: "ENV-008",
		})
		return
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		r.Add(ValidationError{
			File:  envFile,
			Field: fmt.Sprintf("placements[%d].%s", idx, fieldSuffix),
			Message: fmt.Sprintf(
				"address %q is not a valid IPv4 address", ipStr,
			),
			Severity: SeverityError, RuleID: "ENV-008",
		})
		return
	}
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return // Subnet already flagged by NET-004.
	}
	if !ipNet.Contains(ip) {
		r.Add(ValidationError{
			File:  envFile,
			Field: fmt.Sprintf("placements[%d].%s", idx, fieldSuffix),
			Message: fmt.Sprintf(
				"address %q is not within subnet %q of network %q", ipStr, subnet, networkID,
			),
			Severity: SeverityError, RuleID: "ENV-008",
		})
	}
}

// checkAdditionalNetworks applies ENV-014 and ENV-015 for additional_networks entries.
func checkAdditionalNetworks(
	envFile string,
	idx int,
	p PlacementDoc,
	envNetworkSet map[string]bool,
	networkDocs map[string]*RawDocument,
	networkIPs map[string]map[string]string,
	r *ValidationResult,
) {
	for j, an := range p.AdditionalNetworks {
		if !envNetworkSet[an.Network] {
			r.Add(ValidationError{
				File:  envFile,
				Field: fmt.Sprintf("placements[%d].additional_networks[%d].network", idx, j),
				Message: fmt.Sprintf(
					"network %q not listed in environment networks", an.Network,
				),
				Severity: SeverityError, RuleID: "ENV-014",
			})
			continue
		}
		netDoc := networkDocs[an.Network]
		if netDoc == nil || netDoc.Network == nil || netDoc.Network.Type != "ethernet" {
			continue
		}
		if netDoc.Properties == nil || netDoc.Properties.Subnet == "" {
			continue
		}
		// ENV-015: IP within subnet.
		checkIPInSubnet(envFile, idx,
			fmt.Sprintf("additional_networks[%d].ip", j),
			an.IP, netDoc.Properties.Subnet, an.Network, r)

		// Track IP for ENV-009 duplicate detection on additional networks.
		if an.IP != "" {
			if networkIPs[an.Network] == nil {
				networkIPs[an.Network] = make(map[string]string)
			}
			if prev := networkIPs[an.Network][an.IP]; prev != "" {
				r.Add(ValidationError{
					File:  envFile,
					Field: fmt.Sprintf("placements[%d].additional_networks[%d].ip", idx, j),
					Message: fmt.Sprintf(
						"duplicate IP %q on network %q (first seen at placement %q)",
						an.IP, an.Network, prev,
					),
					Severity: SeverityError, RuleID: "ENV-009",
				})
			} else {
				networkIPs[an.Network][an.IP] = p.ID
			}
		}
	}
}

// checkGatewayBridge verifies the gateway bridges to the serial device's network (ENV-012).
func checkGatewayBridge(
	envFile string,
	idx int,
	p PlacementDoc,
	placements []PlacementDoc,
	placementIDs map[string]int,
	r *ValidationResult,
) {
	gw := placements[placementIDs[p.Gateway]]
	for _, br := range gw.Bridges {
		if br.ToNetwork == p.Network {
			return // Valid bridge found.
		}
	}
	r.Add(ValidationError{
		File:  envFile,
		Field: fmt.Sprintf("placements[%d].gateway", idx),
		Message: fmt.Sprintf(
			"gateway %q does not bridge to network %q (serial network must appear in to_network, not from_network)",
			p.Gateway, p.Network,
		),
		Severity: SeverityError, RuleID: "ENV-012",
	})
}

// checkDevicePortMatchesNetwork verifies the device has a port matching the network type (ENV-018).
func checkDevicePortMatchesNetwork(
	envFile string,
	idx int,
	p PlacementDoc,
	devDoc, netDoc *RawDocument,
	r *ValidationResult,
) {
	requiredPortType := networkToPortType(netDoc.Network.Type)
	if requiredPortType == "" {
		return
	}
	if devDoc.Connectivity == nil {
		r.Add(ValidationError{
			File: envFile, Field: fmt.Sprintf("placements[%d]", idx),
			Message: fmt.Sprintf(
				"device %q has no port matching network type %q (device has no connectivity section)",
				p.Device, netDoc.Network.Type,
			),
			Severity: SeverityError, RuleID: "ENV-018",
		})
		return
	}
	for _, port := range devDoc.Connectivity.Ports {
		if port.Type == requiredPortType {
			return
		}
	}
	r.Add(ValidationError{
		File: envFile, Field: fmt.Sprintf("placements[%d]", idx),
		Message: fmt.Sprintf(
			"device %q has no port matching network type %q", p.Device, netDoc.Network.Type,
		),
		Severity: SeverityError, RuleID: "ENV-018",
	})
}

// networkToPortType maps a network type string to the required device port type.
func networkToPortType(netType string) string {
	switch netType {
	case "ethernet":
		return "ethernet"
	case "serial-rs485":
		return "rs485"
	case "serial-rs232":
		return "rs232"
	case "serial-rs422":
		return "rs422"
	default:
		return ""
	}
}
