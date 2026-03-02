// Package process coordinates the physical process simulation across the water
// treatment plant, manufacturing floor, and pipeline monitoring station. It defines
// the ProcessModel interface and SimulationEngine that tick all models every second,
// driving register values toward realistic operating states.
//
// Per ADR-001, process fidelity targets plausible engineering values rather
// than high-precision physics simulation. The intent is to produce realistic
// register values for protocol training, not accurate fluid dynamics.
//
// The 1-second simulation tick is a simulator throughput constraint, not a
// model of PLC scan time. Real PLCs scan at 5-100ms depending on the platform.
//
// Water treatment models: IntakeModel, TreatmentModel, DistributionModel.
// Manufacturing models: LineAModel, CoolingModel.
// Gateway models: GatewayModel (serial-gateway), PipelineGatewayModel (pipeline-serial).
// Pipeline models: CompressorModel (compressor-control), MeteringModel (pipeline-metering),
// StationMonitorModel (station-monitoring), GasAnalysisModel (gas-analysis).
//
// Water treatment and manufacturing implemented in SOW-003.0.
// Pipeline models implemented in SOW-009.0.
package process
