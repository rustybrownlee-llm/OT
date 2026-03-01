# Milestone: Beta 0.1

**Target**: First functional POC -- an IT engineer can discover and interact with a simulated OT environment using standard tools.

## Definition of Done

An IT engineer can:

1. Run `docker compose up` and get a working OT environment
2. Use `nmap` to discover Modbus TCP endpoints on ports 5020-5030
3. Use a Modbus client to read register values from each device
4. Observe different response times per device (5ms CompactLogix vs 80ms Modicon)
5. Wait 30 seconds and see register values have drifted (process simulation)
6. Write to a writable coil (e.g., pump_run) and see related registers respond (flow increases)
7. Access serial devices (SLC-500, Modicon) through the Moxa gateway via unit ID routing
8. Follow Scenario 01 (Discovery) step-by-step and complete it successfully

## What Beta 0.1 Is NOT

- No web HMI (big scope, not needed to prove the concept)
- No monitoring module (the learning journey, comes after the plant works)
- No Modbus RTU/serial emulation (gateway abstracts this at TCP level)
- No OPC UA (Phase 3+ per ADR-002)
- No multiple environments (greenfield-water-mfg only)
- No Blastwave/Dragos integration (post-beta, once plant is solid)

## SOW Backlog

### Completed

| SOW | Title | Status | Date |
|-----|-------|--------|------|
| SOW-000.0 | Project Initialization | Complete | 2026-03-01 |
| SOW-000.1 | Design Layer Pivot | Complete | 2026-03-01 |
| SOW-001.0 | Full Device Atom Profiles | Complete | 2026-03-01 |

### Remaining (ordered by dependency)

| # | SOW | Title | Dependencies | Status | Notes |
|---|-----|-------|-------------|--------|-------|
| 1 | POC-001 | Modbus Library Validation | None | Not started | Validate simonvetter/modbus in poc/: TCP server, register serving, unit ID routing, configurable response delays. Quick throwaway -- proves the library works before committing to it. |
| 2 | SOW-002.0 | Modbus TCP Server | POC-001 | Not started | Plant binary starts Modbus TCP listeners for each Ethernet placement. Serves registers from design layer variants. Gateway routes by unit ID to serial device register maps. Per-device response delays. Static register values (no simulation yet). |
| 3 | SOW-003.0 | Process Simulation Engine | SOW-002.0 | Not started | Registers change over time. Sensor drift within scale ranges. Coil writes affect related holding registers (pump on -> flow increases, valve open -> pressure changes). Simple behaviors, not physics. Enough to make the environment feel alive. |
| 4 | SOW-004.0 | Scenario 01 Discovery Playbook | SOW-003.0 | Not started | Write step-by-step instructions for Scenario 01. Engineer discovers the network, identifies devices, reads registers, writes a coil, observes the effect. Expected outputs documented. Validation checklist. |
| 5 | SOW-005.0 | Docker Compose Integration | SOW-002.0 | Not started | Ensure `docker compose up` brings up a fully functional environment. Port mapping, health checks, log output, clean shutdown. May fold into SOW-002.0 if current setup is close enough. |

### Dependency Graph

```
POC-001 (Modbus library validation)
  |
  v
SOW-002.0 (Modbus TCP server) -----> SOW-005.0 (Docker Compose)
  |
  v
SOW-003.0 (Process simulation)
  |
  v
SOW-004.0 (Scenario 01 playbook)
  |
  v
[Beta 0.1 Complete]
```

## Technical Debt Inventory

Active debt items that should be resolved before or during beta 0.1:

| ID | Location | Description | Created By | Resolution |
|----|----------|-------------|-----------|------------|
| TD-002 | `plant/internal/config/config.go` | `findDesignRoot` uses relative path walking -- fragile | SOW-000.1 | Revisit when design tooling built |
| TD-004 | `plant/cmd/plant/main.go` | Default `--environment` path is relative, assumes binary runs from `plant/` | SOW-000.1 | Revisit when design tooling built |
| TD-005 | `design/devices/*.yaml` | All register maps use holding registers + coils only; input registers (FC04) and discrete inputs (FC02) not used | SOW-001.0 | When Modbus server is implemented |
| TD-006 | `design/devices/*.yaml` | No 32-bit float register pairs; cannot demonstrate Modicon word-swap issue | SOW-001.0 | Future SOW |

## Open Questions

- Should SOW-005.0 (Docker Compose) be a separate SOW or fold into SOW-002.0?
- Does Scenario 01 need a validation script (automated check) or just a written playbook?
- Should TD-005 (input registers) be addressed in SOW-002.0 when the Modbus server is built, or deferred further?

## Architecture Reference

- ADRs: `docs/architecture/decisions/` (ADR-001 through ADR-009)
- Design layer schema: ADR-009
- Device atoms: ADR-004
- Network architecture: ADR-003
- Protocol priority: ADR-002
