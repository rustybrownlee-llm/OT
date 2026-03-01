// Package serial simulates Modbus RTU over a serial bus, representing the
// RS-485 connections between legacy PLCs and the serial-to-Ethernet gateway
// (Moxa NPort 5150) on the manufacturing floor.
//
// The package models Modbus RTU framing (CRC-16 validation, device address
// byte, function code), simulates the latency added by serial baud rates
// (9600-19200 baud), and enforces the single-master constraint of RS-485.
//
// This package does not control real serial hardware. It simulates the timing
// and framing characteristics so that monitoring tools see realistic RTU
// bridge behavior from the Modbus TCP side. Per ADR-002, serial simulation
// is Phase 2 after Modbus TCP is validated.
//
// Implemented in SOW-004.0.
package serial
