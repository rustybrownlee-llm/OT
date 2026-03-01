# ADR-001: Simulation Fidelity Philosophy

**Status**: Proposed
**Date**: 2026-02-28
**Decision Makers**: Rusty Brownlee

---

## Context

The OT Simulator must model physical processes (water treatment, manufacturing) and expose them via industrial protocols (Modbus TCP/RTU, BACnet, OPC UA). A fundamental design tension exists between two types of fidelity:

| Fidelity Type | What It Means | Who Cares |
|---------------|---------------|-----------|
| Physics fidelity | Accurate fluid dynamics, thermodynamics, chemical kinetics | Process engineers, control system designers |
| Protocol fidelity | Authentic wire-level protocol behavior, realistic register maps, correct function codes | IT security engineers, SOC analysts, OT security teams |

The target audience is IT engineers learning OT security. They need to understand how Modbus works on the wire, what happens when an unauthorized write changes a setpoint, and why legacy protocols have no authentication. They do not need to validate PID controller tuning parameters or verify Bernoulli equation accuracy.

### Existing Simulator Landscape

| Simulator | Physics Fidelity | Protocol Fidelity | Teaching Value |
|-----------|-----------------|-------------------|---------------|
| MITRE Aloha | Trivial (accumulator) | Basic (Modbus TCP, BACnet) | Low -- too simple |
| GRFICSv3 | Moderate (chemical process) | Good (OpenPLC, real Modbus) | High -- but GPL, 20GB+ |
| ICSSIM | Basic (bottle filling) | Basic (Modbus TCP) | Medium -- Purdue layers |
| MiniCPS | Framework (you build it) | Via Mininet emulation | Medium -- abandoned |

No existing simulator prioritizes protocol fidelity for IT security training.

---

## Decision

### D1: Protocol Fidelity Over Physics Fidelity

**Decision**: The simulator prioritizes wire-level protocol authenticity over physical process accuracy.

**Options Considered**:
- (a) High physics fidelity with simplified protocols
- (b) High protocol fidelity with plausible physics
- (c) Both high fidelity

**Chosen**: (b) High protocol fidelity with plausible physics.

**Rationale**: The audience is IT engineers, not process engineers. A tank that fills linearly when a pump is on teaches the same security lesson as one modeled with Navier-Stokes equations. But a Modbus implementation that uses wrong function codes or unrealistic register layouts teaches nothing useful.

**Protocol fidelity requirements**:
- Correct Modbus TCP frame structure on the wire (MBAP header, function codes, exception responses)
- Realistic register maps matching real-world water treatment and manufacturing equipment
- Proper 16-bit scaled integer representation (0-32767 mapped to engineering ranges)
- IEEE 754 float pairs for modern instruments (two consecutive registers)
- Correct coil/discrete input/input register/holding register separation
- Authentic polling intervals (1-second process, 5-second quality, 60-second totals)
- Realistic device response times (10-50ms per request for PLCs)

**Physics fidelity requirements (minimum viable)**:
- Plausible cause-and-effect (pump on -> tank fills, valve open -> flow increases)
- Values that stay within realistic engineering ranges
- Sensor noise (small random fluctuations around true value)
- Time-dependent behavior (chlorine decays, temperature drifts, filters clog gradually)
- Actuator response delay (pump doesn't reach full speed instantly)

### D2: Register Values Must Tell a Believable Story

**Decision**: Register values must be internally consistent and change in response to process conditions, even if the underlying model is simplified.

**Rationale**: An IT engineer watching Modbus traffic in Wireshark should see values that make sense together. If the pump is off, flow should be zero. If turbidity is high, the filter differential pressure should be rising. These correlations are what analysts use to detect anomalies -- a process value that changes without the expected correlated changes is suspicious.

**Correlation examples**:
- Pump running + flow rate zero = pump fault or valve closed (alarm condition)
- Tank level rising + no inflow = sensor fault (anomaly)
- Chlorine residual dropping over time = normal decay (expected behavior)
- pH spiking with no chemical dosing change = process upset or attack

---

## Consequences

### Positive
- Simulator produces protocol traffic indistinguishable from real devices at the Modbus frame level
- IT engineers develop transferable skills (Wireshark analysis, Modbus parsing)
- Register maps serve as reference material for real-world OT assessments
- Physics model is simple enough to implement, test, and extend quickly

### Negative
- Simulator cannot be used for process engineering validation or PID tuning
- Oversimplified physics may create false confidence about process behavior
- Control engineers may find the simulation unrealistic

### Risks
- **Risk**: Physics model too simplified, producing obviously unrealistic register patterns
- **Mitigation**: Validate register value patterns against SWaT dataset (51 sensors, 1-second intervals from real water treatment plant) to ensure correlations are plausible

---

## References
- SWaT Testbed Dataset (iTrust SUTD): https://itrust.sutd.edu.sg/itrust-labs_datasets/dataset_info/
- MITRE Aloha Water Treatment Simulator: https://github.com/mitre/aloha-water-treatment
- Modbus Application Protocol Specification V1.1b3
