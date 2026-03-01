// Package process coordinates the physical process simulation across both
// the water treatment plant and the manufacturing floor. It defines the
// shared interfaces that sub-packages (water, manufacturing) must satisfy
// and provides the update loop that advances process state on each tick.
//
// Per ADR-001, process fidelity targets plausible engineering values rather
// than high-precision physics simulation. The intent is to produce realistic
// register values for protocol training, not accurate fluid dynamics.
//
// Implemented in SOW-002.0.
package process
