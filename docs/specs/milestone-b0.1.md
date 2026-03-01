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
| POC-001 | Modbus Library Validation | Complete | 2026-03-01 |
| SOW-002.0 | Modbus TCP Server | Complete | 2026-03-01 |
| SOW-003.0 | Process Simulation Engine | Complete | 2026-03-01 |
| SOW-005.0 | Docker Compose Integration | Complete | 2026-03-01 |
| SOW-004.0 | Scenario 01 Discovery Playbook | Complete | 2026-03-01 |

### Remaining (ordered by dependency)

None. All Beta 0.1 SOWs are complete.

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
| TD-005 | `design/devices/*.yaml` | All register maps use holding registers + coils only; input registers (FC04) and discrete inputs (FC02) not used | SOW-001.0 | Post-Beta 0.1 |
| TD-006 | `design/devices/*.yaml` | No 32-bit float register pairs; cannot demonstrate Modicon word-swap issue | SOW-001.0 | Post-Beta 0.1 |
| TD-013 | `plant/internal/process/water.go` | Cross-variant coupling not implemented (e.g., intake output does not feed treatment input) | SOW-003.0 | Future SOW |
| TD-014 | `plant/internal/process/engine.go` | Tick rate hardcoded to 1 second | SOW-003.0 | When configurable tick rate needed |
| TD-015 | `plant/internal/process/manufacturing.go` | Jam/e-stop probabilities hardcoded (0.1%, 0.05%) | SOW-003.0 | When probabilities need tuning |
| TD-016 | `scenarios/01-discovery/` | Scenario references specific tool versions (nmap, mbpoll). Tool syntax may change. | SOW-004.0 | Review when tools update |
| TD-017 | `scenarios/01-discovery/solutions/` | Solution shows register values dependent on init values and simulation timing | SOW-004.0 | Update when process models are tuned |
| TD-011 | `monitoring/config/monitor.yaml` | Config file is a stub -- monitor binary does not actually consume it yet | SOW-005.0 | When monitoring module is implemented |
| TD-012 | `docker-compose.yml` | Monitor health check only verifies config is readable, not that monitoring is functional | SOW-005.0 | When monitoring module has real endpoints |
| TD-018 | `docker-compose.yml` | cross-plant network assigns only 172.16.0.2; 172.16.0.3 (mfg-gateway-01) is unroutable | SOW-005.0 | When multi-container architecture is considered |
| TD-019 | `docker-compose.yml` | mfg-serial-bus (RS-485) absent from Docker Compose; RS-485 cannot be modeled as bridge network | SOW-005.0 | Documentation only |

## Open Questions

- TD-005 (input registers) deferred past Beta 0.1 -- all current scenarios use holding registers + coils only
- Scenario 01 uses manual validation (per ADR-007 D4: content, not code). Automated scoring deferred.

## Architecture Reference

- ADRs: `docs/architecture/decisions/` (ADR-001 through ADR-009)
- Design layer schema: ADR-009
- Device atoms: ADR-004
- Network architecture: ADR-003
- Protocol priority: ADR-002
