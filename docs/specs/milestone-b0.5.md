# Milestone: Beta 0.5

**Target**: Bridge the IT-to-OT mental model gap by adding live process visualization to the monitoring dashboard. An engineer looking at Modbus registers should be able to see *what physical equipment those registers control* -- tanks, pumps, valves, sensors -- with live values updating in real time. This transforms the simulator from "a bunch of register tables" into "a facility I can understand."

**Implements**: ADR-011 (Process Visualization and Instrument Mapping) -- to be authored as part of this milestone

## Definition of Done

An engineer can:

1. Open `http://localhost:8090/process` and see a list of all environments with process schematic thumbnails
2. Click into the water treatment process view and see a simplified P&ID showing intake, treatment, and distribution stages with tanks, pumps, sensors, and connecting pipes
3. See live register values displayed on each instrument in engineering units (72.4%, 340 GPM, 7.2 pH) -- not raw uint16 values -- updating every 2 seconds via HTMX
4. See ISA-5.1 instrument tags on every sensor and actuator (LT-001 = Level Transmitter, FT-002 = Flow Transmitter, AT-001 = Analyzer Transmitter), learning the standard OT tagging convention
5. Click any instrument tag and navigate to the asset detail page showing the full register table for that PLC
6. Click any PLC label in the process view and navigate to the device atom in the Design Library
7. See pump and valve status indicators change color based on coil state (green = running, grey = stopped, red = alarm)
8. View the pipeline process schematic and understand gas compression, metering, and analysis as a physical flow
9. View the wastewater process schematic with era markers on equipment, seeing how 1997 original equipment connects to 2013 and 2022 additions
10. Write a rogue value to a wastewater aeration blower setpoint and watch the dissolved oxygen level drop in real time on the process view, then see the anomaly alert appear -- connecting the attack to its physical consequence
11. Compare all three process views and understand that each facility has different physical processes but the same Modbus TCP protocol underneath
12. Open the process view on a tablet-sized browser (1024px) and have the SVG scale readably

## What Beta 0.5 Is NOT

- No interactive drag, zoom, or pan on process diagrams (server-rendered SVG, static layout)
- No physics coupling between process stages (intake flow still doesn't feed treatment inlet -- TD-013 unchanged)
- No P&ID symbol library conforming to ISA-5.1 drawing standards (simplified symbols sufficient for teaching)
- No piping and instrumentation database (PIDA) or SIL classification
- No animated fluid flow (pipe colors may indicate flow direction but no particle animation)
- No new OT protocols (Modbus TCP only)
- No attack simulation scenarios (Scenario 03 remains deferred -- the process view enhances observation, not offense)
- No Blastwave/BlastShield/Dragos integration
- No historical trend charts (live snapshot only -- historical trending is a future milestone)
- No editable process schematics (YAML-defined, not authored in the dashboard)
- No mobile-optimized layout below 1024px (tablet minimum)

## Process Schematic Design

### New Design Layer Artifact: `process.yaml`

Each environment gains an optional `process.yaml` file alongside its `environment.yaml`. This file defines the physical process in terms an IT engineer can understand, mapping abstract register addresses to real-world equipment.

```
design/environments/greenfield-water-mfg/
  environment.yaml      # existing: devices, networks, placements
  process.yaml          # NEW: physical equipment, instruments, stages
```

### Schema Structure

```yaml
# process.yaml - Physical process schematic for an environment
schema_version: "v0.1"

process:
  id: "greenfield-water-mfg"
  name: "Greenfield Water Treatment and Manufacturing Facility"
  description: "Municipal water treatment with adjacent manufacturing floor"

  # Stages define left-to-right process flow groups
  stages:
    - id: "intake"
      name: "Raw Water Intake"
      equipment:
        - id: "intake-well"
          type: "tank"
          label: "Intake Well"
          instruments:
            - tag: "LT-101"
              name: "Intake Well Level"
              isa_type: "LT"          # Level Transmitter
              placement: "wt-plc-01"  # references environment.yaml placement ID
              register:
                type: "holding"
                address: 0            # zero-based per device atom addressing
              unit: "%"
              range: [0, 100]
              scale: [0, 16383]       # raw uint16 range

        - id: "intake-pump-01"
          type: "pump"
          label: "Intake Pump P-101"
          instruments:
            - tag: "P-101"
              name: "Intake Pump 1 Run Status"
              isa_type: "run"
              placement: "wt-plc-01"
              register:
                type: "coil"
                address: 0

    - id: "treatment"
      name: "Water Treatment"
      equipment:
        # ... treatment basin, UV system, chemical feed, etc.

  # Connections define pipe routing between equipment
  connections:
    - from: "intake-well"
      to: "intake-pump-01"
      type: "pipe"
    - from: "intake-pump-01"
      to: "treatment-basin"
      type: "pipe"
      label: "Raw water supply"
```

### ISA-5.1 Instrument Tag Convention

All instrument tags follow the ISA-5.1 standard prefix convention. This is educational -- IT engineers encounter these tags on every real P&ID:

| Prefix | Meaning | Example |
|--------|---------|---------|
| LT | Level Transmitter | LT-101 (intake well level) |
| FT | Flow Transmitter | FT-201 (filter inlet flow) |
| PT | Pressure Transmitter | PT-201 (filter inlet pressure) |
| TT | Temperature Transmitter | TT-101 (raw water temperature) |
| AT | Analyzer Transmitter | AT-201 (pH), AT-202 (turbidity), AT-301 (chlorine residual) |
| PDT | Pressure Differential Transmitter | PDT-201 (filter DP) |
| P- | Pump | P-101 (intake pump 1) |
| V- | Valve | V-301 (distribution valve) |
| UV | UV System | UV-201 (disinfection system) |

Tag numbering uses the pattern `XX-NNN` where the first digit of NNN indicates the stage: 1xx = intake, 2xx = treatment, 3xx = distribution. Each environment uses its own numbering scheme appropriate to its facility.

### Key Design Decisions

1. **`process.yaml` is separate from `environment.yaml`** -- The environment defines what devices exist and how they're networked. The process schematic defines what physical equipment exists and how instruments map to registers. Not every environment needs a process schematic (quickstart-example may not get one initially).

2. **Instrument-to-register mapping lives in `process.yaml`, not in device atoms** -- The same CompactLogix atom is reused across facilities. The instrument tag LT-101 is specific to this facility's intake well, not to the CompactLogix hardware.

3. **Layout is computed from stage/equipment structure, not from coordinates** -- The YAML declares stages, equipment within stages, and connections (pipes between equipment). The `flow_direction` field determines whether stages arrange left-to-right (`horizontal`, default) or top-to-bottom (`vertical`). The Go layout engine computes SVG positions. No x,y coordinates in YAML. Beta 0.5 implements `horizontal` only; `vertical` is a follow-up within the same milestone if time permits, otherwise early Beta 0.6.

4. **Engineering unit scaling uses the existing device atom convention** -- The `scale` field in process.yaml matches the device atom register map's `range` convention: raw uint16 values mapped linearly to engineering units.

## SVG Rendering Architecture

### Symbol Library

A small set of SVG template partials define reusable equipment symbols:

| Symbol | SVG Shape | Size | Notes |
|--------|-----------|------|-------|
| `tank` | Rounded rectangle with fill level indicator | 80x100 | Fill level driven by register value (0-100%) |
| `pump` | Circle with triangle (impeller) | 50x50 | Green fill = running, grey = stopped |
| `valve` | Bowtie (two triangles) | 40x30 | Green = open, grey = closed |
| `sensor` | Small circle with tag label | 30x30 | Connected to equipment via leader line |
| `pipe` | SVG `<line>` or `<path>` | Variable | Connects equipment; arrow indicates flow direction |
| `plc` | Rectangle with label | 100x40 | Shows PLC placement ID and device type |
| `gateway` | Rectangle with antenna icon | 80x40 | For Moxa, Cradlepoint |
| `blower` | Circle with fan blades | 50x50 | For wastewater aeration |
| `chromatograph` | Rectangle with "GC" label | 60x40 | For pipeline gas analysis |

### Rendering Pipeline

```
process.yaml ──> Go layout engine ──> SVG coordinates ──> Go html/template ──> SVG output
                                                                                   │
                                                                            HTMX polls
                                                                            /partials/process-values/{env-id}
                                                                                   │
                                                                            Updates <text> elements
                                                                            with live register values
```

1. **Layout engine** (Go): Reads `process.yaml`, computes x,y positions for all equipment and pipes. Stages are arranged left-to-right with configurable spacing. Equipment within stages is arranged top-to-bottom. Pipes route between connection points.

2. **SVG generation** (Go templates): Template partials render each symbol type at computed positions. Instrument values are rendered as `<text>` elements with `hx-swap-oob` IDs for HTMX partial updates.

3. **Live updates** (HTMX): The process page polls `/partials/process-values/{env-id}` every 2 seconds. The response is a set of out-of-band `<text>` swaps that update instrument values and equipment status colors without re-rendering the full SVG.

### Technology

- **No new dependencies** -- SVG is native browser capability, generated by Go `html/template`
- **No JavaScript** -- HTMX handles polling and DOM updates (already a CDN dependency)
- **No canvas or WebGL** -- Pure SVG for accessibility and print-friendliness
- **Responsive** -- SVG `viewBox` attribute scales to container width; minimum readable at 1024px

## Water Treatment Process View (Reference Design)

This is the primary teaching example. Pipeline and wastewater follow the same pattern.

```
┌─── INTAKE (wt-plc-01) ──────┐  ┌─── TREATMENT (wt-plc-02) ──────────┐  ┌─── DISTRIBUTION (wt-plc-03) ──┐
│                              │  │                                     │  │                               │
│  ┌──────────┐                │  │  ┌──────────────┐                  │  │  ┌──────────┐                 │
│  │ ~~~~~~~  │  LT-101 72%   │  │  │              │  PDT-201 12 kPa  │  │  │ ~~~~~~~~ │  LT-301 65%    │
│  │ INTAKE   │  TT-101 14°C  │  │  │  TREATMENT   │  UV-201 40mW/cm² │  │  │ CLEAR    │  AT-301 1.2mg/L│
│  │ WELL     │  AT-101 7.2pH │  │  │  BASIN       │  AT-201 0.3 NTU  │  │  │ WELL     │  TT-301 15°C   │
│  │          │  AT-102 3 NTU │  │  │              │  FT-201 42 mL/min│  │  │          │                 │
│  └────┬─────┘               │  │  └──────┬───────┘                  │  │  └────┬─────┘                 │
│       │                     │  │         │                          │  │       │                        │
│  ┌────┴─────┐               │  │    ┌────┴─────┐                   │  │  ┌────┴─────┐   ┌──────────┐  │
│  │ P-101 ●  │ RUNNING       │  │    │ P-201 ●  │ RUNNING           │  │  │ P-301 ●  │   │ P-302 ●  │  │
│  │ P-102 ○  │ STOPPED       │  │    │BACKWASH○  │ IDLE              │  │  │ P-303 ●  │   │BOOSTER   │  │
│  └──────────┘               │  │    └──────────┘                   │  │  └──────────┘   └──────────┘  │
│       │                     │  │         │                          │  │       │                        │
└───────┼─────────────────────┘  └─────────┼──────────────────────────┘  └───────┼────────────────────────┘
        │         pipe                     │         pipe                        │         pipe
        └──────────────────────────────────┘─────────────────────────────────────┘──────────────> Distribution
                                                                                                 Network
```

Each instrument tag (LT-101, AT-101, etc.) displays:
- **Tag ID** (ISA-5.1 standard)
- **Live value** in engineering units (scaled from raw register)
- **Color coding**: green = normal, yellow = warning threshold, red = alarm

Each pump/valve shows:
- **Equipment ID** (P-101, V-301)
- **Status indicator**: filled circle = running/open, hollow circle = stopped/closed
- **Color**: green = active, grey = inactive, red = alarm/trip

## Pipeline Process View (Reference Design)

```
┌─── COMPRESSION (ps-plc-01) ──────────┐  ┌─── METERING (ps-rtu-01) ────────┐  ┌─── ANALYSIS (ps-fc-01) ──┐
│                                       │  │                                  │  │                           │
│  ┌────────────────┐                   │  │  ┌──────────────┐               │  │  ┌──────────────┐        │
│  │  COMPRESSOR    │  ST-401 4200 RPM  │  │  │  ORIFICE     │  FT-501       │  │  │  GAS         │        │
│  │  C-401         │  PT-401 580 PSIG  │  │  │  METER       │  2480 MSCFH   │  │  │  CHROMATOGRAPH│       │
│  │                │  PT-402 870 PSIG  │  │  │  FM-501      │  DPT-501      │  │  │  GC-601      │        │
│  │  [VFD]         │  BT-401 182°F    │  │  │              │  50.2 inH2O   │  │  │              │        │
│  │                │  BT-402 167°F    │  │  │              │  PT-501       │  │  │  CH4  92.1%  │        │
│  │                │  VT-401 1.8 mils │  │  │              │  605 PSIG     │  │  │  BTU  1035   │        │
│  └────────┬───────┘                   │  │  └──────┬───────┘               │  │  │  SG   0.601  │        │
│           │                           │  │         │                        │  │  └──────────────┘        │
│      ┌────┴─────┐                     │  │    ┌────┴─────┐                 │  │                           │
│      │SURGE PRO │ INACTIVE            │  │    │  VOL     │ 1,247 MCF      │  │  Status: ANALYZING        │
│      │SV-401    │                     │  │    │  TODAY   │                 │  │  Cycle: 3:42 remaining    │
│      └──────────┘                     │  │    └──────────┘                 │  │                           │
└───────────┼───────────────────────────┘  └─────────┼────────────────────────┘  └───────────────────────────┘
            │              pipe                       │              pipe
  Suction ──┘                                        └──> Custody Transfer Point ──> Pipeline
```

## Wastewater Process View (Reference Design)

Includes era markers on each equipment group:

```
┌─── INFLUENT [1997] (ww-plc-01) ──────┐  ┌── AERATION [2013] (ww-plc-03) ────┐  ┌── EFFLUENT [1997] (ww-plc-02) ──┐
│                                       │  │                                    │  │                                  │
│  ┌──────────┐    ┌──────────┐        │  │  ┌──────────────┐                  │  │  ┌──────────────┐                │
│  │BAR SCREEN│    │PRIMARY   │        │  │  │  AERATION    │  AT-201 2.1mg/L  │  │  │  SECONDARY   │                │
│  │          │    │CLARIFIER │        │  │  │  BASIN       │  DO setpoint     │  │  │  CLARIFIER   │  LT-301 8.2ft │
│  │PDT-101   │    │LT-102    │        │  │  │              │  AT-202 3100mg/L │  │  │              │                │
│  │3.2 inWC  │    │7.8 ft    │        │  │  │  ┌────────┐ │  FT-201 850 SCFM │  │  │  FT-301      │                │
│  └────┬─────┘    └────┬─────┘        │  │  │  │BLOWER  │ │  IT-201 85°F     │  │  │  45 GPM RAS  │                │
│       │               │              │  │  │  │B-201 ● │ │  PT-201 8.2 PSI  │  │  │  FT-302      │                │
│  ┌────┴─────┐    ┌────┴─────┐        │  │  │  │B-202 ○ │ │                  │  │  │  12 GPM WAS  │                │
│  │GRIT PUMP │    │SLUDGE    │        │  │  │  └────────┘ │                  │  │  └──────┬───────┘                │
│  │P-101 ●   │    │P-102 ●   │        │  │  └──────┬──────┘                  │  │         │                        │
│  └──────────┘    └──────────┘        │  │         │                          │  │    ┌────┴─────┐   ┌──────────┐  │
│       │               │              │  │         │                          │  │    │UV SYSTEM │   │DISCHARGE │  │
│  FT-101              AT-103          │  │         │                          │  │    │UV-301    │   │V-301     │  │
│  2150 GPM            7.1 pH         │  │         │                          │  │    │87% UVT   │   │OPEN ●    │  │
│                                       │  │         │                          │  │    └──────────┘   └──────────┘  │
│  [Cradlepoint WAN ── vendor cloud]    │  │         │                          │  │                                  │
│  [2022] ⚠ INTERNET-CONNECTED         │  │         │                          │  │  Permit: ■ UV ■ Flow ■ Active   │
└───────────┼───────────────────────────┘  └─────────┼──────────────────────────┘  └──────────┼──────────────────────┘
            │                                        │                                        │
  Influent ─┘────────────────────────────────────────┘────────────────────────────────────────┘──> River Discharge
```

The wastewater view highlights:
- **Era markers** [1997], [2013], [2022] on each equipment group
- **Network Context callout** (RD-4): The Cradlepoint WAN link is rendered in a separate dashed-border box labeled "Network Context" with a warning indicator and link to the topology page. It is visually distinct from the physical process flow.
- **Permit interlock status** as a visual indicator (all three bits must be green to discharge)

## SOW Backlog

### Ordered by Dependency

| # | SOW | Title | Dependencies | Status | Notes |
|---|-----|-------|-------------|--------|-------|
| 1 | SOW-021.0 | Process Schematic Schema and Validation | None | Planned | Define process.yaml schema, ISA-5.1 tag convention, validation rules. Extend ot-design validate. |
| 2 | SOW-022.0 | SVG Symbol Library and Layout Engine | SOW-021.0 | Planned | Go template partials for equipment symbols. Layout engine computes positions from process.yaml. HTMX live value update pattern. |
| 3 | SOW-023.0 | Water Treatment Process Schematic | SOW-021.0 | Planned | Author process.yaml for greenfield-water-mfg. Map all intake/treatment/distribution instruments to registers. Verify scaling. |
| 4 | SOW-024.0 | Pipeline and Wastewater Process Schematics | SOW-021.0, SOW-023.0 | Planned | Author process.yaml for pipeline-monitoring and brownfield-wastewater. Pipeline covers compression/metering/analysis. Wastewater includes era markers and Cradlepoint WAN indicator. |
| 5 | SOW-025.0 | Dashboard Process View Integration | SOW-022.0, SOW-023.0 | Planned | New /process routes, environment selector, navigation links from asset detail and topology pages. Register value scaling and live HTMX updates. |
| 6 | SOW-026.0 | Process View Scenario Extensions | SOW-025.0, SOW-024.0 | Planned | Extend Scenarios 01 and 02 with process view walkthroughs. New Phase G showing how process context changes the discovery and assessment experience. |

### Dependency Graph

```
SOW-021.0 (Schema) ──────────────────────> SOW-022.0 (SVG Engine)
  │                                              │
  ├──> SOW-023.0 (Water Schematic)               │
  │         │                                    │
  │         ├──> SOW-024.0 (Pipeline + WW)       │
  │         │                                    │
  │         └────────────────────────────> SOW-025.0 (Dashboard Integration)
  │                                              │
  │                                              v
  └──────────────────────────────────────> SOW-026.0 (Scenario Extensions)
```

## Schema Extensions

### New File: `process.yaml` (per environment)

Top-level structure:

```yaml
schema_version: "v0.1"

process:
  id: string                    # matches environment.yaml id
  name: string                  # human-readable facility name
  description: string           # optional narrative
  flow_direction: enum          # horizontal (default) | vertical
                                # horizontal: stages flow left-to-right (pipeline, water treatment)
                                # vertical: stages flow top-to-bottom (gravity-fed processes)
                                # declared per environment by the schematic author

  stages:                       # ordered in flow_direction (L-to-R or T-to-B)
    - id: string                # unique within this process
      name: string              # display label
      controller:               # optional: primary PLC for this stage
        placement: string       # references environment.yaml placement ID
        device: string          # references device atom ID (informational)
      equipment:                # physical equipment in this stage
        - id: string            # unique within this process
          type: enum            # tank | pump | valve | blower | sensor_station |
                                # chromatograph | gateway | uv_system | clarifier |
                                # screen | compressor | meter
          label: string         # display name (e.g., "Intake Well")
          era: int              # optional: installation year (for wastewater era markers)
          instruments:           # sensors and actuators on this equipment
            - tag: string       # ISA-5.1 tag (e.g., "LT-101")
              name: string      # human-readable name
              isa_type: string  # ISA letter code: LT, FT, PT, TT, AT, PDT, etc.
              placement: string # environment.yaml placement ID
              register:
                type: enum      # holding | coil
                address: int    # register address (respects device addressing mode)
              unit: string      # engineering unit (%, GPM, PSI, degF, mg/L, etc.)
              range: [min, max] # engineering value range
              scale: [min, max] # raw uint16 range (default [0, 16383] if omitted)
              thresholds:       # optional alarm thresholds for color coding
                warning: number # yellow above this value
                alarm: number   # red above this value

  network_context:              # optional: security-relevant network elements overlaid on process view
    - id: string                # unique within this process
      type: enum                # wan_link | internet_gateway | wireless_bridge
      label: string             # display name (e.g., "Cradlepoint WAN - Vendor Cloud")
      era: int                  # optional: installation year
      placement: string         # optional: references environment.yaml placement ID
      warning: string           # optional: security warning text (e.g., "INTERNET-CONNECTED")
      notes: string             # optional: context (e.g., "Installed as 'temporary' vendor access in 2022")

  connections:                  # pipes/links between equipment
    - from: string              # equipment ID
      to: string                # equipment ID
      type: enum                # pipe | serial | wireless
      label: string             # optional flow description
```

### Validation Rules (ot-design validate)

- `flow_direction` must be `horizontal` or `vertical` (default `horizontal` if omitted)
- Every `placement` reference must exist in the environment's `environment.yaml`
- Every `register.address` must be within the device atom's register capacity
- `register.type` must be `holding` or `coil`
- `tag` must be unique within the process
- `equipment.id` must be unique within the process
- `connections.from` and `connections.to` must reference valid equipment IDs
- `network_context` entries with `placement` must reference valid environment placements
- `era` year must be >= 1970 and <= current year
- Warn if a stage references a placement not in the environment

## Port Assignments

No new ports. All data flows through existing infrastructure:

| Port | Service | Change |
|------|---------|--------|
| 8090 | Monitoring dashboard | New `/process` routes added |
| 8091 | Alert API | No change -- process view reads existing `/api/assets/{id}/registers` |

## Technical Debt to Address

| ID | Description | Resolution in Beta 0.5 |
|----|-------------|------------------------|
| TD-029 | CDN dependencies (Bootstrap, HTMX) require internet | No change -- SVG rendering is native, but page layout still uses Bootstrap |
| TD-033 | Register scaling convention is hardcoded | Process view uses explicit scale ranges from process.yaml, partially addressing this |

## Technical Debt Created

| ID | Description | Notes |
|----|-------------|-------|
| TD-032 | Process schematics are static layout only | No drag, zoom, pan, or interactive rearrangement. Adequate for teaching. |
| TD-033 | No animated fluid flow | Pipes show flow direction arrows but no animation. CSS animation could be added later. |
| TD-034 | No process coupling visualization | Process stages are independent in the simulation (TD-013). The process view shows them connected, which could mislead engineers into thinking changes in one stage immediately affect the next. A disclaimer or visual indicator is needed. |
| TD-035 | quickstart-example has no process schematic | Acceptable -- quickstart is a minimal teaching environment, not a realistic facility. |
| TD-036 | SVG symbol library is minimal | Only covers equipment types present in current environments. New equipment types (heat exchangers, reactors, etc.) require new template partials. |
| TD-037 | No print/export of process schematics | SVG is print-friendly by nature, but no dedicated print stylesheet or PDF export. |

## Resolved Design Decisions

These questions were raised during milestone drafting and resolved before SOW work began. Documented here to prevent re-litigation in later sessions.

### RD-1: Non-instrumented equipment in process view

**Decision**: Show physical process equipment (tanks, pipes, pumps) even if they lack Modbus instrumentation, because they provide spatial context for understanding where instruments are in the physical plant. Do NOT show network infrastructure (switches, hubs, routers) -- that belongs on the topology page. The process view shows the physical plant; the topology view shows the network.

**Example**: A pipe connecting two tanks is shown even without a flow sensor. An unmanaged switch is not shown.

### RD-2: Flow direction (horizontal vs. vertical)

**Decision**: Support both via a `flow_direction` field in `process.yaml` (`horizontal` | `vertical`). The schematic author declares the appropriate direction per environment -- this is a property of the facility, not a user preference at view time. Horizontal suits water treatment and pipeline (left-to-right process flow). Vertical suits gravity-fed processes like wastewater (top-to-bottom).

**Implementation sequence**: Horizontal first (SOW-022.0). Vertical as a follow-up within Beta 0.5 if time permits, otherwise early Beta 0.6. The `flow_direction` field is in the schema from day one regardless.

### RD-3: Historical sparklines

**Decision**: No. Live current values only. If trend analysis is needed in the future, the right answer is exporting telemetry to external tools (Prometheus/Grafana), not hand-rolling SVG sparklines in the dashboard.

### RD-4: Cradlepoint and network context in process view

**Decision**: Render security-relevant network elements (internet gateways, WAN links, wireless bridges) in a separate "Network Context" callout box within the process view. This box sits adjacent to the process flow, visually distinct (different background, dashed border), and links to the topology page for that environment. It bridges the process and topology views without polluting the physical process flow diagram.

**Schema support**: The `network_context` section in `process.yaml` defines these overlay elements with optional warning text and era markers.

## Architecture Reference

- ADR-005: Monitoring Integration Architecture (D4 boundary -- process view reads from monitoring API, not plant)
- ADR-009: Design Layer and Composable Environments (process.yaml extends the design layer)
- ADR-010: Environment Archetypes and Adaptive Visualization (topology view complements process view)
- ADR-011: Process Visualization and Instrument Mapping (to be authored -- covers schema design, ISA-5.1 convention, SVG rendering approach)
