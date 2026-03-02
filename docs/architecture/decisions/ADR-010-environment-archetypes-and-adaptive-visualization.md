# ADR-010: Environment Archetypes and Adaptive Visualization

**Status**: Proposed
**Date**: 2026-03-02
**Decision Makers**: Rusty Brownlee
**Builds on**: ADR-003, ADR-007, ADR-009

---

## Context

### The Real Goal of This Platform

The OT Simulator exists to get IT engineers productive in OT environments as fast as possible. Speed matters because the IT/OT convergence is happening now -- organizations are assigning IT security teams to assess and protect OT infrastructure they have never seen before. These engineers have strong mental models for enterprise networking but no framework for understanding what they encounter on a plant floor.

The fastest path from IT competence to OT competence is through concepts the engineer already owns. IT engineers understand layered architectures (OSI model, TCP/IP stack), network segmentation (VLANs, subnets, firewall zones), and defense-in-depth. The Purdue Enterprise Reference Architecture maps directly to these concepts:

| IT Concept | Purdue Equivalent | Bridging Insight |
|-----------|-------------------|------------------|
| Server VLAN | Level 3 (SCADA/site ops) | "This is where the historians and SCADA servers live" |
| DMZ | Level 3.5 | Identical concept, familiar territory |
| Management network | Level 2 (HMI/local control) | "Operator workstations are like jump boxes for the process" |
| IoT/edge devices | Level 1 (PLCs/RTUs) | "These are endpoints, but they run 20 years and never get patched" |
| Physical layer | Level 0 (process) | "There's a layer below the network -- actual physics with safety consequences" |

This mapping accelerates conceptual transfer. But it breaks down the moment an IT engineer encounters a real OT environment, because most real environments do not follow the Purdue model cleanly. Teaching only the clean version sets engineers up for a shock. Teaching only the messy version gives them no reference point for what "good" looks like. The simulator must present both -- and the transition between them.

### Where IT Assumptions Break

The teaching value lives in the gaps between IT expectations and OT reality:

- **No authentication**: IT engineers assume every protocol has auth. Modbus doesn't. Seeing an unauthenticated coil write succeed is visceral and immediate.
- **Availability over confidentiality**: In IT, you patch and reboot. In OT, uptime is safety. A PLC controlling a chemical process cannot be taken offline for a Tuesday patch.
- **Flat networks in production**: The pipeline environment isn't a bad example -- it's a common reality. Side-by-side comparison with the segmented water plant makes this land without a lecture.
- **Decades of coexistence**: A 1995 SLC-500 running next to a 2024 CompactLogix is normal. IT infrastructure cycles every 3-5 years. OT infrastructure runs for 25-40 years. This coexistence is the defining characteristic of real OT environments.

### Three Archetypes in the Wild

IT engineers will encounter three distinct archetypes of OT environment, each requiring a different mental model:

**1. Modern Segmented** -- The textbook Purdue model. Clear levels, managed switches, firewall rules between tiers. This is what standards (IEC 62443, NIST 800-82) say you should build. The water treatment plant in our simulator models this.

**2. Legacy Flat** -- Everything on one LAN. No segmentation, no managed infrastructure, no visibility. Built 15-30 years ago, never redesigned because the process never stopped running. The pipeline station in our simulator models this.

**3. Frankenstein Hybrid** -- The most common archetype in the wild and the one no simulator currently models. Started as flat in the 1990s, had Ethernet bolted on in the 2000s, got a partial VLAN scheme after an audit finding in 2018 that stalled at 60%, and acquired a cloud connector for predictive maintenance in 2023. Layers partially exist but with gaps, bypasses, and mixed-era equipment. This is what IT engineers will actually encounter on day one.

The simulator currently covers archetypes 1 and 2. Archetype 3 is absent. This gap means engineers are trained for the clean cases but unprepared for the messy reality that dominates the installed base.

### Visualization as a Teaching Tool

A topology diagram teaches more than a paragraph of documentation -- if the visual representation adapts to what it's showing. A static device list treats a properly segmented Purdue installation the same as a flat legacy network. The shape of the visualization should communicate the architecture immediately, before the engineer reads a single label.

### Shared Visual Language for IT/OT Teams

Beyond individual learning, visualization serves a communication function. When IT and OT teams look at the same topology view, the conversation shifts:

- **Without shared visual**: IT says "this network is misconfigured." OT says "it's worked fine for 20 years." Impasse.
- **With shared visual**: Both teams see the flat network, the serial backbone, the partial segmentation attempt. IT asks "what if we added a managed switch here?" OT responds "we can't touch the serial side until the next planned shutdown in Q3." The conversation moves from judgment to problem-solving.

The visualization becomes a boundary object -- an artifact both communities can reason about from their own perspective while building shared understanding.

### Composability as Architectural Validation

The design layer (ADR-009) introduced atomic composition: device atoms + network atoms + environment templates. This architecture was validated against two clean archetypes (segmented water plant, flat pipeline station). The Frankenstein hybrid is the acid test.

If the same atoms compose into a believable hybrid environment -- serial backbones from the 1990s wired alongside 2020s Ethernet with incomplete VLAN boundaries -- it proves the design layer models real-world OT infrastructure evolution, not just textbook examples. If it can't, the architecture has a fundamental gap.

---

## Decision

### D1: Three Environment Archetypes

**Decision**: The simulator recognizes three canonical environment archetypes. Every environment template is classified as one of these archetypes in its metadata. The archetype drives visualization layout and informs scenario design.

```yaml
# In environment.yaml metadata
environment:
  id: "brownfield-wastewater"
  archetype: "hybrid"          # modern-segmented | legacy-flat | hybrid
  era_span: "1996-2023"        # Oldest to newest installation date
```

| Archetype | Structure | Purdue Compliance | Visualization Layout |
|-----------|-----------|-------------------|---------------------|
| `modern-segmented` | Clear Purdue levels, managed infrastructure | Full | Vertical stack with level boundaries |
| `legacy-flat` | Single network plane, no segmentation | None | Horizontal plane, no vertical separation |
| `hybrid` | Partial levels, mixed eras, incomplete boundaries | Partial | Collapsed stack with gaps, dashed boundaries |

### D2: Adaptive Topology Visualization

**Decision**: The monitoring dashboard renders environment topology differently based on the archetype. The visual shape itself communicates the architecture before the engineer reads any labels.

**Modern Segmented** -- Vertical Purdue stack:

```
Level 3  ┌──────────────────────────────┐
         │  [SCADA/Historian]           │
         └────────────┬─────────────────┘
                      │ managed switch
Level 2  ┌────────────┴─────────────────┐
         │  [HMI Workstation]           │
         └────────────┬─────────────────┘
                      │ managed switch
Level 1  ┌────────────┴─────────────────┐
         │  [PLC]  [PLC]  [PLC]         │
         └──────────────────────────────┘
```

Traffic lines show poll paths crossing managed level boundaries. Each boundary is a solid line representing enforced segmentation.

**Legacy Flat** -- Single horizontal plane:

```
┌────────────────── Station LAN (flat) ──────────────────┐
│                                                         │
│  [Flow Computer]  [RTU]  [CompactLogix]  ╔══════════╗  │
│     10.20.1.10    .11       .12          ║ Gateway  ║  │
│                                          ╚════╤═════╝  │
│                                               │ serial  │
│                                          [SLC-500]      │
│                                          (DH-485)       │
└─────────────────────────────────────────────────────────┘
```

No vertical separation. All devices on one plane. The gateway is the only structural element. The flatness is the message.

**Frankenstein Hybrid** -- Partially collapsed stack with gaps:

```
Level 3  ┌──────────────────────────────┐
         │  [SCADA Server] (2019)       │
         └────────────┬─────────────────┘
                      │ managed switch (2019)
- - - - - - - - - - -│- - - - - - - - - - - - -  ← Level 2
                      │                              boundary
                      │ (no L2 switch -- gap)        MISSING
                      │
Level 1  ┌────────────┴──────────────────────────────────────┐
         │                                                    │
         │  [CompactLogix]     [SLC-500]═══serial═══[SLC-500]│
         │   (2014, Ethernet)   (1998, DH-485)      (1998)   │
         │         │                    │                      │
         │         │            ╔═══════╧═══════╗              │
         │         │            ║ ProSoft Gateway║              │
         │         │            ║ (2007)         ║              │
         │    ┌────┴────┐       ╚═══════════════╝              │
         │    │unmanaged│                                      │
         │    │ switch   │  ┌───────────┐                      │
         │    │ (1998)   │  │ Cloud GW  │                      │
         │    └─────────┘  │ (2023)    │                      │
         │                  └───────────┘                      │
         └─────────────────────────────────────────────────────┘

Legend:  ─── solid boundary (enforced)
        - - dashed boundary (intended but incomplete)
        (year) era marker
```

Dashed lines where boundaries should exist but don't. Era markers on every device. The visual immediately communicates: this environment was built in layers over decades, and the architecture is incomplete.

### D3: Era Markers and Device Temporal Context

**Decision**: Device atoms carry a `vintage` field (already in schema v0.1) and environment placements carry an `installed` field indicating when the device was placed in this specific environment. The visualization uses era markers to show the age mix.

```yaml
# Existing in device atom (ADR-009 D3)
device:
  vintage: 2024       # Year device model was introduced by vendor

# New in environment placement
placements:
  - id: "ww-plc-01"
    device: "slc-500-05"
    installed: 1998    # Year this specific device was installed in this facility
```

The `installed` field is optional. When present, the UI displays it as an era marker next to the device. When absent, the device atom's `vintage` is used as a fallback.

**Teaching purpose**: An IT engineer sees a 1998 SLC-500 next to a 2023 cloud gateway and immediately understands the temporal mismatch. The question shifts from "why is this here?" to "how do we secure something that predates the security tools by 25 years?"

### D4: Boundary Visualization States

**Decision**: Network boundaries between Purdue levels are rendered in one of three visual states, driven by the environment template metadata.

| State | Visual | Meaning |
|-------|--------|---------|
| `enforced` | Solid line | Managed switch, firewall, or VLAN boundary exists and is active |
| `intended` | Dashed line | Boundary was designed or planned but not fully implemented |
| `absent` | No line | No boundary exists; devices communicate freely across levels |

```yaml
# New in environment template (future schema version)
boundaries:
  - between: ["level-3", "level-2"]
    state: "enforced"
    infrastructure: "managed-switch"
    installed: 2019

  - between: ["level-2", "level-1"]
    state: "intended"
    notes: "VLAN scheme designed in 2019 audit response, never completed"

  - between: ["level-1", "external"]
    state: "absent"
    notes: "Cloud gateway added 2023 with no boundary"
```

For `modern-segmented` environments, all boundaries are `enforced`. For `legacy-flat`, no boundaries exist. For `hybrid`, the mix of states tells the facility's story.

### D5: Construction and Deconstruction

**Decision**: The atomic design layer must support incremental construction (building up a Frankenstein environment era by era) and analytical deconstruction (breaking an environment into its constituent atoms to understand what-if scenarios).

**Construction** -- A Frankenstein environment is composed by layering atoms from different eras:

| Era | Action | Atoms Added |
|-----|--------|-------------|
| 1998 | Original installation | 2x SLC-500, DH-485 serial bus, unmanaged switch |
| 2007 | Serial-to-Ethernet bridge | ProSoft gateway, flat Ethernet segment |
| 2014 | Partial modernization | CompactLogix (Ethernet-native), same flat segment |
| 2019 | Audit-driven segmentation | Managed switch, VLAN network atom (Level 3 only) |
| 2023 | Predictive maintenance | Cloud gateway, WAN link atom |

Each row is a valid, runnable environment at that point in time. The full Frankenstein is all rows composed together. This enables a scenario where the engineer walks through the facility's history to understand how it arrived at its current state.

**Deconstruction** -- What-if analysis by removing or modifying atoms:

- Remove the 2019 managed switch: what security posture exists without it?
- Remove the 2023 cloud gateway: what attack surface disappears?
- Add a VLAN boundary between Level 1 and Level 2: what does partial segmentation buy?
- Replace the ProSoft gateway with a modern managed gateway: what visibility improves?

The design layer already supports this through its atomic composition model (ADR-009). No new schema mechanism is needed -- composition and decomposition are inherent properties of the atom + environment template architecture. What is needed is:

1. Environment variants that represent different points in the facility's timeline
2. UI that can render and compare these variants
3. Scenario content that walks engineers through the construction/deconstruction exercise

### D6: Frankenstein Hybrid Environment

**Decision**: A third canonical environment will be added to the design layer, representing the Frankenstein hybrid archetype. This environment reuses existing device atoms where possible and introduces only the atoms needed to model the hybrid characteristics.

**Candidate facility types** (to be selected during SOW drafting):

| Facility | Hybrid Characteristics | Atom Reuse |
|----------|----------------------|------------|
| **Wastewater treatment** | Sister facility to existing water plant, 15 years older, ad-hoc modernization | Reuses CompactLogix, SLC-500, Moxa atoms |
| **Packaging/bottling plant** | Mix of old Allen-Bradley serial PLCs and new Ethernet drives, partial segmentation after audit | Reuses SLC-500, CompactLogix atoms |
| **Power substation** | Original serial RTUs, partial IEC 61850 migration, mixed bay modernization | Needs new RTU atom, partial reuse |

The selected facility must satisfy all of these criteria:

1. Realistic mix of eras (at least three distinct installation periods spanning 15+ years)
2. Partial Purdue compliance (some levels segmented, others not)
3. At least one protocol gateway bridging serial and Ethernet
4. At least one "unauthorized" network addition (cloud connector, vendor laptop port, etc.)
5. Maximum reuse of existing device and network atoms
6. Distinct teaching value from the existing water and pipeline environments

**This environment is the composability proof.** If it can be defined entirely through atomic YAML configuration -- device atoms, network atoms, boundary states, era markers -- without new plant code, it validates that the design layer models real-world OT infrastructure evolution. If it requires code changes to express, the schema has a gap that must be addressed first.

### D7: Visualization Architecture Requirements

**Decision**: The Beta 0.3 monitoring dashboard (SOW-014.0) must be designed with the following requirements so that the adaptive visualization can be added without a rewrite:

1. **Topology rendering as a distinct component**: The asset inventory view must separate data (device list, network membership, relationships) from presentation (layout algorithm, visual style). The initial implementation may render a flat list, but the data model must support spatial layout.

2. **Environment metadata accessible to the UI**: The dashboard must have access to environment archetype, boundary states, and era markers. The design layer volume mount (`./design:/design:ro`) already provides this.

3. **No hardcoded layout assumptions**: The dashboard must not assume a specific number of networks, Purdue levels, or devices. The layout must adapt to whatever the environment template defines.

4. **Reference vs. Observed distinction maintained**: Per the Beta 0.3 milestone spec (ADR-005 D4 boundary), design-layer data is labeled "Reference" and live monitoring data is labeled "Observed." The topology visualization shows the reference architecture; the device detail view shows observed behavior. These must remain visually distinct.

---

## Consequences

### Positive

- IT engineers learn faster by mapping Purdue levels to network concepts they already understand
- The visual shape of each archetype teaches architecture before a single label is read
- The Frankenstein hybrid prepares engineers for the most common real-world scenario
- Construction/deconstruction exercises build intuition for how OT environments evolve
- Shared visualization gives IT and OT teams a common artifact for productive conversations
- The composability proof validates the entire design layer architecture
- Era markers make the temporal reality of OT infrastructure visible and visceral

### Negative

- Adaptive visualization is more complex to implement than a static device list
- A third environment increases the design layer maintenance surface
- Boundary state metadata adds schema complexity
- Construction/deconstruction scenarios require multiple environment variants per facility

### Risks

- **Risk**: Frankenstein environments may be too complex for the atomic schema to express cleanly
- **Mitigation**: This is exactly what D6 tests. If the schema can't express it, we fix the schema before building more environments. The design layer's value depends on this capability.
- **Risk**: Adaptive visualization may over-simplify real OT architectures
- **Mitigation**: The visualization is a teaching tool, not an engineering drawing. It needs to communicate concepts accurately, not model every cable. Accept appropriate abstraction.
- **Risk**: Era markers may imply that older equipment is inherently worse
- **Mitigation**: Scenario content and UI labeling must frame age as context, not judgment. A 1998 SLC-500 that has run reliably for 28 years is not a failure -- it's a constraint that security planning must account for.
- **Risk**: IT/OT shared visualization may be perceived as oversimplifying OT complexity
- **Mitigation**: The visualization bridges understanding; it does not replace OT expertise. Scenarios explicitly require the engineer to consult OT context (process constraints, shutdown windows, safety implications) that the visualization alone cannot convey.

---

## Implementation Sequencing

This ADR does not prescribe a specific beta milestone for implementation. The following dependencies exist:

| Capability | Prerequisite | Earliest Milestone |
|-----------|-------------|-------------------|
| Environment archetype metadata | Schema v0.1 extension (non-breaking) | Beta 0.3 (in SOW-014.0 environment template) |
| Flat device list with environment grouping | SOW-014.0 dashboard | Beta 0.3 |
| Adaptive topology visualization | SOW-014.0 component architecture | Beta 0.4+ |
| Frankenstein hybrid environment | Design layer atoms + environment YAML | Beta 0.4+ |
| Era markers in UI | `installed` field in placements | Beta 0.4+ |
| Boundary state visualization | `boundaries` schema addition | Beta 0.4+ |
| Construction/deconstruction scenarios | Frankenstein environment + scenario framework | Beta 0.5+ |

Beta 0.3 lays the architectural foundation. The adaptive visualization and Frankenstein environment are candidates for the next milestone after Beta 0.3 completes.

---

## References

- ADR-003: Network Architecture and Topology Templates
- ADR-007: Educational Scenario Framework
- ADR-009: Design Layer and Composable Environments
- Purdue Enterprise Reference Architecture (PERA)
- IEC 62443: Industrial communication networks -- Network and system security
- NIST SP 800-82 Rev. 3: Guide to Operational Technology (OT) Security
- SANS ICS515: ICS Visibility, Detection, and Response
