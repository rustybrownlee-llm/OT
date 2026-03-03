# Solution: Scenario 04 Phase A - Deploy Active Monitoring

This document is the complete reference walkthrough and includes expected observations for
all 6 devices. Values that vary with simulation time are shown as ranges. Values that are
fixed (device configuration, topology) are shown exactly.

---

## Prerequisites

Verify both the plant and monitor are running:

```
docker compose --profile water --profile monitor up
```

Verify the plant is responding:

```
mbpoll -t 4 -r 0 -c 1 -1 localhost -p 5020
```

Expected: a value between 12000 and 22000 (intake flow rate, varying).

Verify the monitoring dashboard is accessible:

```
http://localhost:8090
```

Expected: 6 devices listed as Online.

---

## Phase A1: Deploy and Configure Monitoring

### A1-1: Dashboard overview

Navigate to `http://localhost:8090`. Expected:

```
Environment: greenfield-water-mfg
Devices: 6 (all Online)
  - wt-plc-01 (Water Treatment PLC 1 - Intake)       Online
  - wt-plc-02 (Water Treatment PLC 2 - Treatment)    Online
  - wt-plc-03 (Water Treatment PLC 3 - Distribution) Online
  - mfg-gateway-01 (Serial-to-Ethernet Gateway)      Online
  - mfg-plc-01 (Line A Conveyor and Assembly)        Online
  - mfg-plc-02 (Cooling Water System)                Online
```

### A1-2: Transaction event log

Navigate to `http://localhost:8090/events`. Expected event stream (sample, rotating):

```
Time                  Device         FC  FC Name                  Addr  Count  Success  Write
2026-03-02T10:00:00Z  wt-plc-01       3  Read Holding Registers    0      5    yes
2026-03-02T10:00:00Z  wt-plc-01       1  Read Coils                0      4    yes
2026-03-02T10:00:02Z  wt-plc-02       3  Read Holding Registers    0      7    yes
2026-03-02T10:00:02Z  wt-plc-02       1  Read Coils                0      4    yes
2026-03-02T10:00:04Z  wt-plc-03       3  Read Holding Registers    0      5    yes
2026-03-02T10:00:04Z  wt-plc-03       1  Read Coils                0      3    yes
2026-03-02T10:00:06Z  mfg-gateway-01  3  Read Holding Registers    0      9    yes
2026-03-02T10:00:08Z  mfg-plc-01      3  Read Holding Registers    1      7    yes
2026-03-02T10:00:08Z  mfg-plc-01      1  Read Coils                1      4    yes
2026-03-02T10:00:10Z  mfg-plc-02      3  Read Holding Registers    1      7    yes
2026-03-02T10:00:10Z  mfg-plc-02      1  Read Coils                1      4    yes
```

**Observations**:
- FC 3 dominates (all devices respond to holding register reads)
- FC 1 appears for all devices except mfg-gateway-01 (gateway has no coils)
- No Write badges appear during normal monitoring
- All 6 devices rotate through the event table every 10 seconds

### A1-3: Communication matrix (star topology)

Navigate to `http://localhost:8090/comms`. Expected:

```
Source           → Destinations
127.0.0.1        → wt-plc-01, wt-plc-02, wt-plc-03, mfg-gateway-01, mfg-plc-01, mfg-plc-02
```

One source address connects to all 6 endpoints. This is the star topology expected for
active polling. The source address is the monitoring process (127.0.0.1 in a local
deployment, or the monitor container IP in a Docker deployment).

---

## Phase A2: Build a Traffic Baseline

### A2-1: Baseline timing

The learning period completes approximately 5 minutes after the monitor starts polling.
At 150 cycles * 2 seconds = 300 seconds. Serial devices (mfg-plc-01, mfg-plc-02) complete
slightly later if gateway unit ID scanning took extra cycles.

### A2-2: Expected baseline status transitions

Approximate transition order:
1. wt-plc-01, wt-plc-02, wt-plc-03: established at T+5:00 (Ethernet PLCs polled first)
2. mfg-gateway-01: established at T+5:10 (gateway polled after PLCs)
3. mfg-plc-01, mfg-plc-02: established at T+5:30 (serial devices discovered via gateway)

### A2-3 through A2-4: Per-device function code profiles

Expected FC profiles after the learning period:

| Device | Observed FCs | Write FCs | IsWriteTarget |
|--------|-------------|-----------|---------------|
| wt-plc-01 | FC 1, FC 3 | FC 6 (pump speed setpoint) | true |
| wt-plc-02 | FC 1, FC 3 | FC 6 (chemical feed setpoint) | true |
| wt-plc-03 | FC 1, FC 3 | none | false |
| mfg-gateway-01 | FC 3 | none | false |
| mfg-plc-01 | FC 1, FC 3 | FC 6 (conveyor speed setpoint) | true |
| mfg-plc-02 | FC 1, FC 3 | none | false |

**Note**: The write FCs listed for wt-plc-01, wt-plc-02, and mfg-plc-01 reflect the SCADA
setpoint writes that are part of normal plant operations. If your monitoring deployment
does not see these writes (for example, because no SCADA process is running in your test
environment), these devices will be baselined as read-only and `IsWriteTarget` will be
false. Adjust your expected detection behavior accordingly.

**If no write FCs appear for any device during learning**: This is also a valid lab
configuration. The monitor only sends read requests. If no other Modbus client is running,
all 6 devices will be baselined as read-only (`IsWriteTarget=false`). In this case, the
Phase A5 write injection to wt-plc-03 will still trigger both `write_to_readonly` and
`fc_anomaly`. The exercise works either way.

### A2-5: Response time expected ranges

| Device | Expected Mean | Expected Jitter | Notes |
|--------|--------------|----------------|-------|
| wt-plc-01 | ~5ms | ±2ms | Ethernet, 100Mbps |
| wt-plc-02 | ~5ms | ±2ms | Ethernet, 100Mbps |
| wt-plc-03 | ~5ms | ±2ms | Ethernet, 100Mbps |
| mfg-gateway-01 | ~15ms | ±5ms | Ethernet, gateway processing |
| mfg-plc-01 | ~65ms | ±20ms | RS-485 9600 baud, SLC-500 |
| mfg-plc-02 | ~95ms | ±50ms | RS-485 9600 baud, Modicon 984 |

The Modicon 984's high jitter (±50ms) is characteristic of the 1988 processor's variable
interrupt response time. This is not a network fault. The baseline engine learns this
range and will only alert on `response_time_anomaly` if response times fall substantially
outside the learned range.

---

## Phase A3: Analyze Communication Patterns

### A3-1: Device categorization

Based on the FC profiles from Phase A2:

| Device | Category | Basis |
|--------|----------|-------|
| wt-plc-01 | Read-write | SCADA writes pump speed setpoint SC-101 (HR[1]) |
| wt-plc-02 | Read-write | SCADA writes chemical feed setpoint FIC-202 (HR[4]) |
| wt-plc-03 | Read-only | All registers are process measurements; no writable setpoints in active use |
| mfg-gateway-01 | Read-only | Gateway status registers (diagnostics); no writable setpoints |
| mfg-plc-01 | Read-write | SCADA writes conveyor speed setpoint (HR[1]) and cycle time setpoint (HR[6]) |
| mfg-plc-02 | Read-only | Cooling system monitoring registers; no writable setpoints in active use |

### A3-2: Write filter result

Write filter on `/events` during Phases A1-A3: zero write events. The monitoring module
sends only FC 1 and FC 3 requests. No other Modbus client is active in the standard lab
configuration.

### A3-3: FC histogram comparison

| Device | FC 1 (Read Coils) | FC 3 (Read HR) | FC 1 exception |
|--------|-------------------|----------------|----------------|
| wt-plc-01 | Yes, 4 coils | Yes, 5 registers | No |
| wt-plc-02 | Yes, 4 coils | Yes, 7 registers | No |
| wt-plc-03 | Yes, 3 coils | Yes, 5 registers | No |
| mfg-gateway-01 | No (exception 02) | Yes, 9 registers | Yes -- exception 02 |
| mfg-plc-01 | Yes, 4 coils | Yes, 7 registers (addr 1-7) | No |
| mfg-plc-02 | Yes, 4 coils | Yes, 7 registers (addr 1-7) | No |

**mfg-gateway-01 FC 1 exception**: The Moxa NPort 5150 gateway exposes only holding
registers in its own Modbus address space. FC 1 (Read Coils) returns exception 02
(Illegal Data Address) because the gateway has no coils defined. This is correct behavior
and is documented in the device atom profile.

### A3-4: Normal communication pattern summary

```
Source:    127.0.0.1 (monitoring process)
Targets:   6 devices (4 Ethernet, 2 serial via gateway)
Protocol:  Modbus TCP, ports 5020/5021/5022/5030
FCs used:  FC 1 (Read Coils), FC 3 (Read Holding Registers) only
Direction: Monitor initiates; devices respond
Interval:  1 poll cycle per 2 seconds per device
Writes:    Zero during normal operations
```

---

## Phase A4: Configure SIEM Forwarding

### A4-1: monitor.yaml syslog section

```yaml
syslog:
  enabled: true
  target: "localhost:1514"
  protocol: "udp"
  facility: "local0"
  format: "cef"
```

### A4-2 through A4-3: Expected CEF events

Read success event (FC 3, wt-plc-01):

```
<134>CEF:0|OTSimulator|Monitor|0.6|3|Read Holding Registers|1|src=127.0.0.1:52341 dst=127.0.0.1:5020 cs1=Read Holding Registers cs1Label=FunctionCode cn1=0 cn1Label=AddressStart cn2=5 cn2Label=AddressCount cs2=wt-plc-01 cs2Label=DeviceID cs3=greenfield-water-mfg cs3Label=Environment rt=1740873600000 outcome=success
```

Read coils event (FC 1, wt-plc-01):

```
<134>CEF:0|OTSimulator|Monitor|0.6|1|Read Coils|1|src=127.0.0.1:52341 dst=127.0.0.1:5020 cs1=Read Coils cs1Label=FunctionCode cn1=0 cn1Label=AddressStart cn2=4 cn2Label=AddressCount cs2=wt-plc-01 cs2Label=DeviceID cs3=greenfield-water-mfg cs3Label=Environment rt=1740873600000 outcome=success
```

Gateway read (FC 3, mfg-gateway-01, reading 9 registers):

```
<134>CEF:0|OTSimulator|Monitor|0.6|3|Read Holding Registers|1|src=127.0.0.1:52341 dst=127.0.0.1:5030 cs1=Read Holding Registers cs1Label=FunctionCode cn1=0 cn1Label=AddressStart cn2=9 cn2Label=AddressCount cs2=mfg-gateway-01 cs2Label=DeviceID cs3=greenfield-water-mfg cs3Label=Environment rt=1740873600000 outcome=success
```

Serial PLC read (FC 3, mfg-plc-01, one-based addressing):

```
<134>CEF:0|OTSimulator|Monitor|0.6|3|Read Holding Registers|1|src=127.0.0.1:52341 dst=127.0.0.1:5030 cs1=Read Holding Registers cs1Label=FunctionCode cn1=1 cn1Label=AddressStart cn2=7 cn2Label=AddressCount cs2=mfg-plc-01 cs2Label=DeviceID cs3=greenfield-water-mfg cs3Label=Environment rt=1740873600000 outcome=success
```

Note that mfg-plc-01 and mfg-plc-02 show `dst=127.0.0.1:5030` (the gateway port) because
they are accessed via the Moxa NPort 5150. The `cs2` field identifies the actual device.

### A4-4: Severity distribution in the live feed

Expected during Phases A1-A4 (no writes):
- `severity=1`: overwhelming majority (all read successes)
- `severity=5`: occasional (read exceptions, e.g., FC 1 on mfg-gateway-01 returns exception 02)
- `severity=7`: zero (no write operations)
- `severity=3`: zero (no FC 43 diagnostic requests in this exercise)

---

## Phase A5: Detect Unauthorized Activity

### A5-1: Pre-injection alert state

Expected before injection:
- Possible `value_out_of_range` alerts for registers with high sensor noise
  (e.g., raw_water_turbidity at wt-plc-01, which has natural variation)
- Possible `response_time_anomaly` for mfg-plc-02 if a poll cycle exceeded the baseline range
- No `write_to_readonly` alerts
- No `fc_anomaly` alerts

### A5-2: Write injection command

```
mbpoll -t 4 -r 2 -c 1 -1 localhost -p 5022 -- 9999
```

Expected output:

```
Written 1 registers at address 2 on server localhost:5022
```

This writes value 9999 to holding register 2 (distribution_pressure) on wt-plc-03. In
engineering units, this represents approximately 1.5 MPa -- above the normal operating
range for distribution pressure (typical: 200-450 kPa, encoded as ~13000-29000 in 16-bit
scaled representation). Value 9999 is within the 16-bit unsigned range (0-32767) so the
write will succeed without triggering a hardware exception.

### A5-3 through A5-4: Expected alerts after injection

**Alert 1: write_to_readonly**

```json
{
  "alert_id": "write_to_readonly:wt-plc-03:-1",
  "rule_id": "write_to_readonly",
  "severity": "critical",
  "device_id": "wt-plc-03",
  "description": "Write operation (FC 6 Write Single Register, succeeded) to device wt-plc-03, which had no write function codes observed during the learning period (IsWriteTarget=false).",
  "first_seen": "2026-03-02T10:05:23Z",
  "last_seen": "2026-03-02T10:05:23Z"
}
```

**Alert 2: fc_anomaly**

```json
{
  "alert_id": "fc_anomaly:wt-plc-03:-1",
  "rule_id": "fc_anomaly",
  "severity": "high",
  "device_id": "wt-plc-03",
  "description": "Unexpected function code FC 6 (Write Single Register) observed for device wt-plc-03. Baseline observed FCs: [1, 3]. FC 6 was not present during the learning period.",
  "first_seen": "2026-03-02T10:05:23Z",
  "last_seen": "2026-03-02T10:05:23Z"
}
```

### A5-5: Expected write event in CEF syslog feed

```
<130>CEF:0|OTSimulator|Monitor|0.6|6|Write Single Register|7|src=127.0.0.1:54201 dst=127.0.0.1:5022 cs1=Write Single Register cs1Label=FunctionCode cn1=2 cn1Label=AddressStart cn2=1 cn2Label=AddressCount cs2=wt-plc-03 cs2Label=DeviceID cs3=greenfield-water-mfg cs3Label=Environment rt=1740873923000 outcome=success
```

Note the priority `<130>` (16*8+2=130, syslog Critical). This is the first `<130>` event
in the feed since monitor startup. It stands out immediately from the steady stream of
`<134>` (read success) events.

### A5-6: new_source alert absence explained

No `new_source` alert fires. Both the monitoring module and the mbpoll command used the
same source IP (127.0.0.1 in the local lab). The monitor already observed this source
address during the learning period (all monitoring poll traffic originated from 127.0.0.1).

`new_source` becomes a high-value detection rule in Phase B (passive capture): when the
SPAN port captures all traffic on the manufacturing network, Modbus requests from
engineering workstations, vendor laptops, or unauthorized sources will show new source
addresses not seen during the learning period.

---

## Conceptual Questions: Reference Answers

**SC-CON-01: Why do behavioral baselines outperform signatures in OT?**

OT environments are highly deterministic: the same devices communicate with the same
SCADA system using the same function codes at the same intervals every day. Attackers
targeting OT increasingly use the legitimate protocol (Modbus) as the attack vector --
there is no malware binary to hash-match, no exploit string to detect. Behavioral deviation
detection catches novel attacks that have no known signature because any deviation from
the learned normal is anomalous by definition. Signatures require knowing the attack in
advance; baselines require only knowing what "normal" looks like.

**SC-CON-02: One capability passive capture provides that active polling cannot:**

Visibility into traffic from sources other than the monitoring process. Active polling
makes the monitor the only Modbus client; all traffic on the network appears to originate
from one source. Passive capture (TAP/SPAN) copies all network traffic regardless of
source, revealing the complete communication graph: SCADA server polls, engineering
workstation connections, inter-device communications, and unauthorized sources. The
`new_source` alert fires meaningfully only with passive capture.

**SC-CON-03: Two things commercial tools automate from this manual process:**

Any two of: automated asset inventory built from passive traffic observation; automated
baseline learning with 30-90 day characterization periods requiring no operator
interaction; threat intelligence matching against known ICS adversary TTPs (TRITON,
CRASHOVERRIDE, STUXNET); built-in detection rules for common OT attack patterns; pre-built
SIEM connector integrations (Splunk, Elastic, QRadar).

**SC-CON-04: Why learn the manual layer before deploying commercial tools:**

Automated tools fail in predictable ways. When Dragos fires an alert at 2:00 AM, the
engineer on call must determine whether it is a real attack or a false positive. That
determination requires understanding what the tool is doing -- how the baseline was built,
what FCs are normal for each device, what the expected source addresses are. An engineer
who has never built a behavioral baseline by hand cannot evaluate whether the automated
baseline is correct, cannot recognize a misconfigured learning period, and cannot
troubleshoot a false positive under pressure.
