// Package process coordinates the physical process simulation across both
// the water treatment plant and the manufacturing floor. It defines the
// ProcessModel interface and SimulationEngine that tick all models every second,
// driving register values toward realistic operating states.
//
// Per ADR-001, process fidelity targets plausible engineering values rather
// than high-precision physics simulation. The intent is to produce realistic
// register values for protocol training, not accurate fluid dynamics.
//
// The 1-second simulation tick is a simulator throughput constraint, not a
// model of PLC scan time. Real PLCs scan at 5-100ms depending on the platform.
//
// Process models: IntakeModel, TreatmentModel, DistributionModel (water treatment),
// LineAModel, CoolingModel (manufacturing), GatewayModel (serial gateway).
//
// Implemented in SOW-003.0.
package process
