# Milestone: Beta 0.6

**Target**: Build the traditional network telemetry foundation that IT engineers use before deploying commercial OT security tools. An engineer monitoring the simulated environment should be able to see every Modbus transaction as a structured event, understand who is talking to whom, detect unauthorized write operations, and forward events to an external SIEM -- all using skills that transfer directly from IT security operations.

**Implements**: ADR-011 (Monitoring Telemetry Stack) -- Layers 1-2 with L3/L4 strengthening

## Definition of Done

An engineer can:

1. Open `http://localhost:8090/events` and see a live-updating table of Modbus transaction events showing timestamp, source, destination, function code, register address, value, and response time
2. Filter the event table by device, function code, time range, or write-only operations to isolate suspicious activity
3. See every Modbus write operation (FC 5, 6, 15, 16) highlighted in the event table as a distinct category, because writes are the highest-risk operations in an OT environment
4. Open `http://localhost:8090/comms` and see a communication graph (who-talks-to-whom matrix) built from observed transactions, showing which devices communicate and how frequently
5. View a per-device function code distribution histogram on the asset detail page, showing the breakdown of read vs. write vs. diagnostic function codes observed for that device
6. See transaction events persist across monitor restarts (SQLite storage), with the event log retaining 7 days of history by default
7. Configure syslog forwarding in `monitor.yaml` and see Modbus transaction events appear in CEF format on an external syslog receiver, ready for import into Splunk, QRadar, or Elastic
8. See the baseline engine consume structured transaction events instead of raw register snapshots, with the same anomaly detection quality (zero regression)
9. See a new alert type: "Write to read-only device" -- triggered when any Modbus write operation targets a device that was baselined as read-only during the learning period
10. Open Scenario 04 (Deploy Passive Monitoring) Phase A and follow a guided exercise that uses the new telemetry capabilities to build a traffic baseline, identify communication patterns, and detect a simulated unauthorized write
11. Understand from the scenario materials why this manual telemetry layer exists and what commercial tools (Cisco CyberVision, Dragos) would automate or enhance

## What Beta 0.6 Is NOT

- No passive packet capture (requires plant-side inter-device traffic generation -- deferred to Beta 0.7)
- No Cisco CyberVision integration (Beta 0.7)
- No Dragos integration (Beta 0.8)
- No Blastwave BlastShield integration (Beta 0.9)
- No OPC UA or DNP3 protocol support (Modbus TCP only)
- No real-time streaming dashboard (HTMX polling is sufficient; WebSocket upgrade deferred)
- No multi-instance federation (single monitor process)
- No encrypted transport for syslog (plaintext UDP/TCP CEF -- acceptable for lab environments)
- No packet-level deep inspection (transaction-level events from the poller, not raw frame parsing)
- No cross-device correlation rules (per-device anomaly detection only -- correlation is a future milestone)
- No STIX/TAXII threat intelligence integration (Dragos milestone)

## Transaction Event Model

### Modbus Transaction Event

Every Modbus read/write cycle produces a structured event:

```go
type TransactionEvent struct {
    ID            string    // UUID v4
    Timestamp     time.Time // when the transaction completed
    SrcAddr       string    // source IP:port (monitor's address)
    DstAddr       string    // destination IP:port (device endpoint)
    UnitID        uint8     // Modbus unit/slave ID
    FunctionCode  uint8     // Modbus function code (1-127)
    FunctionName  string    // human-readable name ("Read Holding Registers")
    RegisterStart uint16    // starting register address
    RegisterCount uint16    // number of registers in request
    IsWrite       bool      // true for FC 5, 6, 15, 16
    Success       bool      // transaction completed without error
    ExceptionCode uint8     // Modbus exception code if !Success
    ResponseTimeUs int64    // round-trip time in microseconds
    DeviceID      string    // resolved asset ID (ip:port:unit_id)
    EnvID         string    // environment ID if known from config
}
```

### Write Event Extension

Write transactions (FC 5, 6, 15, 16) include the written values:

```go
type WriteDetail struct {
    Values     []uint16  // register values written (FC 6, 16)
    CoilValues []bool    // coil values written (FC 5, 15)
}
```

### Function Code Reference

The event model uses canonical Modbus function code names:

| FC | Name | Category | Risk Level |
|----|------|----------|------------|
| 1 | Read Coils | Read | Low |
| 2 | Read Discrete Inputs | Read | Low |
| 3 | Read Holding Registers | Read | Low |
| 4 | Read Input Registers | Read | Low |
| 5 | Write Single Coil | Write | High |
| 6 | Write Single Register | Write | High |
| 15 | Write Multiple Coils | Write | High |
| 16 | Write Multiple Registers | Write | High |
| 43 | Read Device Identification | Diagnostic | Medium |

Teaching point: In IT, reads are safe and writes are dangerous. In OT, the same principle applies but the stakes are physical. FC 6 writing value 0 to a chlorine dosing register has the same technical weight as FC 3 reading that register -- but radically different consequences.

## SQLite Event Store

### Schema

```sql
CREATE TABLE IF NOT EXISTS events (
    id          TEXT PRIMARY KEY,
    timestamp   TEXT NOT NULL,          -- RFC 3339 format
    src_addr    TEXT NOT NULL,
    dst_addr    TEXT NOT NULL,
    unit_id     INTEGER NOT NULL,
    func_code   INTEGER NOT NULL,
    func_name   TEXT NOT NULL,
    reg_start   INTEGER,
    reg_count   INTEGER,
    is_write    INTEGER NOT NULL DEFAULT 0,
    success     INTEGER NOT NULL DEFAULT 1,
    exception   INTEGER,
    response_us INTEGER,
    device_id   TEXT,
    env_id      TEXT,
    -- Write details stored as JSON when is_write=1
    write_values TEXT
);

CREATE INDEX idx_events_timestamp ON events(timestamp);
CREATE INDEX idx_events_device ON events(device_id);
CREATE INDEX idx_events_write ON events(is_write) WHERE is_write = 1;
CREATE INDEX idx_events_func ON events(func_code);
```

### Retention

- Default retention: 7 days (configurable via `event_retention_days` in monitor.yaml)
- Pruning runs once per hour, deleting events older than the retention window
- Database uses WAL mode for concurrent read/write from poller and dashboard
- Database file location: `monitoring/data/events.db` (configurable via `event_db_path`)

### Capacity Estimate

At 2-second polling with 12 devices, ~6 events/second = ~520K events/day = ~3.6M events/week. At ~200 bytes per row, that is ~720MB for 7 days. Acceptable for an educational tool; configurable if needed.

## CEF Syslog Format

### Event Mapping

```
CEF:0|OTSimulator|Monitor|0.6|{func_code}|{func_name}|{severity}|
  src={src_addr} dst={dst_addr}
  cs1={func_name} cs1Label=FunctionCode
  cn1={addr_start} cn1Label=AddressStart
  cn2={addr_count} cn2Label=AddressCount
  cs2={device_id} cs2Label=DeviceID
  cs3={env_id} cs3Label=Environment
  rt={timestamp_ms}
  outcome={success|failure}
```

Severity mapping:
- Write operations: 7 (High)
- Diagnostic operations (FC 43): 3 (Medium-Low)
- Read failures (exception codes): 5 (Medium)
- Read successes: 1 (Low/Informational)

### Configuration

```yaml
# monitor.yaml additions
syslog:
  enabled: false              # default off
  target: "localhost:514"     # syslog receiver address (host:port, not URL)
  protocol: "udp"            # udp or tcp
  facility: "local0"         # syslog facility
  format: "cef"              # cef (only format for Beta 0.6)
```

## Communication Graph

### Data Structure

The communication graph is built from transaction events, not live traffic:

```go
type CommGraph struct {
    Nodes []CommNode
    Edges []CommEdge
}

type CommNode struct {
    DeviceID    string
    Label       string  // human-readable from design library
    TotalEvents int64
    LastSeen    time.Time
}

type CommEdge struct {
    Source       string  // source device ID
    Target       string  // target device ID
    EventCount   int64
    WriteCount   int64   // subset that are write operations
    LastEvent    time.Time
    FunctionCodes []uint8 // distinct function codes observed
}
```

### Dashboard Rendering

The communication graph renders as a simple matrix/table on the `/comms` page -- not a force-directed graph. Rows and columns are devices; cells show event count with write counts highlighted. This is the format IT engineers see in firewall log analysis tools and is immediately familiar.

A visual graph (D3/SVG force-directed) is a future enhancement -- the matrix is sufficient for the teaching goal and avoids introducing a JavaScript dependency.

## Baseline Engine Refactor

### Current State

The baseline engine in `baseline/baseline.go` consumes `DeviceSnapshot` structs containing raw `[]uint16` holding registers and `[]bool` coils. Statistics are computed per-register using Welford's algorithm.

### Target State

The baseline engine additionally consumes `TransactionEvent` structs to track:
- **Function code baseline**: Which function codes are normal for each device (e.g., device X only receives FC 3 and FC 1 during learning → any FC 6/16 after learning triggers alert)
- **Polling interval baseline**: Expected time between transactions per device (e.g., device X is polled every 2.0s ± 0.1s → a gap of 30s or a burst of 10 requests in 1s is anomalous)
- **Source baseline**: Which source addresses communicate with each device (e.g., only 10.10.30.100 talks to device X → a new source 192.168.1.50 triggers alert)

The existing register value baseline (mean/stddev/min/max) is unchanged. The new event-driven baselines are additive.

### New Alert Rules

| Rule ID | Name | Trigger | Severity |
|---------|------|---------|----------|
| `write_to_readonly` | Write to Read-Only Device | FC 5/6/15/16 to a device that received only read function codes during learning | Critical |
| `new_source` | Unauthorized Communication Source | Transaction from a source address not seen during learning period | High |
| `fc_anomaly` | Unusual Function Code | Function code not observed during learning period for this device | High |
| `poll_gap` | Polling Interval Anomaly | Time since last transaction exceeds 3x the learned mean interval | Warning |

## Dashboard Pages

### New Pages

| Path | Title | Content |
|------|-------|---------|
| `/events` | Transaction Log | Filterable table of Modbus transaction events with live HTMX updates |
| `/comms` | Communications | Device communication matrix built from transaction events |

### Modified Pages

| Path | Change |
|------|--------|
| `/assets/{id}` | Add function code distribution histogram (bar chart showing FC breakdown) |
| Nav bar | Add "Events" and "Comms" links in the "Observed" section |

### Event Table Columns

| Column | Content | Filterable |
|--------|---------|------------|
| Time | Timestamp (HH:MM:SS.mmm) | Time range picker |
| Device | Device ID with link to asset detail | Dropdown |
| FC | Function code number + name | Dropdown |
| Register | Start address + count | No |
| Write | Highlighted badge if is_write=true | Toggle |
| Value | Written value (writes only) or "—" (reads) | No |
| Status | Success/failure with exception code | Toggle |
| RTT | Response time in ms | No |

## SOW Backlog

### Ordered by Dependency

| # | SOW | Title | Dependencies | Status | Notes |
|---|-----|-------|-------------|--------|-------|
| 1 | SOW-027.0 | Transaction Event Model and Store | None | Complete | Define TransactionEvent type, SQLite schema, event store package with insert/query/prune, retention configuration |
| 2 | SOW-028.0 | Poller Transaction Logging | SOW-027.0 | Complete | Instrument the poller to emit TransactionEvent for every Modbus read/write cycle; wire event store into monitor startup |
| 3 | SOW-029.0 | Transaction Log Dashboard Page | SOW-028.0 | Complete | /events route with filterable HTMX table, FC/device dropdowns, write-only toggle, 4x/0x address notation |
| 4 | SOW-030.0 | Communication Graph and Function Code Histograms | SOW-028.0 | Complete | /comms route with device matrix, per-device FC distribution histogram, DistinctFCs per device |
| 5 | SOW-031.0 | CEF Syslog Forwarding | SOW-027.0 | Complete | Syslog emitter package, CEF formatter, 4-tier severity, UDP/TCP, makeEventHook helper |
| 6 | SOW-032.0 | Event-Driven Baseline Extensions | SOW-028.0 | In Progress | write_to_readonly, new_source, fc_anomaly, poll_gap alert rules; RecordEvents method |
| 7 | SOW-033.0 | Scenario 04 Phase A: Deploy Monitoring | SOW-029.0, SOW-030.0, SOW-032.0 | In Progress | Guided exercise: deploy, baseline, analyze, SIEM forward, detect |

### Dependency Graph

```
SOW-027.0 (Event Model + Store) ──────> SOW-028.0 (Poller Logging)
  │                                        │
  │                                        ├──> SOW-029.0 (Event Dashboard)
  │                                        │
  │                                        ├──> SOW-030.0 (Comms Graph + FC Histograms)
  │                                        │
  │                                        └──> SOW-032.0 (Baseline Extensions)
  │                                                            │
  ├──> SOW-031.0 (CEF Syslog)                                │
  │                                                            v
  └────────────────────────────────────────> SOW-033.0 (Scenario 04A)
```

Note: SOW-031.0 (syslog) depends on the event model (SOW-027.0) and the EventHook pipeline (SOW-028.0). It chains into the EventHook alongside store.InsertBatch() for synchronous forwarding (not polling the store). This allows syslog development to proceed in parallel with dashboard work.

## Configuration Additions

```yaml
# monitor.yaml -- new fields for Beta 0.6
event_db_path: "data/events.db"     # SQLite database file path (relative to working directory)
event_retention_days: 7              # days to retain transaction events before pruning

syslog:
  enabled: false
  target: "udp://localhost:514"
  protocol: "udp"                    # udp | tcp
  facility: "local0"
  format: "cef"
```

## Port Assignments

No new ports. All new functionality is served through existing infrastructure:

| Port | Service | Change |
|------|---------|--------|
| 8090 | Monitoring dashboard | New `/events` and `/comms` routes; modified asset detail page |
| 8091 | Alert API | New alert rules (write-to-readonly, new-source, fc-anomaly, poll-gap) |

## Technical Debt to Address

| ID | Description | Resolution in Beta 0.6 |
|----|-------------|------------------------|
| td-alert-021 | Alerts are in-memory only, lost on restart | Transaction events are now persisted in SQLite. Alerts themselves remain in-memory (resolving alert persistence is deferred), but the event trail survives restarts. |
| td-baseline-029 | Fixed baseline after learning period | No change to the learning/established lifecycle. New event-driven baselines follow the same pattern. Sliding window baselines remain deferred. |
| td-poller-027 | Sequential polling limits scalability | No change. Sequential polling is acceptable for current device count and produces a clean event stream. |

## Technical Debt Created

| ID | Description | Notes |
|----|-------------|-------|
| td-events-060 | SQLite is single-writer; concurrent dashboard queries may contend with poller inserts | WAL mode mitigates this. Acceptable for educational scale. If contention becomes measurable, add a write-ahead buffer goroutine. |
| td-events-061 | CEF syslog is plaintext UDP/TCP; no TLS | Acceptable for lab environment. TLS syslog (RFC 5425) deferred to production hardening milestone. |
| td-events-062 | Communication graph is rebuilt from full event query on each page load | For 7 days of data (~3.6M rows), this query may take 1-2 seconds. Add materialized view or periodic cache if latency is noticeable. |
| td-events-063 | No event export (CSV, JSON download) from dashboard | Engineers who want to analyze events in external tools must use syslog forwarding or direct SQLite access. Dashboard export button deferred. |
| td-events-064 | Scenario 04 Phase A only; Phases B-D (commercial tool deployment) deferred to Beta 0.7-0.9 | Scenario 04 is designed as a multi-phase exercise spanning multiple milestones, one phase per commercial tool integration. |
| td-events-065 | Write detail values are stored as JSON text in SQLite | Avoids a separate join table for write values. Query performance is adequate for educational scale. Structured extraction requires JSON1 extension (bundled with modern SQLite). |

## Architecture Reference

- ADR-005: Monitoring Integration Architecture (D4 boundary -- all telemetry from network observation)
- ADR-007: Educational Scenario Framework (Scenario 04 Phase A)
- ADR-009: Design Layer and Composable Environments (event store joins the monitoring module)
- ADR-011: Monitoring Telemetry Stack (Layers 1-2 scope, Beta 0.6 deliverables)
