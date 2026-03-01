# ADR-007: Educational Scenario Framework

**Status**: Proposed (Revised 2026-03-01)
**Date**: 2026-02-28
**Decision Makers**: Rusty Brownlee
**Revised by**: ADR-009 (Design Layer and Composable Environments)

---

## Context

The simulator's primary purpose is education. The plant runs and produces realistic OT traffic, but without structured scenarios, an IT engineer may not know what to look at, what to try, or how to measure their progress.

### Target Audience

IT engineers and security analysts with strong enterprise IT backgrounds but limited OT exposure. They understand TCP/IP, firewalls, SIEM, and incident response. They do not understand:
- Industrial protocols (Modbus, DNP3, EtherNet/IP)
- PLC programming and operation
- Physical process control (PID loops, setpoints, safety interlocks)
- The operational culture of OT (availability-first, no patching, no rebooting)
- How to assess and secure an environment they cannot install agents on

### Learning Progression

The natural progression for IT engineers entering OT follows this pattern:

```
Discovery        ->  Assessment       ->  Attack          ->  Defense
"What's here?"       "What's exposed?"    "What can break?"   "How do we fix it?"
```

Each phase builds on the previous. An engineer who cannot discover the devices on the network cannot assess their vulnerabilities. An engineer who does not understand the attack surface cannot prioritize defenses.

### Relationship to the Design Layer

Per ADR-009, environment topology is defined in the design layer. Scenarios leverage this by referencing environment templates from `design/environments/` rather than embedding their own topology overrides. A scenario specifies *which* environment to run and *what* to do in it, keeping environment composition and educational content cleanly separated.

---

## Decision

### D1: Scenarios Follow a Four-Phase Learning Progression

**Decision**: Scenarios are organized into four phases, each building on the skills developed in the previous phase.

| Phase | Name | Goal | Skills Developed |
|-------|------|------|-----------------|
| 1 | Discovery | Map the OT environment, build an asset inventory | Network scanning, Modbus polling, serial device awareness, asset documentation |
| 2 | Assessment | Identify vulnerabilities and attack surface | Protocol analysis, authentication gaps, network segmentation review, legacy risk identification |
| 3 | Attack Simulation | Demonstrate impact of OT attacks | Unauthorized Modbus writes, setpoint manipulation, man-in-the-middle on Modbus, lateral movement IT->OT |
| 4 | Defense | Deploy and validate security controls | Monitoring deployment, anomaly detection, Blastwave/Dragos integration, segmentation improvement |

### D2: Scenario Structure

**Decision**: Each scenario is a self-contained directory under `/scenarios/` with a standard file structure. Scenarios reference design layer environment templates rather than embedding topology configuration.

```
scenarios/
  01-discovery/
    README.md                # Instructions, context, objectives
    environment.ref          # Points to design/environments/{id}
    success-criteria.md      # How to know you have completed the scenario
    hints/
      hint-1.md              # Progressive hints (optional)
      hint-2.md
    solutions/
      solution.md            # Reference solution (for self-study)
  02-vulnerability-assessment/
  03-attack-simulation/
  04-defense/
```

**environment.ref format** (simple text file):
```
# Environment template for this scenario
environment: greenfield-water-mfg
```

This indirection means:
- Scenarios don't duplicate topology definitions
- The same environment can serve multiple scenarios
- Scenarios that need a modified environment (e.g., with a compromised device) reference a scenario-specific environment template in `design/environments/`
- Environment updates automatically flow to all scenarios that reference them

**README.md format**:
```markdown
# Scenario NN: [Title]

**Phase**: [Discovery | Assessment | Attack | Defense]
**Environment**: [environment template ID from design/environments/]
**Difficulty**: [Beginner | Intermediate | Advanced]
**Estimated Time**: [duration]
**Prerequisites**: [prior scenarios or skills]

## Background

[Narrative context -- who you are, why you are here, what you know]

## Objectives

1. [Specific, measurable objective]
2. [Another objective]

## Rules of Engagement

- [What you can and cannot do]
- [Operational constraints]

## Starting Conditions

- [What is running when you start]
- [What tools you have available]
- [What you know vs. what you must discover]

## Deliverables

- [What you must produce to complete the scenario]
```

### D3: Scenarios Reference Design Layer Environments

**Decision**: Each scenario references an environment template from the design layer. The environment defines the topology; the scenario defines the exercise.

**Examples**:
- Discovery scenario: references `greenfield-water-mfg` -- the standard reference environment with all devices active
- Attack scenario: references `greenfield-water-mfg-compromised` -- a variant environment with a pre-positioned attacker or modified device configuration
- Defense scenario: references `greenfield-water-mfg` with additional monitoring overlay attachment points defined
- Manufacturing-only scenario: references `manufacturing-only` -- an isolated subset of the reference environment for focused training

Creating scenario-specific environment variants is a design layer concern. The scenario directory contains only educational content.

### D4: Scenario Content Is Not Code

**Decision**: Scenario content (READMEs, hints, solutions) is documentation, not code. Scenarios do not contain Go source files, scripts, or tools.

**Rationale**: The point of a scenario is to define the problem. The engineer uses tools from `/monitoring/` (or external tools like Wireshark, nmap, Zeek) to solve it. If the scenario ships with pre-built tools, the learning value is lost.

**Exception**: A scenario may reference an environment template that configures the plant differently. Environment templates are configuration in the design layer, not code.

### D5: Initial Scenario Set

**Decision**: Four scenarios ship with the first release, one per learning phase. All four reference the `greenfield-water-mfg` environment template (the reference dual-network facility).

**Scenario 01: Network Discovery**
- **Phase**: Discovery
- **Environment**: `greenfield-water-mfg`
- **Objective**: Build a complete asset inventory of the manufacturing flat network
- **Challenge**: Serial devices behind the Moxa gateway are invisible to standard network scans. The cellular modem is undocumented.
- **Deliverable**: Asset inventory document with IP addresses, device types, protocols, and communication paths

**Scenario 02: Vulnerability Assessment**
- **Phase**: Assessment
- **Environment**: `greenfield-water-mfg`
- **Objective**: Identify and document all security vulnerabilities across both networks
- **Challenge**: Modbus has no authentication. The Moxa has default credentials. The cellular modem has port forwarding enabled. The cross-plant link has no firewall.
- **Deliverable**: Vulnerability report with risk ratings and remediation recommendations

**Scenario 03: Unauthorized Setpoint Change**
- **Phase**: Attack
- **Environment**: `greenfield-water-mfg`
- **Objective**: Demonstrate the impact of an unauthorized Modbus write to a water treatment PLC
- **Challenge**: Change the chlorine dosing setpoint to zero. Observe what happens to water quality over time. Determine whether any monitoring detects the change.
- **Deliverable**: Attack report documenting the steps, impact timeline, and detection gaps

**Scenario 04: Deploy Passive Monitoring**
- **Phase**: Defense
- **Environment**: `greenfield-water-mfg`
- **Objective**: Deploy a passive Modbus traffic monitor on both networks and establish a behavioral baseline
- **Challenge**: Water treatment side is straightforward (SPAN port). Manufacturing side has no SPAN capability. Clock drift complicates event correlation.
- **Deliverable**: Monitoring deployment plan, baseline traffic analysis, and anomaly detection rule set

### D6: Scenarios Can Compose Custom Environments

**Decision**: Advanced scenarios (beyond the initial four) may define custom environment templates in the design layer that extend or modify the reference environment. This enables scenario-specific conditions without polluting the base environment.

Examples of scenario-specific environments:
- `greenfield-water-mfg-compromised` -- includes a pre-positioned attacker container on the manufacturing flat network
- `greenfield-water-mfg-segmented` -- the manufacturing floor has been upgraded with a managed switch (the "after" state for a defense scenario)
- `power-substation-basic` -- an entirely different facility type for advanced scenarios (future)

This is the primary mechanism for scenario variety: compose the right environment, then write the educational content around it.

---

## Consequences

### Positive
- Structured learning path guides engineers from novice to practitioner
- Scenarios are reusable across different engineers and teams
- Configuration-driven environments mean the same plant binary serves all scenarios
- Hints and solutions support self-study without instructor presence
- Separating environment composition from scenario content enables both to evolve independently
- Custom environments unlock scenario variety without code changes

### Negative
- Scenario authoring requires OT security domain expertise
- Four initial scenarios provide limited breadth
- No automated scoring or progress tracking (manual success criteria)
- Adding the environment reference layer adds slight complexity compared to inline config

### Risks
- **Risk**: Scenarios may be too difficult for true beginners
- **Mitigation**: Include progressive hints. Scenario 01 (Discovery) starts with familiar IT tools (ping, nmap) before introducing OT-specific challenges.
- **Risk**: Attack scenarios could teach offensive techniques without sufficient defensive context
- **Mitigation**: Every attack scenario requires a corresponding defense scenario. Attack scenarios include "detection" sections that explain what monitoring should have caught.
- **Risk**: Scenario-specific environment variants may proliferate
- **Mitigation**: Prefer extending the reference environment over creating entirely new ones. Use environment composition (multiple network/device references) to vary conditions.

---

## References
- ADR-009: Design Layer and Composable Environments
- SANS ICS515: ICS Visibility, Detection, and Response (course structure reference)
- NIST Cybersecurity Framework: Identify, Protect, Detect, Respond, Recover
- MITRE ATT&CK for ICS: https://attack.mitre.org/techniques/ics/
