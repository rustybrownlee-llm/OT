# Milestone: Beta 0.2

**Target**: Prove design layer composability -- a second OT environment can be authored in YAML and run without plant code changes. Validation tooling catches errors before runtime.

## Definition of Done

An engineer can:

1. Run `ot-design validate design/environments/pipeline-monitoring/` and get a clear pass/fail with actionable error messages
2. Run `docker compose --profile pipeline up` and get a working pipeline SCADA environment on different ports than water/mfg
3. Use a Modbus client to read registers from pipeline RTUs and flow computers
4. Observe different device characteristics (response times, register layouts, addressing modes) between the two environments
5. Switch between environments using Docker Compose profiles -- both work independently
6. Create a minimal custom environment YAML by hand (3 devices, 1 network) and have it pass validation and run
7. Run `ot-design scaffold --device` to generate a skeleton device atom YAML with all required fields

## What Beta 0.2 Is NOT

- No web UI for environment authoring (CLI and YAML are sufficient for this milestone)
- No monitoring module (still deferred -- plant composability comes first)
- No new protocols (pipeline environment uses Modbus TCP, same as Beta 0.1)
- No cross-environment scenarios (each environment is independent)
- No schema v0.2 features (security profiles, monitoring overlays -- deferred)
- No HMI for either environment

## Second Environment: Pipeline Monitoring Station

A natural gas pipeline monitoring station. Chosen because:

- **Maximally different** from water/mfg: remote SCADA site, star topology with WAN backhaul, no Purdue levels
- **Modbus TCP native**: RTUs and flow computers commonly speak Modbus TCP -- no new protocol needed
- **High educational value**: Pipeline security is a critical concern (Colonial Pipeline incident). Remote sites have different threat models than local facilities.
- **Introduces new device types**: RTUs (designed for harsh/remote environments, different from PLCs), flow computers (specialized measurement devices)
- **Reuses existing device atom**: CompactLogix for local compressor control -- proves same device atom works across industries

### Facility Description

A single compressor station on a natural gas transmission pipeline. The station boosts pipeline pressure using gas turbine-driven compressors. A remote SCADA master (not simulated) polls the station over a WAN link.

**Devices (5 placements)**:

| Placement | Device | Role | Notes |
|-----------|--------|------|-------|
| ps-plc-01 | CompactLogix L33ER | Compressor control | REUSE existing device atom, new `compressor-control` variant |
| ps-rtu-01 | Emerson ROC800 | Pipeline metering RTU | NEW device atom. Custody-transfer flow measurement, 4 meter runs |
| ps-rtu-02 | Emerson ROC800 | Station inlet/outlet monitoring | Same device, different variant |
| ps-fc-01 | ABB TotalFlow G5 | Gas chromatograph interface | NEW device atom. Gas composition analysis, BTU calculation |
| ps-gw-01 | Moxa NPort 5150 | Serial gateway for legacy analyzers | REUSE existing device atom, new `pipeline-serial` variant |

**Networks (3)**:

| Network | Type | Topology | Notes |
|---------|------|----------|-------|
| ps-station-lan | Ethernet | Flat (single subnet) | All station devices on one network, like a small industrial site |
| ps-serial-bus | RS-485 | Multidrop | Legacy analyzers behind the Moxa gateway |
| ps-wan-link | Ethernet (WAN) | Point-to-point | Simulates satellite/cellular backhaul to SCADA master. Higher latency. |

### Key Differences from Water/Mfg

| Aspect | Greenfield Water/Mfg | Pipeline Station |
|--------|----------------------|------------------|
| Network model | Purdue (3 levels) + flat legacy | Flat station LAN + WAN backhaul |
| Device types | PLCs + serial gateway | PLCs + RTUs + flow computer + serial gateway |
| Geography | Single building, local | Remote site, wide-area |
| Addressing | Mix of zero-based and one-based | Mix (CompactLogix zero, ROC800 one-based) |
| Security posture | Internal threats, lateral movement | Remote access threats, WAN exposure |
| Register focus | Process control (pumps, valves, chemistry) | Measurement/custody transfer (flow rates, gas composition, pressure) |

## SOW Backlog

### Ordered by Dependency

| # | SOW | Title | Dependencies | Status | Notes |
|---|-----|-------|-------------|--------|-------|
| 1 | SOW-006.0 | Design Layer Validation CLI | None | Complete | `ot-design validate` command. Schema validation, cross-reference checking, actionable error messages. Also `ot-design scaffold` for generating skeleton YAMLs. |
| 2 | SOW-007.0 | Pipeline Device Atoms | None | Complete | 2 new device atoms (ROC800, TotalFlow G5) with register map variants. New CompactLogix variant (`compressor-control`). New Moxa variant (`pipeline-serial`). |
| 3 | SOW-008.0 | Pipeline Environment Definition | SOW-007.0 | Complete | 3 network atoms, environment.yaml with 5 placements, port assignments (5040-5049 range). |
| 4 | SOW-009.0 | Pipeline Process Models | SOW-008.0 | Complete | Process simulation for compressor, metering, gas analysis. Registers drift and respond to writes. 5 technical debt items (TD-016 through TD-020). |
| 5 | SOW-010.0 | Multi-Environment Docker Compose | SOW-008.0 | Complete | Docker Compose profiles for environment selection. `--profile water` vs `--profile pipeline`. Shared network infrastructure where needed. |
| 6 | SOW-011.0 | Environment Authoring Guide | SOW-006.0, SOW-008.0 | Complete | Documentation: how to create a new environment from scratch. Step-by-step guide with the pipeline station as a worked example. Includes quickstart environment (3 PLCs, 1 network) that passes validation. |

### Dependency Graph

```
SOW-006.0 (Validation CLI) ---------> SOW-011.0 (Authoring Guide)
                                          ^
SOW-007.0 (Pipeline device atoms)         |
  |                                       |
  v                                       |
SOW-008.0 (Pipeline environment) -----> SOW-011.0
  |         \
  v          v
SOW-009.0   SOW-010.0
(Process)   (Multi-env Docker)
  |          |
  v          v
[Beta 0.2 Complete]
```

## Design Layer Validation Tool

The validation CLI is the cornerstone of this milestone. Without it, authoring environments is fragile guesswork.

### Validation Rules

**Device atom validation (`ot-design validate design/devices/foo.yaml`)**:
- All required fields present (id, manufacturer, model, protocol, addressing)
- Register addresses are contiguous and within device limits (max_holding_registers, max_coils)
- Scale ranges are valid (scale_min < scale_max)
- Init values reference valid strategies (zero, midpoint, max, explicit number)
- Variant register names are unique within a variant
- Units field is non-empty for holding registers

**Network atom validation (`ot-design validate design/networks/bar.yaml`)**:
- Required fields present (id, type, media)
- Subnet is valid CIDR (for Ethernet types)
- Serial networks have no subnet field
- SPAN capability is boolean

**Environment validation (`ot-design validate design/environments/baz/`)**:
- All referenced devices exist in `design/devices/`
- All referenced networks exist in `design/networks/`
- All referenced register_map_variants exist in the device's variant list
- All placement IDs are unique
- IP addresses are within their network's subnet
- No port collisions across placements
- Serial devices reference a valid gateway placement
- Gateway placements have a `bridges` field connecting networks
- Serial addresses are unique per gateway

### Scaffold Commands

```
ot-design scaffold --device              # Generate skeleton device atom
ot-design scaffold --network             # Generate skeleton network atom
ot-design scaffold --environment         # Generate skeleton environment directory
```

## Port Assignments

Pipeline station uses a separate port range to avoid collisions:

| Port | Service | Notes |
|------|---------|-------|
| 5020-5022 | Water treatment PLCs | Existing (Beta 0.1) |
| 5030 | Manufacturing gateway | Existing (Beta 0.1) |
| 5040-5049 | Pipeline station devices | New (Beta 0.2) |

## Technical Debt to Address

Items from Beta 0.1 that this milestone should resolve:

| ID | Description | Resolution in Beta 0.2 |
|----|-------------|------------------------|
| TD-002 | `findDesignRoot` uses relative path walking | Validation CLI provides canonical path resolution |
| TD-004 | Default `--environment` path is relative | Multi-env Docker Compose makes this moot (path is in compose config) |

## Open Questions

- Should the validation CLI be a standalone binary (`ot-design`) or a subcommand of the plant binary (`plant validate`)?
- Does the pipeline station need a WAN latency simulation (e.g., 200ms+ RTT to SCADA master), or is that a future feature?
- Should SOW-011.0 (authoring guide) be a markdown document or an interactive CLI walkthrough?
- Is the ROC800 the right RTU choice, or would a more common device (e.g., Schneider Electric TRIO) be better for educational purposes?

## Architecture Reference

- ADRs: `docs/architecture/decisions/` (ADR-001 through ADR-009)
- Design layer schema: ADR-009
- Device atoms: ADR-004
- Network architecture: ADR-003
- Airgap and bridges: ADR-006
- Protocol priority: ADR-002
