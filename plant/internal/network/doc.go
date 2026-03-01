// Package network models the logical network topology of the simulated facility.
// It tracks which virtual devices reside on which network segments, enforces
// the access rules between Purdue model levels (ADR-003, D1), and provides
// the cross-plant link configuration between the water treatment plant and
// the manufacturing floor (ADR-003, D3).
//
// Network topology is defined in YAML configuration and loaded at startup.
// Code changes are not required to add new network segments or adjust
// inter-segment access rules.
//
// Implemented in SOW-005.0.
package network
