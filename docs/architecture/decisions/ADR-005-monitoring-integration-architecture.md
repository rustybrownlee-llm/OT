# ADR-005: Monitoring Integration Architecture

**Status**: Proposed (Revised 2026-03-01)
**Date**: 2026-02-28
**Decision Makers**: Rusty Brownlee
**See also**: ADR-009 (Design Layer and Composable Environments)

---

## Context

The simulator must support two categories of monitoring tools:

1. **Custom monitoring** built in the `/monitoring/` module by engineers learning OT security
2. **Commercial (COTS) tools** such as Blastwave/BlastShield (zero trust overlay) and Dragos (passive visibility/threat detection) that may be deployed onto the simulated environment

Both categories need defined integration points -- places where monitoring tools can observe or control traffic. These integration points differ dramatically between the modern water treatment network and the legacy manufacturing floor, and this difference is itself a teaching tool.

### Blastwave/BlastShield Architecture (Reference)

BlastShield is a Software-Defined Perimeter (SDP) product that creates an encrypted peer-to-peer overlay mesh. Key components:

| Component | Purpose | Deployment |
|-----------|---------|------------|
| Orchestrator | Central policy management | Cloud SaaS or on-prem VM |
| Security Gateway | Agentless inline protection, network cloaking | VM with 2+ NICs inline |
| Host Agent | Endpoint zero trust node | Installed on servers/workstations |
| Client | Authenticated access | Installed on engineering laptops |

Gateway requirements: x86 VM, 2GB storage, 1GB RAM, 2 NICs (inline mode). Protocol-transparent -- wraps Modbus/DNP3/OPC in AES-256-GCM tunnels without modifying the underlying protocol.

### Dragos Architecture (Reference)

Dragos is a passive OT network monitoring platform. Key requirements:

| Requirement | Detail |
|-------------|--------|
| Network TAP or SPAN port | Must see a copy of all traffic |
| Sensor appliance | Physical or virtual, runs protocol deep-packet inspection |
| Console | Central management, threat intelligence, asset inventory |

Dragos performs deep-packet inspection of Modbus, DNP3, EtherNet/IP, OPC, and other OT protocols. It builds an asset inventory, baselines normal behavior, and detects anomalies and known threat signatures.

---

## Decision

### D1: Each Network Zone Defines Its Monitoring Interfaces

**Decision**: Every network zone in the simulation explicitly defines what monitoring interfaces are available, reflecting the real infrastructure constraints. Per ADR-009, monitoring capability is a property of the network atom in the design layer. In future schema versions (v0.2+), monitoring attachment points (SPAN ports, TAP insertion points, syslog sources) are defined on network atoms and environment templates.

**Water Treatment Plant (Modern)**:

| Zone | SPAN Available | TAP Possible | Inline Point | Syslog Source |
|------|---------------|-------------|-------------|---------------|
| Level 3 (SCADA) | Yes | Yes | Firewall L3/L4 boundary | Managed switch |
| Level 2 (HMI) | Yes | Yes | Between L2 and L1 switches | Managed switch |
| Level 1 (PLCs) | Yes | Yes | Between L1 switch and PLCs | Managed switch |
| IT/OT DMZ | Yes | Yes | DMZ firewall | Firewall logs |

**Manufacturing Floor (Legacy)**:

| Zone | SPAN Available | TAP Possible | Inline Point | Syslog Source |
|------|---------------|-------------|-------------|---------------|
| Flat network | No (unmanaged switch) | Yes (but requires physical insertion) | Between switch and any device | None |
| Serial bus | No | No (RS-485 is not IP) | Moxa gateway is the only IP-visible point | Moxa web log (if enabled) |

**Teaching value**: On the water treatment side, deploying Dragos is straightforward -- configure a SPAN port on the managed switch, done. On the manufacturing side, there is no SPAN port. An engineer must either replace the unmanaged switch with a managed one (requires downtime) or install a network TAP (requires physical cable insertion, also requires downtime). This friction is real and intentional.

### D2: Simulated SPAN/TAP as Docker Network Mirroring

**Decision**: Monitoring integration points are implemented as Docker network attachment points. A monitoring container joins the appropriate Docker network in read-only/promiscuous mode to receive traffic copies.

**Implementation**:
- Water treatment SPAN: monitoring container joins `ot-wt-level1` network
- Manufacturing TAP: monitoring container joins `ot-mfg-flat` network
- Cross-plant observation: monitoring container joins `ot-cross-plant` network

In practice, Docker does not enforce true promiscuous mode isolation -- all containers on a network can see all traffic. This is actually more permissive than reality (a SPAN port only mirrors specific ports). The gap is acceptable because the educational goal is protocol analysis, not network-level access control fidelity.

### D3: Blastwave Integration Points

**Decision**: The simulation defines specific insertion points where a BlastShield gateway can be deployed, with documentation for how to configure each one.

| Insertion Point | Location | Purpose |
|----------------|----------|---------|
| Cross-plant boundary | Between water treatment L1 and manufacturing flat network | Encrypt and authenticate cross-plant Modbus traffic |
| Manufacturing ingress | In front of Moxa gateway on manufacturing network | Protect serial-to-Ethernet bridge from unauthorized access |
| Remote access | Between enterprise network and IT/OT DMZ | Secure remote engineering access |

Each insertion point requires the BlastShield gateway VM to be configured with two NICs bridging the adjacent network segments. The Docker Compose file will include commented-out service definitions for BlastShield gateway containers at each insertion point, ready to be enabled when a trial license is available.

Per ADR-009, security overlay insertion points will be defined in the environment template (schema v0.3) so they can vary by environment. The reference environment (`greenfield-water-mfg`) defines the three insertion points above.

### D4: Monitoring Module Consumes Only Network-Observable Data

**Decision**: The `/monitoring/` module MUST NOT import `/plant/` packages or access plant state directly. All monitoring data comes from network observation.

**Allowed data sources for monitoring**:
- Modbus TCP traffic captured from Docker networks (packet capture)
- Modbus TCP responses from polling PLCs (active scanning)
- HTTP responses from device web interfaces (CompactLogix, Moxa)
- ICMP responses (ping sweeps)
- ARP table entries (passive discovery)

**Forbidden data sources for monitoring**:
- Direct Go function calls to plant code
- Shared memory or message queues between plant and monitoring
- Plant configuration files (monitoring must discover the environment, not read a manifest)
- Plant internal state (register values before they are written to Modbus server)

**Rationale**: This constraint forces engineers to build monitoring tools that work against real OT environments, not just this simulator. If a monitoring tool reads plant config files instead of doing network discovery, it teaches nothing transferable.

---

## Consequences

### Positive
- Clear integration points for both custom and COTS monitoring tools
- Infrastructure constraints differ by zone, teaching realistic deployment challenges
- Monitoring module develops transferable skills (packet analysis, Modbus parsing, active scanning)
- Blastwave/Dragos integration is designed in, not bolted on

### Negative
- Docker network mirroring is more permissive than real SPAN/TAP (all traffic visible)
- Cannot simulate physical TAP installation challenges (cable splicing, signal loss)
- BlastShield integration requires a trial license from BlastWave (sales-assisted process)

### Risks
- **Risk**: Engineers may find the monitoring module too constrained (cannot see plant internals)
- **Mitigation**: This constraint IS the lesson. If monitoring is easy, the training has no value. Provide a "cheat sheet" mode in scenarios for verification, not for monitoring.
- **Risk**: Docker networking differences from real OT networks may create false confidence
- **Mitigation**: Document the gaps explicitly in scenario materials. Emphasize that Docker networks are an abstraction.

---

## References
- ADR-009: Design Layer and Composable Environments
- BlastShield Overview and Architecture: https://support.blastwave.com/bws/blastshieldtm-overview-and-architecture
- BlastWave Free Trial: https://www.blastwave.com/free-trial
- Dragos Platform Architecture: https://www.dragos.com/platform/
- Docker Networking Documentation: https://docs.docker.com/network/
