# ADR-011: Monitoring Telemetry Stack

**Status**: Proposed
**Date**: 2026-03-02
**Decision Makers**: Rusty Brownlee
**Builds on**: ADR-005, ADR-007, ADR-009

---

## Context

### The Learning Journey Has a Gap

ADR-005 defines where monitoring tools can attach (SPAN ports, TAP points, Docker network joins) and establishes the hard boundary that monitoring consumes only network-observable data. ADR-007 defines the four-phase learning progression (Discovery → Assessment → Attack → Defense). What neither ADR addresses is the *telemetry progression* -- the layers of observability an IT engineer must build up before commercial tools make sense.

The monitoring module today has:
- **Active Modbus polling** (register snapshots every 2 seconds)
- **Statistical baselining** (Welford's algorithm, per-register mean/stddev/min/max)
- **In-memory alerts** (threshold violations, stuck registers, new device detection)
- **Time-series ring buffers** (300 samples per register, ~10 minutes at 2s polling)
- **Dashboard** with asset inventory, register detail, topology view, and process view

What it lacks is everything between "I can read registers" and "I can detect an attacker." That gap is exactly where traditional network monitoring lives, and it is where IT engineers have transferable skills they don't yet know how to apply in an OT context.

### Why Traditional Telemetry Before Commercial Tools

An IT engineer who deploys Dragos or Cisco CyberVision without understanding what those tools do underneath will treat them as black boxes. When the tool raises an alert, the engineer cannot evaluate whether it is a true positive or noise. When the tool misses something, the engineer cannot explain why.

The simulator's educational value depends on engineers building the telemetry stack from the ground up -- at least conceptually -- before layering commercial tools on top. The progression mirrors how a SOC analyst learns: you parse raw logs before you trust a SIEM, you read packets before you trust an IDS.

### Commercial Tool Landscape

The simulator targets integration with three categories of commercial OT security tools, each occupying a distinct niche:

| Tool | Category | What It Does | What It Needs |
|------|----------|-------------|---------------|
| **Cisco CyberVision** | Passive visibility + asset inventory | DPI of OT protocols, automatic asset classification, vulnerability mapping, communication baselines | SPAN/TAP traffic feed, sensor appliance (physical or virtual), Cisco DNA Center or standalone management |
| **Dragos Platform** | Threat detection + intelligence | Protocol-aware behavioral analysis, threat intelligence correlation (CHERNOVITE, VOLTZITE, etc.), incident investigation workbench | SPAN/TAP traffic feed, sensor appliance, console, threat intel subscription |
| **Blastwave BlastShield** | Zero-trust microsegmentation | SDP overlay mesh, network cloaking, encrypted peer-to-peer tunnels, protocol-transparent | Security gateway VM (2 NICs inline), orchestrator (cloud or on-prem), host agents on endpoints |

These tools do not compete -- they layer. CyberVision and Dragos are visibility tools (see what's happening). BlastShield is a control tool (restrict what's allowed). A mature OT security program uses both categories together.

### The Trainee's Mental Model

An IT engineer encountering these tools needs a framework for understanding what each tool replaces or augments in their manual workflow:

```
Manual Layer              Commercial Equivalent         What It Adds
─────────────────────     ────────────────────────      ─────────────────────────────
Packet capture (tcpdump)  CyberVision / Dragos sensor   Protocol parsing, asset ID
Asset inventory (manual)  CyberVision asset DB          Automatic classification, CVE mapping
Traffic baseline (custom) CyberVision / Dragos baseline Learned normal, threshold tuning
Anomaly alerts (custom)   Dragos threat detection        Known threat signatures, TTP correlation
Network segmentation      BlastShield SDP overlay        Encrypted tunnels, identity-based access
```

The left column is what the simulator must teach first. The right column is what commercial tools provide. The middle column shows the tool category. Without the left column, the right column is magic.

---

## Decision

### D1: Five-Layer Telemetry Stack

**Decision**: The monitoring module implements a five-layer telemetry stack. Each layer builds on the one below it. The simulator teaches all five layers; commercial tools accelerate layers 2-5.

| Layer | Name | What It Captures | Educational Goal |
|-------|------|-----------------|------------------|
| L1 | **Protocol Telemetry** | Modbus transactions: function codes, register addresses, values, timing, source/destination, success/failure | "What is each device saying on the wire?" |
| L2 | **Asset Intelligence** | Device fingerprinting, communication graph, protocol capabilities, firmware version (where discoverable), vulnerability mapping | "What is each device, and what are its weaknesses?" |
| L3 | **Behavioral Baseline** | Normal traffic patterns, polling intervals, register value ranges, function code distribution, conversation pairs | "What does normal look like for this environment?" |
| L4 | **Anomaly Detection** | Deviations from baseline: new devices, new conversations, unauthorized writes, value excursions, timing anomalies | "What just changed, and is it suspicious?" |
| L5 | **Security Overlay** | Network segmentation enforcement, access control, encrypted tunnels, policy compliance | "How do we prevent this from happening again?" |

**Current state**: L3 and L4 are partially implemented (baseline engine, alert system). L1 and L2 are largely missing. L5 requires commercial tool integration (BlastShield).

**Build order**: L1 → L2 → (strengthen L3, L4) → L5. This mirrors the natural learning progression: you cannot baseline behavior you cannot observe, and you cannot detect anomalies in data you do not collect.

### D2: Protocol Telemetry Is the Foundation

**Decision**: Layer 1 (Protocol Telemetry) captures every Modbus transaction as a structured event. This is the OT equivalent of a web server access log -- the single most useful artifact for understanding what is happening on the network.

A Modbus transaction event contains:

| Field | Type | Description |
|-------|------|-------------|
| timestamp | time.Time | When the transaction occurred |
| src_addr | string | Source IP:port |
| dst_addr | string | Destination IP:port |
| unit_id | uint8 | Modbus slave/unit ID |
| function_code | uint8 | Modbus function code (1-127) |
| function_name | string | Human-readable function name ("Read Holding Registers") |
| register_start | uint16 | Starting register address |
| register_count | uint16 | Number of registers in request |
| values | []uint16 | Register values (for write operations and read responses) |
| success | bool | Whether the transaction completed without error |
| exception_code | uint8 | Modbus exception code if success=false |
| response_time_us | int64 | Round-trip time in microseconds |
| is_write | bool | Whether this is a write operation (FC 5,6,15,16) |

**Teaching value**: An IT engineer reviewing a transaction log immediately sees patterns:
- Device A polls device B for registers 0-15 every 2 seconds (normal SCADA polling)
- An unknown IP suddenly writes to register 5 on device B (unauthorized setpoint change)
- Device C starts returning exception code 2 (illegal data address) -- possible misconfiguration or probing

This is the same analytical skill IT engineers use with web access logs, firewall logs, and DNS query logs -- applied to an unfamiliar protocol.

### D3: Telemetry Capture Points

**Decision**: Protocol telemetry is captured at two points, each teaching a different monitoring technique:

| Capture Point | Technique | What It Sees | Limitation |
|--------------|-----------|-------------|------------|
| **Active poller** | The monitor's own Modbus client records its transactions | Only the monitor's own polls -- register values, response times, error codes | Cannot see other clients polling the same device; cannot see device-to-device traffic |
| **Passive listener** | Packet capture on the Docker/simulated network; Modbus TCP frame parsing | All Modbus traffic on the segment -- including traffic between devices and from other clients | Requires SPAN/TAP access; encrypted traffic (post-BlastShield) is opaque |

**Phase 1 (Beta 0.6)**: Implement active poller telemetry only. The monitor already polls devices; adding structured transaction logging to the existing poller is low-effort and immediately useful.

**Phase 2 (Beta 0.7+)**: Implement passive capture. This requires the plant to generate inter-device Modbus traffic (PLC-to-PLC or HMI-to-PLC), which is a plant-layer feature. The monitoring module adds a packet capture goroutine that joins the appropriate Docker network and parses raw TCP frames.

**Teaching value of the gap**: Active monitoring sees only what you ask for. Passive monitoring sees everything. This is the fundamental difference between an IT engineer running `curl` against endpoints vs. running Wireshark on the network segment. Both are useful; neither is sufficient alone. CyberVision and Dragos are passive tools -- they see everything on the segment. The monitor's poller is an active tool. Showing both to trainees and asking "what can each one detect?" is a powerful exercise.

### D4: Event Store Architecture

**Decision**: Telemetry events are stored in a structured, queryable, persistent event store. The store serves three purposes:

1. **Dashboard visualization** -- display event timelines, transaction tables, communication graphs
2. **Baseline input** -- feed the behavioral baseline engine with structured events instead of raw register snapshots
3. **Export/integration** -- forward events to external systems (syslog, SIEM, webhook) for commercial tool integration

**Storage strategy**: SQLite with a single `events` table. Partitioned by day. Configurable retention (default: 7 days). WAL mode for concurrent read/write.

**Rationale for SQLite**: The monitoring module runs as a single process. SQLite provides ACID transactions, SQL query capability, and zero external dependencies. It embeds cleanly in a Go binary. For an educational tool processing hundreds (not millions) of events per second, it is the right tool. If a future deployment needs to scale beyond what SQLite handles, that is a migration decision -- not a design-time concern.

**Alternative considered**: Append-only log files with grep-based search. Rejected because structured queries ("show me all write operations to device X in the last hour") are essential for the dashboard and for baseline computation.

### D5: Commercial Tool Integration Points

**Decision**: Each commercial tool category has a defined integration path that connects to the telemetry stack at specific layers.

#### Cisco CyberVision

| Integration Point | Layer | Mechanism |
|------------------|-------|-----------|
| Traffic feed | L1 | CyberVision sensor receives SPAN/TAP traffic from Docker network mirror; same mechanism as passive capture |
| Asset correlation | L2 | Dashboard displays CyberVision asset classification alongside observed inventory (API or manual import) |
| Vulnerability data | L2 | CyberVision CVE mappings displayed on process view equipment symbols |
| Baseline comparison | L3 | Side-by-side: "what our baseline learned" vs. "what CyberVision's baseline learned" |

**CyberVision deployment model in the simulator**: CyberVision sensor container joins the same Docker network as the monitoring module. It receives identical traffic. The educational exercise is comparing the monitor's hand-built telemetry with CyberVision's automatic analysis and asking: "What did CyberVision find that we missed? What did we find that CyberVision missed?"

**CyberVision-specific teaching points**:
- CyberVision uses Cisco's Industrial Protocol Library for DPI -- it understands Modbus, EtherNet/IP, PROFINET, and dozens more protocols at the field level
- It maps each device to a Cisco-maintained vulnerability database (different from NVD)
- It visualizes communication patterns as a "network map" -- the simulator already has topology and process views, making the comparison direct
- It integrates with Cisco DNA Center for IT/OT network management convergence -- a relevant architecture pattern for IT engineers to understand

#### Dragos Platform

| Integration Point | Layer | Mechanism |
|------------------|-------|-----------|
| Traffic feed | L1 | Dragos sensor receives SPAN/TAP traffic |
| Threat intelligence | L4 | Dragos threat groups (CHERNOVITE, VOLTZITE, KAMACITE) correlated against observed activity |
| Investigation | L1-L4 | Dragos investigation workbench queries the same event data the dashboard shows |

**Dragos-specific teaching points**:
- Dragos names threat groups targeting ICS/OT specifically (not generic APT numbers)
- Its detection signatures are tuned for OT protocols -- it detects Modbus write-after-read-only patterns, not just generic network anomalies
- The "Neighborhood Keeper" program provides anonymized cross-customer threat sharing -- a concept IT engineers may recognize from ISAC/ISAO membership

#### Blastwave BlastShield

| Integration Point | Layer | Mechanism |
|------------------|-------|-----------|
| Network segmentation | L5 | BlastShield gateway containers inserted at network boundaries defined in ADR-005 D3 |
| Traffic encryption | L1 | Post-BlastShield, passive capture sees encrypted tunnels -- teaching the visibility vs. security tradeoff |
| Access control | L5 | Identity-based policies replace network-based (IP/port) access control |

**BlastShield-specific teaching points**:
- Protocol transparency: Modbus traffic traverses the BlastShield tunnel unchanged. The PLC does not know it is being protected. This is critical in OT where device firmware cannot be modified.
- Network cloaking: Devices behind a BlastShield gateway do not respond to unsolicited traffic. Port scans return nothing. For an IT engineer accustomed to nmap, this is a dramatic demonstration.
- The visibility tradeoff: After deploying BlastShield, the passive listener can no longer see Modbus frames in cleartext. CyberVision/Dragos sensors must be placed *inside* the trust boundary (between gateway and device) to maintain visibility. This is a real architectural decision that IT engineers must understand.

### D6: Beta 0.6 Scope -- Traditional Telemetry Foundation

**Decision**: Beta 0.6 focuses on Layers 1-2 of the telemetry stack plus strengthening Layers 3-4. No commercial tool integration in this milestone.

| Deliverable | Layer | Description |
|-------------|-------|-------------|
| Transaction event model | L1 | Structured Modbus transaction type with all fields from D2 |
| Active transaction logging | L1 | Poller emits transaction events for every Modbus read/write cycle |
| SQLite event store | L1 | Persistent storage with day partitioning and retention policy |
| Transaction log dashboard page | L1 | Filterable table: by device, function code, time range, write-only toggle |
| Communication graph | L2 | Who-talks-to-whom matrix built from transaction events; displayed on dashboard |
| Function code distribution | L2 | Per-device breakdown of function codes observed; histogram on asset detail page |
| Syslog forwarding | L1 | Optional CEF-format syslog output of transaction events to external SIEM |
| Event-driven baseline refactor | L3 | Baseline engine consumes transaction events instead of raw register snapshots |
| Write detection alerts | L4 | Alert on any Modbus write (FC 5, 6, 15, 16) to devices baselined as read-only |
| Scenario 04 Phase A | All | Deploy monitoring scenario leveraging new telemetry capabilities |

**Out of scope for Beta 0.6**: Passive packet capture (requires plant-side traffic generation), commercial tool integration containers, encrypted traffic analysis.

### D7: Syslog and SIEM Integration Format

**Decision**: Transaction events are exported in CEF (Common Event Format) over syslog for compatibility with enterprise SIEM platforms (Splunk, QRadar, Sentinel, Elastic).

CEF format is chosen because:
- It is the lingua franca of SIEM integration
- IT engineers already know how to parse it
- CyberVision, Dragos, and most OT tools also emit CEF
- It provides a structured key-value format that maps naturally to Modbus transaction fields

Example CEF event:
```
CEF:0|OTSimulator|Monitor|0.6|modbus-write|Modbus Write Operation|7|src=10.10.30.10 spt=5020 dst=10.10.30.100 dpt=50200 cs1=FC16 cs1Label=FunctionCode cn1=5 cn1Label=RegisterStart cn2=1 cn2Label=RegisterCount cs2=42 cs2Label=Value msg=Write Single Register to holding register 5
```

**Teaching value**: An IT engineer who has never seen OT traffic in their SIEM can import these events alongside their existing IT log sources. The exercise of writing correlation rules that span IT and OT events is a core SOC analyst skill for converged environments.

### D8: Future Milestones (Not Scoped, Directional Only)

| Milestone | Focus | Key Deliverables |
|-----------|-------|-----------------|
| Beta 0.7 | Passive Capture + CyberVision | Packet capture goroutine, Modbus TCP frame parser, CyberVision sensor container, comparison exercises |
| Beta 0.8 | Dragos Integration | Dragos sensor container, threat intel correlation, investigation workflow scenario |
| Beta 0.9 | BlastShield Integration | Gateway containers at ADR-005 insertion points, before/after visibility comparison, encrypted tunnel analysis |
| Beta 1.0 | Full Stack | All five layers operational, all three commercial tools integrated, capstone scenario using all tools together |

These are directional targets, not commitments. Each milestone will be scoped through its own milestone spec and SOW backlog when reached.

---

## Consequences

### Positive
- Engineers build understanding from the ground up -- no black-box tool dependence
- The five-layer model provides a mental framework that maps to both manual and commercial approaches
- CEF/syslog integration uses skills IT engineers already have
- SQLite persistence is zero-dependency and simple to operate
- Each commercial tool has a clearly defined integration point and educational purpose
- CyberVision fills the "passive asset inventory" gap that Dragos and BlastShield don't address
- The active vs. passive capture distinction teaches a fundamental monitoring concept

### Negative
- Building traditional telemetry before commercial tools delays the "wow factor" of Dragos/BlastShield
- SQLite limits concurrent write throughput (acceptable for educational scale)
- CEF format is verbose and somewhat dated (but universally supported)
- Three commercial tool integrations multiply the testing and documentation burden

### Risks
- **Risk**: CyberVision requires Cisco hardware or virtual appliance licensing
- **Mitigation**: Design the integration point generically (SPAN traffic feed + optional API). If CyberVision licensing is unavailable, the same integration point serves Dragos or any passive DPI tool.
- **Risk**: Engineers may skip the manual telemetry layers and jump to commercial tools
- **Mitigation**: Scenario 04 (Defense) requires demonstrating manual monitoring before introducing commercial tools. The comparison exercise only works if both sides are populated.
- **Risk**: SQLite event store grows unbounded on long-running labs
- **Mitigation**: Day-partitioned tables with configurable retention. Default 7-day retention auto-prunes old data.
- **Risk**: Passive capture in Docker may not perfectly replicate SPAN port behavior
- **Mitigation**: Document the differences explicitly. Docker bridges forward all frames to all containers on the bridge; a real SPAN port mirrors selected ports only. This is more permissive, not less.

---

## References
- ADR-005: Monitoring Integration Architecture
- ADR-007: Educational Scenario Framework
- ADR-009: Design Layer and Composable Environments
- Cisco CyberVision: https://www.cisco.com/site/us/en/products/security/industrial-threat-defense/cyber-vision/index.html
- Dragos Platform: https://www.dragos.com/platform/
- Blastwave BlastShield: https://www.blastwave.com/blastshield
- CEF Format Specification: ArcSight Common Event Format v25
- SQLite WAL Mode: https://www.sqlite.org/wal.html
