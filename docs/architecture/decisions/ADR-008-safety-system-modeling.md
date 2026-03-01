# ADR-008: Safety System Modeling

**Status**: Proposed
**Date**: 2026-02-28
**Decision Makers**: Rusty Brownlee

---

## Context

Industrial environments have two distinct control layers:

1. **Basic Process Control System (BPCS)**: The PLCs, SCADA, and HMI that run the process under normal conditions. This is what the simulator primarily models.

2. **Safety Instrumented System (SIS)**: An independent system designed to bring the process to a safe state when dangerous conditions are detected. SIS operates independently of the BPCS -- if the BPCS fails completely, the SIS still functions.

### Why IT Engineers Must Understand SIS

| Misconception | Reality |
|---------------|---------|
| "We can firewall the safety system like any other server" | If the firewall blocks SIS communication when it matters, people die |
| "Safety is a software feature of the PLC" | SIS is physically separate hardware with its own power supply, wiring, and logic |
| "We should patch the safety controllers too" | SIS changes require SIL (Safety Integrity Level) recertification -- a months-long process |
| "Let's put the safety system on the network for monitoring" | Adding network connectivity to SIS adds attack surface to the most critical layer |
| "If we detect a breach, we should shut everything down" | Uncontrolled shutdown can be more dangerous than a compromised process |

The TRITON/TRISIS attack (2017) targeted a Schneider Electric Triconex safety controller at a Saudi Arabian petrochemical plant. The attacker's goal was to disable the SIS so that a simultaneous process attack could cause physical damage without triggering a safe shutdown. IT engineers must understand what SIS is and why it requires different security treatment than BPCS.

### Safety in the Two Environments

| Environment | Safety Implementation | Characteristics |
|-------------|----------------------|-----------------|
| Water treatment (modern) | Safety PLC (separate controller) | Dedicated safety controller monitoring critical parameters (chlorine levels, tank overflow, pump overspeed). Trips process to safe state independently of BPCS. Connected via dedicated safety network. |
| Manufacturing (legacy) | Hardwired e-stop circuit | Physical relay circuit with mushroom-head e-stop buttons at each workstation. No software involvement. Removes power to motors and actuators directly. Cannot be bypassed by any network attack. |

---

## Decision

### D1: Model Safety as Separate from Process Control

**Decision**: The simulator models the safety system as a distinct layer, independent of the BPCS PLCs. Safety logic runs in its own process/container and can bring the simulated process to a safe state even if all BPCS PLCs are compromised.

**Water treatment SIS model**:
- Independent safety controller (simulated as a separate process)
- Monitors: tank high-high level, pump overspeed, chlorine high, pH extreme deviation
- Action: trips to safe state (stops pumps, closes valves, opens drain)
- Communication: dedicated safety network (separate Docker network from BPCS)
- Not accessible from the BPCS network under normal configuration

**Manufacturing e-stop model**:
- Hardwired relay logic (simulated as a simple state machine)
- Triggered by: e-stop button press (simulated via scenario event or HMI)
- Action: removes power to all motors and actuators (all coils forced to 0)
- Communication: none -- this is a physical circuit, not a network device
- Cannot be attacked via Modbus because it is not on any network

### D2: Safety System is a Later Phase

**Decision**: SIS modeling is not included in Phase 1. It is designed here for architectural clarity but implemented after the BPCS is functional.

**Rationale**: The BPCS must work before we can demonstrate what happens when it fails and the SIS takes over. Phase 1 establishes the PLCs, process simulation, and Modbus communication. The SIS adds the safety layer on top.

**Phase mapping**:
- Phase 1-3: BPCS only (PLCs, process simulation, HMI)
- Phase 4+: SIS modeling (safety controller, e-stop, trip logic)
- Scenario integration: Attack scenarios demonstrate "what happens when SIS is absent/disabled"

### D3: E-Stop Is Always Functional

**Decision**: The manufacturing hardwired e-stop works from day one, even before the formal SIS phase. It is a simple boolean state that, when triggered, forces all manufacturing coils to 0.

**Rationale**: E-stop is the most basic safety mechanism and is trivial to implement. It also creates an immediate teaching moment: an IT engineer can trigger the e-stop and observe the process response. More importantly, they learn that this safety mechanism cannot be disabled via Modbus because it is not a Modbus device.

### D4: Do Not Simulate Safety System Attacks in Early Phases

**Decision**: TRITON/TRISIS-style attacks on the safety system are not included in initial scenarios. Safety system attack scenarios require both a functional SIS and a mature understanding of BPCS attacks.

**Rationale**: Safety system attacks are the most advanced and dangerous category of OT attacks. Engineers must first understand BPCS attacks (unauthorized Modbus writes, setpoint changes) before progressing to SIS attacks. Introducing SIS attacks too early risks normalizing attacks on safety-critical systems without adequate context about the consequences.

**Future scenario**: "Scenario 08: Safety System Integrity" -- Verify that the SIS triggers correctly when the BPCS is compromised. Detect attempts to modify safety controller logic. This scenario requires completing all four initial scenarios first.

---

## Consequences

### Positive
- Engineers learn the BPCS/SIS separation that is fundamental to OT safety
- Hardwired e-stop demonstrates that not everything is network-attackable
- Safety modeled as independent layer prevents "security compromise = unsafe process" assumption
- Phased implementation keeps initial scope manageable

### Negative
- SIS modeling adds complexity (separate containers, separate networks, separate logic)
- Deferring SIS to later phases means early scenarios lack safety system context
- No TRITON/TRISIS scenario initially limits advanced training

### Risks
- **Risk**: Engineers may develop a false sense that BPCS compromise is the worst case
- **Mitigation**: Scenario documentation explicitly references SIS and notes it as a future phase. Background reading on TRITON included in scenario materials.
- **Risk**: Hardwired e-stop is so simple it may seem unimportant
- **Mitigation**: Include a scenario that asks "what if the e-stop was networked instead of hardwired?" to illustrate why physical circuits matter.

---

## References
- IEC 61511: Safety Instrumented Systems for the Process Industry Sector
- TRITON/TRISIS Attack Analysis (Dragos): https://www.dragos.com/blog/trisis-triton-hatman-malware/
- NIST SP 800-82 Rev. 3: Section 6.2 Safety System Integration
- ISA-84 / IEC 61511: Functional Safety -- Safety Instrumented Systems
