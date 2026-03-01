# OT Simulator - Project Overview

**Project**: OT Simulator
**Purpose**: Simulated OT/ICS environment for training IT engineers on operational technology security
**Status**: Pre-implementation -- ADRs proposed, SOW-000.0 drafted
**Updated**: 2026-02-28

## Vision

A software-based simulation of a dual-network industrial facility -- a modern Purdue-compliant water treatment plant connected to a legacy flat-network manufacturing floor. The simulator produces realistic industrial protocol traffic, exposes authentic register maps, and creates a training ground where IT engineers learn OT security by doing: discovering devices, assessing vulnerabilities, observing attacks, and deploying defenses.

The long-term goal is a platform where engineers can practice deploying commercial OT security tools (Blastwave/BlastShield for zero trust access control, Dragos for passive visibility and threat detection) onto both modern and legacy OT infrastructure, experiencing the different challenges each environment presents.

### Core Use Case

**IT Engineer OT Security Training**:
- Discover and inventory OT devices across modern and legacy networks
- Understand industrial protocols (Modbus TCP/RTU) at the wire level
- Experience the operational constraints of airgapped, 30+ year old environments
- Practice deploying security monitoring with realistic infrastructure limitations
- Compare modern segmented architecture against legacy flat network reality

### Key Differentiators

1. **Dual-Network Contrast**: Side-by-side modern Purdue model and legacy flat network teach by comparison
2. **Protocol-First Fidelity**: Wire-level Modbus authenticity over physics accuracy
3. **Separation of Concerns**: Plant runs independently; monitoring observes externally, just like reality
4. **Configuration-Driven**: Plant topology, device profiles, and scenarios defined in YAML
5. **COTS Integration Ready**: Designed for Blastwave/BlastShield and Dragos deployment from the start
6. **Scenario-Based Learning**: Structured exercises progress from discovery through defense
7. **Legacy Realism**: Serial devices, clock drift, no centralized logging, accidental internet bridges
8. **Go-Native**: Entire stack in Go, matching team production standards

## Technology Approach

### Simulated Physical Process

The water treatment plant models a simplified 6-stage treatment process: raw water intake, chemical dosing, filtration, UV treatment, chlorination, and distribution. Physics are plausible but simplified -- cause-and-effect relationships are accurate (pump on = flow increases, chlorine decays over time) but fluid dynamics are not modeled.

The manufacturing floor models process equipment consuming treated water: cooling systems, wash stations, and mixing tanks. Equipment runs on legacy PLCs with serial communication.

### Protocol Simulation

All virtual PLCs expose realistic Modbus register maps:
- Coils (FC01/FC05): pump commands, valve states, e-stop
- Discrete inputs (FC02): fault contacts, float switches
- Holding registers (FC03/FC06): process values, setpoints, alarm words
- 16-bit scaled integers (0-32767 mapped to engineering range) matching real SCADA conventions
- 1-second polling intervals for process variables, 5-second for water quality

### Device Profiles

Virtual PLCs are configured with device profiles that modify response characteristics:
- CompactLogix L33ER: Ethernet native, 5ms response, web diagnostics
- SLC-500/05: Serial only, 25ms response, limited registers
- Modicon 984: Serial only, 50ms response, original Modbus device
- Moxa NPort 5150: Serial-to-Ethernet gateway, adds latency and serialization

### Network Architecture

Two distinct network topologies run side by side:
- **Water Treatment**: Purdue model with VLANs per level, managed switches, SPAN ports, firewalls between levels
- **Manufacturing**: Single flat 192.168.1.0/24, unmanaged switch, no segmentation, serial devices behind a Moxa gateway, undocumented cellular modem

Docker Compose models the network segmentation with separate Docker networks per zone.

## Technology Stack

**Approved**:
- Go 1.24+, standard library preferred
- simonvetter/modbus (MIT) -- Modbus TCP/RTU client + server (pending POC validation)
- gopkg.in/yaml.v3 -- configuration
- chi v5 -- HTTP routing for HMI
- slog -- structured JSON logging
- Bootstrap 5 + HTMX -- HMI web interface
- Docker and Docker Compose -- deployment and network topology

**Forbidden** (per shared standards):
- Cobra, viper, gin, echo, gorm, logrus, zap (see ~/Development/CLAUDE.md)
- OpenPLC (GPL, wrong abstraction)
- pymodbus (wrong language)

## Project Structure

```
OT/
  poc/              Throwaway validation code
  plant/            Simulated OT environment (stable, production-quality)
    cmd/plant/      Entry point
    pkg/            Process models, PLC engine, protocol adapters, HMI, device profiles
    config/         YAML plant topology and device profiles
  monitoring/       Security monitoring tools (experimental, fluid)
    cmd/            Monitoring tool entry points
    pkg/            Monitoring libraries
  scenarios/        Guided exercises for engineers
    01-discovery/
    02-vulnerability-assessment/
    03-attack-simulation/
    04-security-overlay/
  docs/             ADRs, SOWs, specs, project overview
```

## Architectural Decisions

| ADR | Title | Status |
|-----|-------|--------|
| ADR-001 | Simulation Fidelity Philosophy | Proposed |
| ADR-002 | Protocol Priority and Legacy Representation | Proposed |
| ADR-003 | Dual-Network Architecture | Proposed |
| ADR-004 | Device Profiles and Vendor Simulation | Proposed |
| ADR-005 | Monitoring Integration Architecture | Proposed |
| ADR-006 | Airgap and Accidental Bridge Modeling | Proposed |
| ADR-007 | Educational Scenario Framework | Proposed |
| ADR-008 | Safety System Modeling | Proposed |

## Implementation Roadmap

### Phase 1: Foundation -- PENDING
- SOW-000.0: Project initialization (Go modules, build tooling, Docker Compose skeleton)
- POC-001: Modbus library validation (simonvetter/modbus server capabilities)

### Phase 2: Water Treatment PLCs -- PLANNED
- SOW-001.0: Water treatment Level 1 (virtual PLCs with Modbus TCP)
- SOW-002.0: Water treatment Level 0 (process simulation engine)

### Phase 3: HMI and Visualization -- PLANNED
- SOW-003.0: Water treatment Level 2 (web-based HMI)

### Phase 4: Manufacturing Floor -- PLANNED
- SOW-004.0: Manufacturing flat network (legacy PLCs, serial simulation)
- SOW-005.0: Cross-plant connection

### Phase 5: Full Environment -- PLANNED
- SOW-006.0: Docker Compose full topology
- SOW-007.0: Initial scenario content

### Phase 6: Security Overlays -- DESIGNED
- Blastwave/BlastShield integration
- Dragos integration points
- Advanced attack scenarios
- Safety system modeling

## SOW Registry

| SOW | Description | Status | Dependencies |
|-----|-------------|--------|-------------|
| SOW-000.0 | Project Initialization | Draft | None |

## Development Workflow

All development follows SOW-driven methodology:
1. Create SOW document with scope, deliverables, success criteria
2. Present SOW for review and approval
3. Implement exactly as specified using the SOW implementation executor agent
4. Validate against success criteria
5. Track technical debt in the SOW that creates it

## Code Quality Standards

Per ~/Development/CLAUDE.md:
- Functions: 60 lines maximum
- Files: 500 lines maximum (excluding tests)
- Packages: 10 public functions maximum
- Interfaces: 5 methods maximum
- No emojis, professional language only
- Table-driven tests, standard testing package
