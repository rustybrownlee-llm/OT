# Milestone: Beta 0.4

**Target**: Prove the design layer can model real-world OT infrastructure evolution by composing a Frankenstein hybrid environment from atomic YAML. Deliver an adaptive topology visualization that teaches architecture through visual shape, not labels. Close the gap between textbook OT training and the messy reality IT engineers encounter on day one.

**Implements**: ADR-010 (Environment Archetypes and Adaptive Visualization)

## Definition of Done

An engineer can:

1. Run `docker compose --profile water --profile pipeline --profile wastewater up` and have all three environments start with independent monitoring
2. Open `http://localhost:8090` and see three environments in the asset inventory, each labeled with its archetype (`modern-segmented`, `legacy-flat`, `hybrid`)
3. Click into the wastewater environment topology view and see a collapsed Purdue stack with dashed boundaries where segmentation is incomplete, solid boundaries where it was implemented, and no boundary where none exists
4. Compare the wastewater topology (hybrid) side-by-side with water treatment (segmented) and pipeline (flat) -- the visual shape of each is immediately distinct
5. See era markers on every device in the wastewater environment, showing installation dates spanning 1997-2022
6. Click an era marker and understand the facility's evolution: what was added when, and why the architecture looks the way it does
7. Run `ot-design validate design/environments/brownfield-wastewater/` and get a clean pass -- the entire hybrid environment is expressed in atomic YAML with zero plant code changes
8. Browse the Design Library to see the new device atoms alongside existing ones, confirming the library grows organically
9. View the brownfield wastewater environment definition in the Design Library and see placements grouped by installation era
10. Observe the monitor discovering wastewater devices and establishing baselines, same as water and pipeline environments -- proving the monitoring module is truly environment-agnostic
11. Write a rogue value to a wastewater PLC register and see anomaly detection fire, confirming monitoring works on the hybrid environment without special handling

## What Beta 0.4 Is NOT

- No attack simulation scenarios (Scenario 03 deferred to Beta 0.5 -- Beta 0.4 focuses on architecture visualization, not offensive exercises)
- No defense deployment exercises (Scenario 04 deferred to Beta 0.5)
- No construction/deconstruction interactive mode (ADR-010 D5 describes the concept; Beta 0.4 delivers the static hybrid environment and era markers; interactive what-if analysis is Beta 0.5+)
- No Blastwave/BlastShield integration (insertion points designed but not activated)
- No Dragos integration
- No new OT protocols (Modbus TCP only, consistent with prior milestones)
- No historical data persistence (in-memory ring buffers, consistent with Beta 0.3)
- No cross-environment correlation (each environment monitored independently)
- No environment timeline animation (showing the facility at different eras is a future capability)

## Brownfield Wastewater Treatment Facility

### Why Wastewater

The Frankenstein hybrid environment must satisfy the criteria in ADR-010 D6. Wastewater treatment was selected because:

- **Narrative connection**: Sister facility to the existing greenfield water treatment plant, but 15 years older with ad-hoc modernization. The contrast is natural and immediate -- same industry, different eras.
- **Maximum atom reuse**: SLC-500, CompactLogix, and Moxa gateway atoms already exist. Only 1-2 new device atoms needed.
- **Realistic hybrid characteristics**: Original serial backbone (1997) with Ethernet bridge (2008), partial VLAN segmentation after a compliance audit (2018), and a cellular gateway for vendor remote access (2022). Every element has a real-world counterpart.
- **Distinct teaching value**: The greenfield water plant teaches "what good looks like." The wastewater plant teaches "what you'll actually find." Same process domain, different architectural reality.

### Facility History (drives era markers)

| Era | Year | What Happened | Atoms Added |
|-----|------|---------------|-------------|
| Original build | 1997 | Municipal wastewater treatment plant constructed. SLC-500 PLCs on DH-485 serial bus. Simple HMI on same flat Ethernet segment. Unmanaged switch. | SLC-500 (x2-3), serial bus, flat Ethernet, unmanaged switch |
| Ethernet bridge | 2008 | Serial-to-Ethernet gateway added for remote monitoring from city hall. No network redesign. | Moxa NPort gateway |
| Partial modernization | 2013 | New aeration blower system with modern PLC. Connected to existing flat Ethernet. No segmentation. | CompactLogix (Ethernet-native, on flat network) |
| Audit response | 2018 | State compliance audit. Added managed switch. Created VLAN for SCADA server (Level 3). Level 1 and Level 2 remain flat. Segmentation project stalled at ~40%. | Managed switch, VLAN network atom (Level 3 only) |
| Vendor remote access | 2022 | Blower vendor requires remote access for predictive maintenance. Cradlepoint cellular gateway added to flat network by vendor technician. "Temporary." Still there. | Cradlepoint cellular modem |

### Network Architecture (Hybrid)

```
Level 3  ┌──────────────────────────────────┐
(2018)   │  [SCADA Server]                  │
         │  VLAN 30, 10.30.30.0/24          │
         │  Managed switch (2018)            │
         └───────────────┬──────────────────┘
                         │ enforced boundary (managed switch + VLAN)
- - - - - - - - - - - - -│- - - - - - - - - - - -  ← Level 2
                         │                            boundary
                         │ (no L2 infrastructure)     ABSENT
                         │
Level 1  ┌───────────────┴──────────────────────────────────────────┐
(1997+)  │ FLAT SEGMENT: 192.168.10.0/24, unmanaged switch (1997)  │
         │                                                          │
         │  [CompactLogix]   [Moxa GW]══serial══[SLC-500] [SLC-500]│
         │   (2013)           (2008)     (1997)   (1997)    (1997)  │
         │   Ethernet-native                DH-485 bus              │
         │                                                          │
         │  [Cradlepoint]                                           │
         │   (2022)                                                 │
         │   Cellular WAN ──── vendor cloud                         │
         └──────────────────────────────────────────────────────────┘
```

### Key Teaching Points

1. **Partial segmentation is common**: Level 3 got a VLAN after the audit. Everything else stayed flat. The audit response was real but incomplete -- a story every OT security assessor will recognize.
2. **Era mixing is normal**: A 1997 SLC-500 shares a network with a 2022 cellular gateway. Neither was designed for the other's threat model.
3. **Unauthorized additions accumulate**: The Cradlepoint was "temporary" vendor access. Four years later, it's still there, providing an internet-connected backdoor to the flat OT network.
4. **Serial backbone persists**: The DH-485 bus still carries process data. It can't be replaced without a full plant shutdown. The Moxa gateway bridges it to Ethernet but doesn't add any security.
5. **The managed switch is both progress and false confidence**: Level 3 segmentation exists, but an attacker on the flat Level 1 segment can reach every device except the SCADA server. The audit checkbox was checked, but the actual security improvement is minimal.

### Educational Contrast (All Three Archetypes)

| Aspect | Water Treatment | Pipeline Station | Wastewater (NEW) |
|--------|----------------|-----------------|------------------|
| Archetype | Modern segmented | Legacy flat | Frankenstein hybrid |
| Era span | 2024 (single build) | 2015 (single build) | 1997-2022 (25 years) |
| Purdue compliance | Full (L1-L3) | None | Partial (L3 only) |
| Segmentation | Managed switches, VLANs | None | One managed switch (2018), rest flat |
| Serial presence | Via gateway (legacy mfg side) | One serial link | Original serial backbone still active |
| Internet exposure | None | None | Cellular gateway (vendor "temporary") |
| Monitoring blind spots | Minimal (SPAN on all levels) | Unmanaged switch (no SPAN) | Mixed: SPAN on L3 switch, none on L1 flat segment |
| Attack surface | Smallest (segmented, no internet) | Medium (flat but air-gapped) | Largest (flat + internet + partial seg gives false confidence) |

## Adaptive Topology Visualization

### Architecture

The monitoring dashboard topology view is a distinct UI component that renders environment architecture based on metadata, not hardcoded layout. The component reads:

1. **Environment archetype** (`modern-segmented`, `legacy-flat`, `hybrid`) -- determines layout algorithm
2. **Network atom properties** -- managed/unmanaged, VLAN, SPAN capability
3. **Placement data** -- device positions on networks, IP addressing, gateway bridges
4. **Boundary states** -- `enforced`, `intended`, `absent` between logical levels
5. **Era markers** -- `installed` year on each placement

### Rendering Rules

| Archetype | Layout | Boundaries | Devices |
|-----------|--------|------------|---------|
| `modern-segmented` | Vertical stack, one row per Purdue level | All solid lines | Grouped by level, uniform era |
| `legacy-flat` | Single horizontal plane | No boundaries shown | Clustered by physical proximity (gateway as focal point) |
| `hybrid` | Partially collapsed stack | Mix of solid/dashed/absent | Era markers visible, grouped by level where levels exist |

### Technology

- Server-side rendered HTML (consistent with existing dashboard approach: chi + Go templates + HTMX)
- Topology layout computed in Go from environment YAML metadata
- CSS-driven visual styling (solid/dashed/absent borders, era marker badges)
- HTMX for switching between environments without full page reload
- No JavaScript visualization library (D3, vis.js, etc.) -- keep the zero-JS-build-step constraint

### Routes

| Route | Content |
|-------|---------|
| `/topology` | Environment selector + topology view for selected environment |
| `/topology/{env-id}` | Topology view for a specific environment |

The topology view is a new page in the monitoring dashboard, alongside the existing asset inventory, device detail, alert timeline, and design library pages.

## Schema Extensions

### Non-Breaking Additions to v0.1

These fields are added as optional extensions. Existing environments continue to validate without them.

**Environment metadata**:

```yaml
environment:
  id: "brownfield-wastewater"
  name: "Brownfield Wastewater Treatment Plant"
  archetype: "hybrid"              # NEW: modern-segmented | legacy-flat | hybrid
  era_span: "1997-2022"            # NEW: oldest to newest installation date
  description: "..."
```

**Placement era marker**:

```yaml
placements:
  - id: "ww-plc-01"
    device: "slc-500-05"
    installed: 1997                 # NEW: year this device was placed in this facility
    # ... rest of placement unchanged
```

**Boundary states**:

```yaml
boundaries:                         # NEW: optional section in environment template
  - between: ["ww-level3", "ww-flat"]
    state: "enforced"
    infrastructure: "managed-switch"
    installed: 2018
    notes: "Added after state compliance audit"

  - between: ["ww-level2", "ww-flat"]
    state: "absent"
    notes: "Level 2 was never implemented -- HMI sits on flat segment"
```

### Validation Updates

The `ot-design validate` CLI must be extended to:

- Accept and validate `archetype` values (enum: `modern-segmented`, `legacy-flat`, `hybrid`)
- Accept and validate `installed` year on placements (optional, must be >= 1970 and <= current year)
- Accept and validate `boundaries` section (optional, `between` must reference networks in the environment, `state` must be enum)
- Warn if a `hybrid` archetype environment has no `boundaries` section defined
- Warn if a `modern-segmented` environment has `absent` boundaries

## SOW Backlog

### Ordered by Dependency

| # | SOW | Title | Dependencies | Status | Notes |
|---|-----|-------|-------------|--------|-------|
| 1 | SOW-016.0 | Environment Archetype Schema Extensions | None | Complete | Add archetype, era_span, installed, boundaries to schema. Update validation CLI. Retroactively tag existing environments. |
| 2 | SOW-017.0 | Brownfield Wastewater Device and Network Atoms | SOW-016.0 | Complete | 4 network atoms (ww-flat, ww-serial-bus, ww-level3, ww-wan), 2 SLC-500 variants (ww-influent, ww-effluent), 1 CompactLogix variant (ww-aeration), 1 Moxa variant (ww-serial-gateway), new Cradlepoint IBR600 device atom, facility narrative. All pass ot-design validate. |
| 3 | SOW-018.0 | Brownfield Wastewater Environment Definition | SOW-017.0 | Complete | Compose the Frankenstein hybrid from atoms. Docker Compose profile additions. Process model variants. Validation pass. **This is the composability proof.** |
| 4 | SOW-019.0 | Adaptive Topology Visualization | SOW-016.0, SOW-018.0 | Complete | Dashboard topology page. Server-side layout engine. Archetype-driven rendering. Era markers. Boundary state visualization. HTMX environment switching. |
| 5 | SOW-020.0 | Hybrid Environment Scenarios | SOW-018.0, SOW-019.0 | Complete | Scenario content for the wastewater environment. Discovery scenario variant highlighting hybrid challenges. Assessment scenario focused on partial-segmentation blind spots. Extended Scenarios 01-02 with Phase F and Phase E respectively. |

### Dependency Graph

```
SOW-016.0 (Schema Extensions) ──────────> SOW-019.0 (Topology Visualization)
  |                                              ^
  v                                              |
SOW-017.0 (Device & Network Atoms)               |
  |                                              |
  v                                              |
SOW-018.0 (Environment Definition) ─────────────┘
  |                                              |
  v                                              v
  └─────────────────────────────────────> SOW-020.0 (Scenarios)
```

## Device Atom Inventory (Expected)

### Reused from Existing Library

| Device | Current Atom | New Variant Needed | Role in Wastewater |
|--------|-------------|-------------------|-------------------|
| SLC-500/05 | `slc-500-05.yaml` | `ww-influent`, `ww-effluent` | Original 1997 PLCs controlling influent screening and effluent discharge |
| CompactLogix L33ER | `compactlogix-l33er.yaml` | `ww-aeration` | 2013 modernization -- aeration blower control |
| Moxa NPort 5150 | `moxa-nport-5150.yaml` | `ww-serial-gateway` | 2008 serial-to-Ethernet bridge |

### New Atoms (to be confirmed during SOW-017.0 drafting)

| Device | Category | Justification |
|--------|----------|---------------|
| Cradlepoint IBR600 | Cellular modem | Already referenced in ADR-006 device list. Models the "temporary" vendor remote access gateway. Reuse across future environments likely. |
| Managed switch (generic) | Infrastructure | Optional. May be modeled as network atom property rather than device atom. Decision during SOW-017.0. |

**Note**: The Cradlepoint IBR600 already appears in ADR-006 (Air Gap and Accidental Bridge Modeling) and in the ADR-009 design layer directory structure. It may already have a stub atom. The SOW will determine whether to create it fresh or promote the stub.

## Port Assignments

| Port | Service | Notes |
|------|---------|-------|
| 5020-5022 | Water treatment PLCs | Unchanged |
| 5030 | Manufacturing gateway | Unchanged |
| 5040-5043 | Pipeline station devices | Unchanged |
| 5060-5064 | Wastewater treatment devices | **NEW** -- 5 placements |
| 8080 | Water treatment HMI | Unchanged |
| 8081 | Manufacturing HMI | Unchanged |
| 8090 | Monitoring dashboard | Unchanged (topology view added) |
| 8091 | Alert API | Unchanged |

Port range 5060-5064 selected to leave a gap after pipeline (5040-5043) for future pipeline expansion.

## Technical Debt to Address

| ID | Description | Resolution in Beta 0.4 |
|----|-------------|------------------------|
| TD-025 | Quick Start environment couples to `wt-level1` network atom | Evaluate during SOW-016.0 schema work |
| TD-026 | Quick Start environment has no Docker Compose profile | Address during SOW-018.0 Docker Compose updates |

## Technical Debt Created

Expected new debt items from Beta 0.4:

| ID | Description | Notes |
|----|-------------|-------|
| TD-027 | No environment timeline animation | Can view current state but not step through eras. ADR-010 D5 construction/deconstruction is static, not interactive. |
| TD-028 | Topology visualization is server-rendered, not interactive | No drag, zoom, or rearrangement. Adequate for teaching but limits exploration. |
| TD-029 | Boundary states are metadata-only | No runtime enforcement of boundary rules. The managed switch VLAN is modeled as a separate Docker network, but `intended` and `absent` boundaries have no runtime effect. |
| TD-030 | No wastewater HMI | Wastewater environment has monitoring but no operator HMI. Acceptable -- the teaching focus is architecture, not operations. |
| TD-031 | Archetype classification is manual | Environment author sets the archetype field. No automatic classification based on topology analysis. |

## Open Questions

- Should the wastewater facility reuse the Cradlepoint IBR600 from ADR-006, or model a different cellular gateway vendor? (Cradlepoint is the realistic choice for municipal utilities.)
- Should the managed switch (2018 audit addition) be modeled as a device atom or as a property of the network atom? (Network property is simpler; device atom enables richer visualization.)
- How should the topology view handle environments with 15+ devices? The wastewater environment is small (~5-7 placements), but future environments could be larger. Scrolling? Zoom? Grouping?
- Should era markers link to a facility history narrative page, or is the tooltip sufficient?

## Architecture Reference

- ADR-003: Network Architecture and Topology Templates
- ADR-005: Monitoring Integration Architecture (D4 boundary applies to topology view)
- ADR-006: Air Gap and Accidental Bridge Modeling (Cradlepoint reference)
- ADR-007: Educational Scenario Framework
- ADR-009: Design Layer and Composable Environments
- ADR-010: Environment Archetypes and Adaptive Visualization (primary reference)
