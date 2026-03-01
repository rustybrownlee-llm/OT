// validate.go contains validation logic for ResolvedEnvironment.
// Validate() is called by LoadEnvironment after all refs are resolved.
package config

import "fmt"

// Validate checks the resolved environment for consistency errors.
// It verifies that required fields are non-empty, all placement device refs
// resolve, all placement network refs resolve, placement IDs are unique,
// and Modbus port assignments are in the non-privileged range.
func (r *ResolvedEnvironment) Validate() error {
	if err := validateEnvMeta(r.Env.Environment); err != nil {
		return err
	}

	seen := make(map[string]bool)
	for _, p := range r.Env.Placements {
		if err := validatePlacement(p, r, seen); err != nil {
			return err
		}
		seen[p.ID] = true
	}

	return nil
}

// validateEnvMeta checks required environment metadata fields.
func validateEnvMeta(meta EnvMeta) error {
	if meta.Name == "" {
		return fmt.Errorf("environment.name must not be empty")
	}
	return nil
}

// validatePlacement checks one placement entry for referential integrity and port validity.
func validatePlacement(p Placement, r *ResolvedEnvironment, seen map[string]bool) error {
	if p.ID == "" {
		return fmt.Errorf("placement is missing an id field")
	}

	if seen[p.ID] {
		return fmt.Errorf("duplicate placement id %q", p.ID)
	}

	if _, ok := r.Devices[p.Device]; !ok {
		return fmt.Errorf("placement %q references unknown device %q", p.ID, p.Device)
	}

	if err := validatePlacementNetworks(p, r); err != nil {
		return err
	}

	if err := validateRegisterMapVariant(p, r); err != nil {
		return err
	}

	if p.ModbusPort != 0 {
		if err := validatePort(fmt.Sprintf("placement %q modbus_port", p.ID), p.ModbusPort); err != nil {
			return err
		}
	}

	return nil
}

// validatePlacementNetworks checks that a placement's primary and additional networks resolve.
func validatePlacementNetworks(p Placement, r *ResolvedEnvironment) error {
	if p.Network != "" {
		if _, ok := r.Networks[p.Network]; !ok {
			return fmt.Errorf("placement %q references unknown network %q", p.ID, p.Network)
		}
	}

	for _, an := range p.AdditionalNetworks {
		if _, ok := r.Networks[an.Network]; !ok {
			return fmt.Errorf(
				"placement %q additional_network references unknown network %q",
				p.ID, an.Network,
			)
		}
	}

	return nil
}

// validateRegisterMapVariant checks that a placement's register_map_variant, when non-empty,
// names a variant that exists in the device's RegisterMapVariants map.
func validateRegisterMapVariant(p Placement, r *ResolvedEnvironment) error {
	if p.RegisterMapVariant == "" {
		return nil
	}

	dev := r.Devices[p.Device]
	if _, ok := dev.RegisterMapVariants[p.RegisterMapVariant]; !ok {
		return fmt.Errorf(
			"placement %q specifies register_map_variant %q not found in device %q",
			p.ID, p.RegisterMapVariant, p.Device,
		)
	}

	return nil
}

// validatePort checks that port is within the non-privileged range (1024-65535).
func validatePort(field string, port int) error {
	if port < 1024 || port > 65535 {
		return fmt.Errorf("%s %d is outside the non-privileged port range (1024-65535)", field, port)
	}
	return nil
}
