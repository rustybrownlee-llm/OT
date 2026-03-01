// Package manufacturing simulates the legacy manufacturing floor process.
// It models the physical process of a ~1993 manufacturing facility: material
// handling, assembly line conveyor control, and cooling water consumption
// from the water treatment plant.
//
// The manufacturing floor operates on a flat 192.168.1.0/24 network with no
// segmentation (ADR-003, D2). PLCs are serial devices accessed via a
// serial-to-Ethernet gateway, and device behavior is governed by device
// profiles (ADR-004) defined in YAML configuration.
//
// Implemented in SOW-004.0.
package manufacturing
