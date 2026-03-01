Inherits shared standards from ~/Development/CLAUDE.md

# OT Simulator Project Guidelines

## Project Overview

Simulated Operational Technology environment for training IT engineers on OT/ICS security challenges. Models a dual-network facility: a modern Purdue-compliant water treatment plant connected to a legacy flat-network manufacturing floor. The simulator produces realistic protocol traffic (Modbus TCP/RTU), exposes authentic register maps, and creates a training ground for deploying security overlays (Blastwave/BlastShield, Dragos) onto both modern and legacy OT infrastructure.

## Current State

- **Phase**: Foundation complete with design layer -- ready for first feature SOW
- **Last completed**: SOW-000.1 Design Layer Pivot (all 13 success criteria passing)
- **Stats**: 9 ADRs, 2 SOWs complete (000.0, 000.1)
- **Next work**: SOW-001.0 (Modbus TCP + generalized topology engine)
- **Blockers**: None

## Project Structure (Non-Standard)

This project deviates from the standard two-codebase pattern (`poc/` + `platform/`). The structure reflects the project's educational purpose and three-layer architecture (ADR-009):

| Directory | Purpose | Stability |
|-----------|---------|-----------|
| `poc/` | Throwaway validation code | Disposable |
| `design/` | Device library, network templates, environment definitions (pure YAML) | Evolving, schema-versioned |
| `plant/` | Runtime engine that instantiates any environment definition | Stable, production-quality |
| `monitoring/` | Security monitoring and detection tools | Fluid, experimental |
| `scenarios/` | Guided exercises referencing design layer environments | Curated, versioned |
| `docs/` | ADRs, SOWs, specs, project overview | Living documentation |

### Three-Layer Architecture

```
design/ --YAML--> plant/ --network--> monitoring/
  (what to build)   (running sim)     (observation)
```

- **Design** is pure configuration. No Go code. Plant owns all parsing and validation. Three atomic element types: devices, networks, environments.
- **Plant** reads design layer artifacts and instantiates them. It does not assume any specific facility type.
- **Monitoring** observes the running plant externally over the network only.

### Key Separation Principles

`/plant/` and `/monitoring/` are **independent Go modules** with separate `go.mod` files. Monitoring CANNOT import plant packages directly. It interacts over the network (Modbus TCP, packet capture, SPAN) exactly like real monitoring tools would. This boundary enforces realism.

`/design/` defines *what* to simulate. `/plant/` defines *how* to simulate it. A future Go module in design/ may provide guided YAML editing, but for now design/ is hand-edited YAML with a documented schema.

## Project-Specific Stack

Extends the shared Go stack with these project-specific technologies:

| Technology | Purpose | Notes |
|-----------|---------|-------|
| simonvetter/modbus | Modbus TCP/RTU client + server | MIT license, proposed (validate in POC first) |
| gopcua/opcua | OPC UA client + server | MIT license, future phase |
| gobacnet | BACnet/IP | MIT license, future phase |
| chi v5 | HTTP routing for HMI | Lightweight, stdlib-compatible |
| go:embed | Static asset embedding | Single binary deployment for HMI |
| Bootstrap 5 | HMI web framework | Via CDN, no build step |
| HTMX | Dynamic HMI updates | Via CDN, real-time register display |

**Forbidden (project-specific)**: OpenPLC (GPL, wrong abstraction), pymodbus (wrong language), Cobra/viper (per shared standards).

## Configuration

- **Environment variable prefix**: `OTS_` (OT Simulator)
- **Config location**: `design/` (topology), `monitoring/config/` (monitoring runtime)
- **Config format**: YAML only
- **Port assignments** (simulation defaults, non-privileged):

### Plant Ports
| Port | Service | Notes |
|------|---------|-------|
| 5020-5039 | Modbus TCP (virtual PLCs) | Maps to 502 in Docker |
| 8080 | Water treatment HMI | Modern operator interface |
| 8081 | Manufacturing HMI | Legacy operator interface |

### Monitoring Ports
| Port | Service | Notes |
|------|---------|-------|
| 8090 | Monitoring dashboard | Security analyst view |
| 8091 | Alert API | Anomaly detection alerts |

## Non-Goals

- Replacing real OT hardware or PLCs
- High-fidelity physics simulation (fluid dynamics, thermodynamics)
- Production SCADA deployment
- Real-time safety-critical control
- Competing with commercial OT simulators (GRFICSv3, OpenPLC)

## Key Principles

- **Protocol-First**: Wire-level protocol fidelity matters more than physics accuracy. IT engineers need to see real Modbus function codes, not accurate fluid dynamics.
- **Dual-Network Contrast**: Every feature should highlight the difference between modern (water treatment) and legacy (manufacturing) OT environments.
- **Separation of Concerns**: The plant runs independently. Monitoring observes externally. Engineers learn by working within real constraints.
- **Configuration-Driven**: Plant topology, device profiles, and scenarios defined in YAML. Code changes only for new capabilities.
- **Educational Intent**: Every design decision should make the simulator a better teaching tool, not a better engineering tool.

## SOW Workflow

All implementation work follows the SOW-first pattern:

1. **Draft**: Create/revise SOW in `docs/implementation/sows/` using the `/sow` skill
2. **OT Domain Review**: If the SOW includes design layer YAML deliverables (device atoms, network atoms, environment templates), run the `ot-domain-reviewer` agent (`.claude/agents/ot-domain-reviewer.md`) against the register map designs and device metadata BEFORE presenting for user approval. Incorporate all corrections into the SOW and tag them with `[OT-REVIEW]` for traceability.
3. **Review**: User reviews and requests changes if needed
4. **Approve**: User explicitly approves (e.g., "approved", "execute SOW-001.0", "looks good, proceed")
5. **Implement**: Launch the `sow-implementation-executor` agent to execute the approved SOW -- ALL approved SOWs are implemented via this agent, no exceptions
6. **Validate**: Agent validates against SOW success criteria before reporting completion

**Never implement without an approved SOW. Never implement an SOW without the agent.**
**Never submit a SOW with design layer YAML without an OT domain review.**

## Lessons and Anti-Patterns

- (To be populated as project progresses)

## REMEMBER

This is an EDUCATIONAL SIMULATOR. The audience is IT engineers learning OT security. Protocol fidelity and realistic operational constraints matter more than physics accuracy. The plant is the product. Monitoring is the learning journey. Keep them separate.
