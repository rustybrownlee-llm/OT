// Package plc implements the virtual PLC engine that serves Modbus TCP and
// simulated Modbus RTU traffic. Each virtual PLC maintains a register map
// (coils, discrete inputs, input registers, holding registers) and responds
// to Modbus function codes FC01 through FC06, FC15, and FC16.
//
// Device-specific behavior (response timing, register limits, addressing
// conventions) is applied via device profiles loaded from YAML configuration.
// See ADR-004 for the device profile specification and ADR-002 for protocol
// priority decisions.
//
// Implemented in SOW-001.0.
package plc
