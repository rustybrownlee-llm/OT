# Milestone: Beta 0.3

**Target**: Deploy passive Modbus monitoring across both OT environments. Discover devices, establish behavioral baselines, detect anomalies, and present findings on a security analyst dashboard. This completes the three-layer architecture (design → plant → monitoring) and makes the simulator useful for OT security training.

## Definition of Done

An engineer can:

1. Run `docker compose --profile water --profile pipeline up` and have the monitor automatically discover all 12 device endpoints across both environments
2. Open `http://localhost:8090` and see a live asset inventory grouped by environment (water/mfg vs pipeline)
3. View real-time register values for any discovered device, with polling updates every 2 seconds via HTMX
4. Wait for the baseline learning period (configurable, default 5 minutes) and see baseline status transition from "learning" to "established"
5. Write a rogue value to a writable register (e.g., coil write via mbpoll) and see an anomaly alert appear on the dashboard within one polling cycle
6. Query the alert API (`GET http://localhost:8091/api/alerts`) and receive a JSON array of anomaly events with timestamps, severity, device, and description
7. Observe different monitoring challenges between environments: water/mfg has 7 endpoints across Purdue levels; pipeline has 4 endpoints on a flat LAN plus 1 behind a serial gateway
8. Browse the Design Library pages to view device atoms, network atoms, and environment definitions as syntax-highlighted YAML -- understanding what each device exposes before looking at live data
9. Click from a discovered device in the asset inventory to its design-layer device atom, seeing the full register map specification alongside live polled values
10. Follow Scenario 01 (Discovery) using the monitoring dashboard instead of manual mbpoll commands
11. Read Scenario 02 (Assessment) and understand what vulnerabilities exist based on monitoring observations

## What Beta 0.3 Is NOT

- No Blastwave/BlastShield integration (insertion points designed in ADR-005 D3, awaiting trial license)
- No Dragos integration (requires commercial license and sensor appliance)
- No packet capture or deep packet inspection (active Modbus polling only -- passive PCAP is a future phase)
- No historical data persistence (in-memory ring buffers only -- no database)
- No cross-environment correlation (each environment monitored independently)
- No attack simulation tools (Scenario 03 deferred to Beta 0.4)
- No defense deployment exercises (Scenario 04 deferred to Beta 0.4)
- No new OT protocols (Modbus TCP only, consistent with Beta 0.1/0.2)

## Monitoring Architecture

Per ADR-005 D4, the monitoring module MUST NOT import plant packages. All data comes from network observation: Modbus TCP polling, HTTP responses, ICMP. The monitor discovers the environment the same way a real security tool would.

### Network Attachment

The monitor container joins multiple Docker networks to observe both environments:

| Network | IP Address | Purpose |
|---------|-----------|---------|
| ot-wt-level3 | 10.10.30.20 | SCADA network -- access to water/mfg PLCs and gateway |
| ot-ps-station-lan | 10.20.1.20 | Pipeline station LAN -- access to pipeline devices |
| ot-cross-plant | (DHCP) | Cross-plant visibility |

**Teaching point**: In a real facility, a monitoring tool on the SCADA network (Level 3) can poll Level 1 PLCs through the managed switches. On the pipeline station, all devices are on one flat LAN. The monitor's network position determines what it can see.

### Data Flow

```
[Plant Containers]  ---Modbus TCP--->  [Monitor]  ---HTTP--->  [Dashboard :8090]
                                          |                    [Alert API :8091]
                                          v
                                   [In-Memory Store]
                                   - Asset inventory
                                   - Register time-series (ring buffer)
                                   - Behavioral baselines
                                   - Anomaly alerts
```

### Technology Stack

| Technology | Purpose | Notes |
|-----------|---------|-------|
| simonvetter/modbus | Modbus TCP client for polling | Same library as plant; validated in POC-001 |
| chi v5 | HTTP routing for dashboard and API | Lightweight, stdlib-compatible (per CLAUDE.md) |
| Bootstrap 5 | Dashboard CSS framework | Via CDN, no build step |
| HTMX | Live register updates | Via CDN, SSE or polling for real-time display |
| go:embed | Static asset embedding | Single binary deployment |

## SOW Backlog

### Ordered by Dependency

| # | SOW | Title | Dependencies | Status | Notes |
|---|-----|-------|-------------|--------|-------|
| 1 | SOW-012.0 | Monitoring Core: Device Discovery and Register Polling | None | Not Started | Config parsing, Modbus TCP polling client, active device discovery, register enumeration, asset inventory model, HTTP health endpoints. Resolves TD-011, TD-012, TD-020. Docker Compose network changes. |
| 2 | SOW-013.0 | Behavioral Baseline and Anomaly Detection | SOW-012.0 | Not Started | Time-series ring buffer, baseline learning period, anomaly rules (value out of range, unexpected writes, new devices, response time changes). Alert model with severity. Alert API on port 8091. |
| 3 | SOW-014.0 | Monitoring Dashboard and Design Library | SOW-012.0 | Not Started | Web UI on port 8090. Asset inventory view grouped by environment. Live register values via HTMX. Baseline status indicators. Alert timeline. Device detail drilldown. **Design Library pages**: read-only YAML viewers for device atoms, network atoms, and environment definitions with syntax highlighting. Cross-links from discovered assets to their design-layer specs. chi v5 + Bootstrap 5 + HTMX + go:embed. Volume mount to `design/` directory (read-only). |
| 4 | SOW-015.0 | Scenario Integration | SOW-013.0, SOW-014.0 | Not Started | Scenario 01 (Discovery) expansion with monitoring dashboard walkthrough. Scenario 02 (Assessment) full draft. Scenario content lives in `scenarios/` directory. |

### Dependency Graph

```
SOW-012.0 (Monitoring Core) ---------> SOW-014.0 (Dashboard + Design Library)
  |                                        |
  v                                        v
SOW-013.0 (Baseline & Anomaly) -----> SOW-015.0 (Scenarios)
```

## Design Library (YAML Viewer)

The dashboard includes read-only pages for browsing the design layer -- device atoms, network atoms, and environment definitions. This gives engineers reference context alongside live monitoring data.

### Pages

| Page | Route | Content |
|------|-------|---------|
| Device Library | `/design/devices` | List all device atom YAMLs. Click to view full YAML with syntax highlighting. |
| Device Detail | `/design/devices/{id}` | Single device atom: metadata, connectivity, register capabilities, all variants. |
| Network Library | `/design/networks` | List all network atom YAMLs. Click to view. |
| Environment Library | `/design/environments` | List all environments. Click to see environment YAML, placement table, port map. |
| Environment Detail | `/design/environments/{id}` | Single environment: placements, networks, device cross-references. |

### Cross-Links

- Asset inventory rows link to the corresponding device atom page (discovered CompactLogix → `/design/devices/compactlogix-l33er`)
- Environment detail pages link to each device atom and network atom referenced in placements

### ADR-005 D4 Boundary

The design library is a **documentation feature**, not a monitoring data source. The monitor's device discovery and anomaly detection still rely entirely on network observation (Modbus polling). The YAML files provide reference context for the engineer -- "here's what this device is designed to do" alongside "here's what I'm seeing on the wire." This distinction must be explicit in the dashboard UI: design-layer pages are labeled "Reference" and live monitoring pages are labeled "Observed."

### Implementation

- `design/` directory mounted read-only into the monitor container (`./design:/design:ro`)
- Monitor parses YAML files at startup for the library pages but does NOT use them for polling configuration or anomaly rules
- Syntax highlighting via CSS (no JavaScript highlighting library needed -- `<pre>` with YAML-aware CSS classes)

## Device Discovery Behavior

The monitor reads its configuration to know which Modbus TCP endpoints exist, then actively polls each one to build an asset inventory.

### Discovery Process

1. **Connect**: Open TCP connection to each configured endpoint (IP:port)
2. **Enumerate unit IDs**: For gateway ports, poll unit IDs 1-10 and 247 to discover routed devices
3. **Register scan**: For each responding unit ID, read holding registers and coils to determine register count and addressing mode
4. **Fingerprint**: Measure response time (mean, jitter) to characterize device type
5. **Classify**: Map discovered attributes to known device profiles from the asset inventory

### Polling Loop

After discovery, the monitor enters a continuous polling loop:

1. Read all holding registers from each discovered device
2. Read all coils from each discovered device
3. Record values with timestamp in the ring buffer
4. Check values against baseline (if established)
5. Generate alerts for anomalies
6. Sleep for poll interval (default 2 seconds)

## Anomaly Detection Rules

| Rule | Trigger | Severity | Example |
|------|---------|----------|---------|
| Value out of range | Register value exceeds baseline mean +/- 3 standard deviations | Warning | Water intake flow suddenly doubles |
| Unexpected write | Coil or holding register changes value unexpectedly | High | Meter run disabled without operator action |
| New device | Previously unknown unit ID responds | Critical | Rogue device on network |
| Device offline | Previously responding device stops responding | High | PLC communication failure |
| Response time anomaly | Response time exceeds baseline mean + 3 standard deviations | Warning | Network congestion or device under load |

## Port Assignments

No new host ports. Existing allocations from Beta 0.1/0.2:

| Port | Service | Notes |
|------|---------|-------|
| 5020-5022 | Water treatment PLCs | Unchanged |
| 5030 | Manufacturing gateway | Unchanged |
| 5040-5043 | Pipeline station devices | Unchanged |
| 8090 | Monitoring dashboard | **Activated** (was allocated but unused) |
| 8091 | Alert API | **Activated** (was allocated but unused) |

## Technical Debt to Address

| ID | Description | Resolution in Beta 0.3 |
|----|-------------|------------------------|
| TD-011 | Monitor config is a stub; not consumed by binary | SOW-012.0: Config parsing implemented |
| TD-012 | Health check only verifies config readable | SOW-012.0: Health check polls at least one Modbus endpoint |
| TD-020 | Pipeline devices unmonitored | SOW-012.0: Monitor joins pipeline network, discovers pipeline devices |

## Technical Debt Created

Expected new debt items from Beta 0.3:

| ID | Description | Notes |
|----|-------------|-------|
| TD-021 | No persistent storage for baselines or alerts | In-memory ring buffer resets on restart. Database deferred. |
| TD-022 | No packet capture / passive monitoring | Active polling only. Passive PCAP deferred to Phase 4+. |
| TD-023 | No authentication on dashboard or API | Acceptable for training tool. Add if deployed beyond localhost. |
| TD-024 | No TLS on monitoring endpoints | Same as TD-023. |

## Open Questions

- Should the monitor discover devices by scanning configured port ranges, or should the config explicitly list every endpoint? (Config-driven is simpler; scanning is more realistic.)
- Should the baseline learning period be wall-clock time or number of polling cycles?
- Should anomaly alerts auto-clear when values return to normal, or require manual acknowledgment?
- Should the dashboard support multiple simultaneous viewers (SSE broadcast), or is single-viewer sufficient for training?

## Architecture Reference

- ADRs: `docs/architecture/decisions/` (ADR-001 through ADR-009)
- Monitoring architecture: ADR-005 (D1-D4)
- Scenario framework: ADR-007
- Design layer: ADR-009
- Protocol priority: ADR-002
