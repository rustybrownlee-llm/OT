// Package modbus implements the Modbus TCP protocol adapter for the plant
// simulator. It wraps the simonvetter/modbus library to provide a server that
// exposes each virtual PLC's register map over a dedicated TCP port in the
// range 5020-5039.
//
// The adapter handles MBAP framing, function code dispatch, and exception
// response generation. Per ADR-002, Modbus TCP is the Phase 1 protocol.
// Modbus RTU over simulated serial is Phase 2 (see protocol/serial).
//
// Wire-level fidelity is the primary goal: function codes, register addresses,
// byte order, and exception codes must match the Modbus Application Protocol
// Specification V1.1b3.
//
// Implemented in SOW-001.0.
package modbus
