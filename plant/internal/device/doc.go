// Package device loads and validates device profiles from YAML configuration.
// A device profile defines the observable behavior of a specific PLC model:
// communication parameters, register map limits, response timing, diagnostic
// capabilities, and network discoverability.
//
// Per ADR-004, vendor-specific behavior is applied as a configuration overlay
// rather than code. This allows adding new device types by creating YAML
// profile files under plant/config/devices/ without modifying the PLC engine.
//
// The four initial profiles (CompactLogix L33ER, SLC-500/05, Modicon 984,
// Moxa NPort 5150) are defined in SOW-001.0 and SOW-004.0.
//
// Implemented in SOW-001.0.
package device
