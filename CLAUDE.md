Inherits shared standards from ~/Development/CLAUDE.md

# OT Simulator Project Guidelines

## Project Overview

Simulated Operational Technology environment for training IT engineers on OT/ICS security challenges. Models composable OT facilities from atomic device and network elements. The simulator produces realistic protocol traffic (Modbus TCP/RTU), exposes authentic register maps, and creates a training ground for deploying security overlays (Blastwave/BlastShield, Dragos) onto both modern and legacy OT infrastructure.

## Project Status

**Status lives in milestone specs, not here.** See `docs/specs/` for current milestone definitions, SOW backlogs, and progress. This file contains stable instructions only.

## Project Structure

Three-layer architecture per ADR-009:

```
design/ --YAML--> plant/ --network--> monitoring/
  (what to build)   (running sim)     (observation)
```

| Directory | Purpose | Stability |
|-----------|---------|-----------|
| `poc/` | Throwaway validation code | Disposable |
| `design/` | Device library, network atoms, environment definitions (pure YAML) | Evolving, schema-versioned |
| `plant/` | Runtime engine that instantiates any environment definition | Stable, production-quality |
| `monitoring/` | Security monitoring and detection tools | Fluid, experimental |
| `scenarios/` | Guided exercises referencing design layer environments | Curated, versioned |
| `docs/` | ADRs (`architecture/decisions/`), SOWs (`implementation/sows/`), specs (`specs/`) | Living documentation |

### Key Separation Principles

- `plant/` and `monitoring/` are **independent Go modules** with separate `go.mod` files. Monitoring CANNOT import plant packages. It interacts over the network only.
- `design/` defines *what* to simulate. `plant/` defines *how* to simulate it. Design is pure YAML; plant owns all parsing.

## Technology Stack

Extends the shared Go stack with these project-specific technologies:

| Technology | Purpose | Notes |
|-----------|---------|-------|
| simonvetter/modbus | Modbus TCP/RTU client + server | MIT license, validate in POC first |
| gopcua/opcua | OPC UA client + server | MIT license, future phase |
| gobacnet | BACnet/IP | MIT license, future phase |
| chi v5 | HTTP routing for HMI | Lightweight, stdlib-compatible |
| go:embed | Static asset embedding | Single binary deployment for HMI |
| Bootstrap 5 | HMI web framework | Via CDN, no build step |
| HTMX | Dynamic HMI updates | Via CDN, real-time register display |

**Forbidden (project-specific)**: OpenPLC (GPL, wrong abstraction), pymodbus (wrong language), Cobra/viper (per shared standards).

## Configuration

- **Environment variable prefix**: `OTS_`
- **Config location**: `design/` (topology), `monitoring/config/` (monitoring runtime)
- **Config format**: YAML only

### Port Assignments

| Port | Service | Notes |
|------|---------|-------|
| 5020-5039 | Modbus TCP (virtual PLCs) | Maps to 502 in Docker |
| 8080 | Water treatment HMI | Modern operator interface |
| 8081 | Manufacturing HMI | Legacy operator interface |
| 8090 | Monitoring dashboard | Security analyst view |
| 8091 | Alert API | Anomaly detection alerts |

## SOW Workflow

All implementation work follows the SOW-first pattern:

1. **Draft**: Create/revise SOW in `docs/implementation/sows/` using the `/sow` skill
2. **OT Domain Review**: If the SOW includes design layer YAML deliverables, run the `ot-domain-reviewer` agent before presenting for user approval. Tag corrections with `[OT-REVIEW]`.
3. **Review**: User reviews and requests changes
4. **Approve**: User explicitly approves (e.g., "approved", "execute SOW-001.0")
5. **Implement**: Launch the `sow-implementation-executor` agent -- ALL approved SOWs are implemented via this agent, no exceptions
6. **Validate**: Agent validates against SOW success criteria
7. **Update milestone**: Update the active milestone spec in `docs/specs/` with completion status

**Never implement without an approved SOW. Never implement an SOW without the agent.**
**Never submit a SOW with design layer YAML without an OT domain review.**
**SOW drafting and SOW implementation are separate sessions.** Do not draft and implement in the same context window -- context compaction increases hallucination risk.

## Key Principles

- **Protocol-First**: Wire-level protocol fidelity matters more than physics accuracy.
- **Dual-Network Contrast**: Highlight difference between modern (Purdue) and legacy (flat) OT environments.
- **Separation of Concerns**: Plant runs independently. Monitoring observes externally.
- **Configuration-Driven**: Topology and device profiles in YAML. Code changes only for new capabilities.
- **Educational Intent**: Every design decision should make this a better teaching tool, not a better engineering tool.

## Non-Goals

- Replacing real OT hardware or PLCs
- High-fidelity physics simulation
- Production SCADA deployment
- Real-time safety-critical control
- Competing with commercial OT simulators

## Lessons and Anti-Patterns

- OT Domain Reviewer agent caught 12 issues in SOW-001.0 that would have taught incorrect device behavior. Always run the review before approval on design layer changes.
- Implementation agent leaves stray compiled binaries (`plant/plant`, `monitoring/monitor`). These are in `.gitignore` but check before committing.

## REMEMBER

This is an EDUCATIONAL SIMULATOR. The audience is IT engineers learning OT security. Protocol fidelity and realistic operational constraints matter more than physics accuracy. The plant is the product. Monitoring is the learning journey. Keep them separate.
