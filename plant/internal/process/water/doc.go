// Package water simulates the water treatment plant process.
// It models the Purdue Level 0 physical process: intake pumping, chemical
// dosing, filtration, and clear well storage. Process variables (flow rates,
// chemical levels, turbidity, pH, chlorine residual) are updated on each
// simulation tick and written to the PLC register map via the plc package.
//
// The water treatment plant follows the Purdue model (ADR-003, D1) with
// VLANs at Levels 1-3 and firewall-controlled Modbus access between levels.
//
// Implemented in SOW-002.0.
package water
