# ADR-003: Network Architecture and Topology Templates

**Status**: Proposed (Revised 2026-03-01)
**Date**: 2026-02-28
**Decision Makers**: Rusty Brownlee
**Revised by**: ADR-009 (Design Layer and Composable Environments)

---

## Context

OT environments exhibit a wide range of network architectures, from modern Purdue-model installations with proper segmentation to legacy flat networks with zero isolation. Security engineers must understand both extremes and everything in between.

The project's reference environment models a single facility with two distinct OT networks:

1. **Water Treatment Plant (New Build, 2026)**: A modern installation with proper Purdue model segmentation, managed switches, and Ethernet-capable PLCs. Represents what "good" looks like.

2. **Manufacturing Floor (Legacy, ~1993)**: A 30+ year old flat network with serial devices, unmanaged switches, and zero segmentation. Represents reality.

These two environments are physically connected -- the water treatment plant produces process water consumed by the manufacturing floor. This connection creates the central architectural challenge: how to bridge modern segmented infrastructure with legacy flat infrastructure without compromising either.

### Why This Matters for Training

IT engineers entering OT environments typically encounter legacy infrastructure first. By modeling both environments side by side, the simulator teaches:

- What a well-designed OT network looks like (so they know what to aim for)
- What a legacy OT network looks like (so they know what they will actually face)
- Why connecting them is dangerous (so they understand the real-world constraint)
- How security overlays (Blastwave, Dragos) address different challenges in each environment

### Relationship to the Design Layer

Per ADR-009, network topology is defined in the design layer as composable atoms. This ADR defines the network concepts and the reference topology. The design layer (ADR-009) defines how those concepts are expressed in YAML and composed into environments. The plant binary instantiates whatever environment the design layer defines -- it does not assume a specific facility type.

---

## Decision

### D1: Networks Are Atomic Design Elements

**Decision**: Each network segment is defined as an atomic element in the design layer (`design/networks/`). A network atom describes a communication medium and its properties, independent of which devices are connected to it.

Network atoms capture:
- **Type**: Ethernet, RS-485 serial bus, RS-232 serial
- **Addressing**: Subnet, VLAN (Ethernet); bus addresses (serial)
- **Infrastructure**: Managed vs. unmanaged switch, SPAN/mirroring capability
- **Monitoring capability**: Whether passive observation is possible on this segment

Networks are composed into environments by referencing them in environment templates (`design/environments/`). The same network atom can be reused across multiple environments.

### D2: Reference Topology -- Water Treatment Plant (Purdue Model)

**Decision**: The reference water treatment plant implements a proper Purdue model network architecture with defined levels and segmentation between them.

```
Level 5: Enterprise Network (corporate IT, internet access)
Level 4: Site Business Planning (historian, reporting server)
         ======= IT/OT DMZ (firewall) =======
Level 3: Site Operations (SCADA server, engineering workstation)
Level 2: Area Supervisory Control (HMI operator stations)
Level 1: Basic Process Control (CompactLogix PLCs, I/O modules)
Level 0: Physical Process (sensors, actuators, field instruments)
```

**Network details**:

| Level | VLAN | Subnet | Switch | Monitoring |
|-------|------|--------|--------|------------|
| 3 | VLAN 30 | 10.10.30.0/24 | Managed (Stratix-like) | SPAN port available |
| 2 | VLAN 20 | 10.10.20.0/24 | Managed | SPAN port available |
| 1 | VLAN 10 | 10.10.10.0/24 | Managed | SPAN port available |
| 0 | (wired to L1) | -- | Point-to-point | Via L1 PLC |

**Firewall rules between levels**:
- L3 -> L1: Modbus TCP (port 5020-5024) only, source-restricted to SCADA server
- L2 -> L1: Modbus TCP read-only (FC01, FC02, FC03, FC04), no writes from HMI
- L3 -> L2: HTTP/HTTPS for HMI management
- L4 -> L3: Historian pull (OPC UA or Modbus TCP), read-only

These three network segments (`wt-level1`, `wt-level2`, `wt-level3`) are defined as individual network atoms in the design layer. Firewall rules between levels are defined in the environment template (future schema version, per ADR-009 D4 v0.3).

### D3: Reference Topology -- Manufacturing Floor (Flat, Legacy)

**Decision**: The reference manufacturing floor is a single flat network with no segmentation, representing a typical 30+ year old installation.

```
+-------------------------------------------------+
|              FLAT NETWORK                       |
|              192.168.1.0/24                      |
|              Unmanaged 24-port switch            |
|              (no VLAN, no SPAN, no management)   |
|                                                 |
|  +---------+  +----------+  +----------------+  |
|  | SLC-500 |  |Modicon984|  | Windows XP HMI |  |
|  | PLC #1  |  | PLC #2   |  | Wonderware 9.5 |  |
|  | .10     |  | .11      |  | .50            |  |
|  | Serial  |  | Serial   |  | Ethernet       |  |
|  +----+----+  +----+-----+  +----------------+  |
|       |             |                            |
|  +----+-------------+----+                       |
|  |  Moxa NPort 5150      |  +-----------------+  |
|  |  Serial-to-Ethernet   |  | Cradlepoint     |  |
|  |  Gateway (.20)        |  | Cellular Modem  |  |
|  |  Added in 2008        |  | (.99)           |  |
|  +-----------------------+  | "Temporary" 2019|  |
|                              +-----------------+  |
+-------------------------------------------------+
```

**Characteristics**:
- All devices on a single /24 subnet
- Unmanaged switch: no VLAN support, no SPAN port, no management interface
- Two PLCs on RS-485 serial, accessed via serial-to-Ethernet converter (Moxa NPort)
- Windows XP HMI running legacy SCADA software
- A cellular modem that "temporarily" connects the network to the internet (installed by a vendor in 2019, never removed)
- No NTP: device clocks drift independently
- No centralized logging
- No asset inventory beyond tribal knowledge
- Static IP addresses, no DHCP, no DNS

The manufacturing floor is represented by two network atoms: `mfg-flat` (the Ethernet segment) and `mfg-serial-bus` (the RS-485 bus behind the Moxa gateway). This distinction is important -- devices on the serial bus are not directly addressable on the Ethernet segment.

### D4: Process Water Connection Between Environments

**Decision**: The water treatment plant and manufacturing floor are connected by a process water dependency, requiring both a physical pipe and a data link.

**Physical connection**: Treated water from the clear well (water treatment Level 0) feeds the manufacturing floor's cooling and wash water systems.

**Data connection**: The manufacturing floor needs to know water quality (pH, flow rate, chlorine residual) and the water treatment plant needs to know manufacturing demand (flow request). This requires a network link between the two environments.

**The architectural challenge**:

| Option | Risk | Reality |
|--------|------|---------|
| Direct Ethernet cable between networks | Legacy flat network gains access to segmented network, defeating Purdue model | Most common in practice -- and most dangerous |
| Firewall at the boundary | Requires installing and managing infrastructure on the legacy side | Correct approach, rarely done in legacy environments |
| Blastwave gateway at the boundary | Encrypts cross-plant traffic, enforces zero trust access | Modern overlay, does not require legacy network changes |
| Air gap with manual data transfer | Operator reads water quality on one screen, enters on another | Exists in some very old facilities |

The reference environment models the **direct cable** as the default (because that is reality), represented by a `cross-plant` network atom. Scenarios demonstrate deploying a firewall or Blastwave gateway at this boundary point.

### D5: Docker Network Topology Mirrors Environment Templates

**Decision**: Docker Compose defines separate networks that mirror the network atoms defined in the design layer environment template. For the reference environment, this produces:

```yaml
networks:
  wt-level3:
    name: ot-wt-level3
    # Water treatment SCADA/Engineering
  wt-level2:
    name: ot-wt-level2
    # Water treatment HMI
  wt-level1:
    name: ot-wt-level1
    # Water treatment PLCs
  mfg-flat:
    name: ot-mfg-flat
    # Manufacturing flat network (everything on one network)
  cross-plant:
    name: ot-cross-plant
    # Connection between water treatment and manufacturing
```

Containers representing PLCs, HMIs, and SCADA servers are attached only to their appropriate network(s). The cross-plant network is the boundary where security overlays are deployed.

In the current implementation, the Docker Compose file is hand-authored to match the reference environment. In a future version, Docker Compose may be generated from the design layer environment template.

### D6: Network Atoms Are Reusable Across Environments

**Decision**: Network atoms defined in `design/networks/` are reusable building blocks. A "manufacturing-only" environment can reference just `mfg-flat` and `mfg-serial-bus`. A "water-treatment-only" environment can reference the three Purdue-level networks. The reference "greenfield" environment composes all of them with the cross-plant link.

This enables:
- Focused training environments (only the networks relevant to the lesson)
- Progressive complexity (start with one network, add the cross-plant connection later)
- Custom environments for modeling specific customer sites

---

## Consequences

### Positive
- Side-by-side comparison immediately demonstrates why Purdue model matters
- IT engineers see both the goal state and the reality they will encounter
- The cross-plant connection creates a natural integration point for security tools
- Docker networks enforce real isolation -- containers on the manufacturing network truly cannot reach water treatment L1 unless the cross-plant link exists
- Network atoms can be composed into new environments without code changes
- Scenarios can progressively improve the manufacturing network (add managed switch, deploy gateway, segment VLANs)

### Negative
- Two distinct network architectures increase Docker Compose complexity
- More containers required to model switches, gateways, and boundary devices
- Legacy manufacturing simulation requires modeling serial-to-Ethernet conversion
- Network atom YAML adds a layer of indirection compared to inline definition

### Risks
- **Risk**: Docker networking abstractions may not fully represent real L2/L3 behavior
- **Mitigation**: Accept the abstraction; the teaching value of network separation outweighs L2 fidelity
- **Risk**: Cross-plant connection scenarios may be too complex for initial phases
- **Mitigation**: Phase 1 builds each network independently; cross-plant connection is a later phase

---

## References
- ADR-009: Design Layer and Composable Environments
- ISA-95 / IEC 62443: Industrial Automation and Control Systems Security
- Purdue Enterprise Reference Architecture (PERA)
- NIST SP 800-82 Rev. 3: Guide to Operational Technology (OT) Security
