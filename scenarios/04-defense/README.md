# Scenario 04: OT Defense and Security Overlay Deployment

**Phase**: Defense -- Phase A (Active Monitoring)
**Environment**: greenfield-water-mfg
**Difficulty**: Intermediate
**Estimated Time**: 90-120 minutes
**Prerequisites**: Scenarios 01 and 02 completed, monitoring dashboard running at `http://localhost:8090`

---

## Background

You have completed asset discovery and vulnerability assessment for Greenfield Water and
Manufacturing. The facility manager has accepted the risk findings and authorized deployment of
a security monitoring overlay. Your engagement now moves from observation to detection.

This exercise deploys and configures the OT monitoring telemetry stack, builds a behavioral
baseline of normal Modbus TCP traffic, and uses that baseline to detect a simulated unauthorized
write. You are the first security practitioner to bring active monitoring to this facility.

**Active monitoring vs. passive capture**: This scenario uses active polling -- the monitoring
tool sends Modbus read requests to each device and records the responses. This is fundamentally
different from passive capture, where a TAP or SPAN port copies existing traffic without
injecting any packets of its own. The distinction matters:

- Active polling: the monitor is itself a Modbus client. It adds traffic to the network. It
  can only see what it asks for. It is visible to any device logging connections.
- Passive capture: the monitor observes copies of traffic without participating. It can see
  every conversation between every device on the monitored segment. It is invisible to devices
  on the network.

Phase A (this exercise) uses active polling. Phase B (Beta 0.7) will deploy a TAP/SPAN port
for passive capture. Both approaches have distinct capabilities and blind spots -- understanding
the difference is a central learning objective of this scenario series.

The monitoring dashboard is delivered by the tools built across SOW-029.0 through SOW-032.0:

- Transaction event log with function code analysis (SOW-029.0)
- Communication graph and per-device FC histograms (SOW-030.0)
- CEF syslog forwarding for SIEM integration (SOW-031.0)
- Event-driven behavioral baseline with anomaly detection rules (SOW-032.0)

---

## Objectives

1. Deploy the monitoring dashboard, navigate to the transaction event log, and identify the
   active polling pattern across all 6 facility devices.
2. Wait for the baseline learning period to complete and record the per-device function code
   profile and communication pattern for each device.
3. Analyze the communication matrix to categorize devices as read-only sensors or read-write
   actuators, and identify the normal traffic pattern for this environment.
4. Enable CEF syslog forwarding, receive events on a local listener, and parse the CEF
   format to extract device and protocol metadata.
5. Trigger a detection by injecting a Modbus write to a read-only device and observe the
   resulting alerts in the dashboard, including alert severity, rule IDs, and expected
   vs. actual behavior.

---

## Rules of Engagement

- During Phases A1 through A4: read-only observation only. Do not send write requests to any
  device. The baseline must represent normal read-only traffic.
- Phase A5 explicitly authorizes a single Modbus write to a read-only device. This write is
  the detection trigger. After observing the alert, do not write to the device again.
- Do not attempt to crash or hang any device.
- Document all observations. If a dashboard page does not display expected data, record what
  you see rather than skipping it. Unexpected behavior is information.
- The monitoring module sends Modbus read requests to all 6 devices during this exercise.
  This is expected. If you observe Modbus traffic from a source other than your monitoring
  workstation, document it.

---

## Starting Conditions

Both the plant simulation and the monitoring module must be running:

```
docker compose --profile water --profile monitor up
```

Verify the plant is responding:

```
mbpoll -t 4 -r 0 -c 1 -1 localhost -p 5020
```

If you receive a register value, the plant is running.

Verify the monitoring dashboard is accessible:

```
http://localhost:8090
```

The overview page should show 6 devices for the greenfield-water-mfg environment. If any
devices appear Offline, wait 30 seconds for the polling cycle to complete, then refresh.

**Services and ports**:

| Service | URL or Port | Description |
|---------|------------|-------------|
| Monitoring dashboard | `http://localhost:8090` | Primary interface for this exercise |
| Alert API | `http://localhost:8091/api/alerts` | JSON API for active alerts |
| Baseline API | `http://localhost:8091/api/baselines` | JSON API for baseline status |
| Syslog receiver (Phase A4) | `localhost:1514` | Non-privileged port for lab use |
| Plant (water side) | `localhost:5020-5022` | Water treatment PLCs (Modbus TCP) |
| Plant (manufacturing) | `localhost:5030` | Moxa gateway, serial PLCs behind |

**Note on syslog port**: Standard syslog uses port 514, which requires root privileges on most
Unix systems. This exercise uses port 1514 to avoid privilege requirements. In a production
deployment, syslog traffic from OT monitoring tools typically flows to a syslog concentrator
in the IT/OT DMZ on port 514, protected by network ACLs.

**Tools you need**:

- Browser with access to `http://localhost:8090`
- `mbpoll` (version 1.4 or later) -- Modbus TCP client for Phase A5 write injection
- A text editor or markdown tool for recording observations in the baseline template
- `nc` (netcat) or equivalent -- for receiving syslog in Phase A4

---

## Deliverables

At the end of this scenario you must produce:

1. A completed baseline observation form using the template in
   `reference/baseline-observation-template.md`. One row per device, recording FC profile,
   write target status, and observed response time range.
2. A summary of the alert(s) triggered in Phase A5: rule ID, severity, device ID, and your
   explanation of why each rule fired.
3. Answers to the four conceptual questions in `success-criteria.md`, Phases A5-conceptual.

---

## Phase A1: Deploy and Configure Monitoring

### Step A1-1: Open the monitoring dashboard

Navigate to `http://localhost:8090`. The overview page loads the greenfield-water-mfg
environment. Confirm you see:

- 6 devices listed: wt-plc-01, wt-plc-02, wt-plc-03, mfg-gateway-01, mfg-plc-01, mfg-plc-02
- Each device showing Online status
- Baseline status showing "Learning" (learning period begins on first poll after startup)

If any device shows Offline, it indicates a polling failure. Check that the plant simulation
started successfully before proceeding.

### Step A1-2: Observe the live transaction event log

Navigate to `http://localhost:8090/events`.

The event table shows Modbus transaction events as they arrive, in reverse chronological order.
Identify the following columns:

| Column | What it shows |
|--------|--------------|
| Time | Timestamp of the transaction (RFC 3339) |
| Device | Which device received the request (e.g., wt-plc-01) |
| FC | Function code number (e.g., 3 = Read Holding Registers) |
| FC Name | Human-readable function code name |
| Address | Starting register address |
| Count | Number of registers or coils requested |
| Success | Whether the device responded with data (green) or an exception (red) |
| Write | Badge present if the operation was a write (FC 5, 6, 15, or 16) |

Watch the table for 60 seconds. You should see events arriving continuously as the monitor
polls each device every 2 seconds.

**Observe the polling pattern**: Which device ID appears most frequently? Which function
codes appear? Are any "Write" badges visible in the table during normal operation?

Expected observations during normal operation:
- FC 3 (Read Holding Registers) is the dominant function code
- FC 1 (Read Coils) appears for devices that have coils
- No Write badges should appear during Phases A1 through A4
- Events from all 6 devices appear in rotation

### Step A1-3: Identify the star topology

Navigate to `http://localhost:8090/comms`.

The communication matrix shows which source addresses are communicating with which devices.
In active monitoring, all traffic originates from a single source -- the monitoring process.
You should observe a star topology: one source IP connecting to all 6 device endpoints.

This is the expected and correct pattern for SCADA/monitoring polling in a well-managed OT
network. The teaching point: if you later observe a second source IP communicating with these
devices, that deviation from the star topology is anomalous and warrants investigation. In
Phase B (passive capture), the communication graph will become more revealing because it will
capture all device-to-device conversations, not only monitoring poll traffic.

**Record**: The source IP address of the monitoring process. This is the address you will
see in CEF events in Phase A4.

### Step A1-4: Confirm polling frequency

On the `/events` page, count the events for a single device over 30 seconds. At a 2-second
poll interval, you should see approximately 15 events per device per 30 seconds (one event
per poll cycle for the holding register read, plus one for the coil read if the device has
coils).

The Moxa gateway (mfg-gateway-01) generates additional events for each serial device behind
it. Traffic to mfg-plc-01 and mfg-plc-02 passes through the gateway, and each serial
transaction generates a gateway-delay latency that is visible in the response time column.

---

## Phase A2: Build a Traffic Baseline

### Step A2-1: Wait for the learning period

The baseline engine requires 150 polling cycles to build its behavioral model. At a 2-second
interval, this is approximately 5 minutes. The learning period begins with the first poll
after monitor startup.

While waiting, use this time to:
- Read the "OT Security Concepts" section at the end of this document
- Begin completing the baseline observation template in `reference/baseline-observation-template.md`
  (the FC profile columns can be filled from the `/events` data you already observed)

Do not proceed to Phase A3 until at least one device shows "Established" baseline status.

### Step A2-2: Check baseline status

Navigate to `http://localhost:8091/api/baselines` (JSON) or observe the status column on the
Assets page at `http://localhost:8090/assets`.

Each device will show one of:
- `"learning"` -- the baseline engine is still accumulating samples
- `"established"` -- the learning period is complete; anomaly detection is active

The transition from "learning" to "established" does not happen simultaneously for all devices.
Devices that started receiving traffic slightly later (e.g., serial devices behind the gateway
that require an extra discovery poll) may complete their learning period a few cycles after the
Ethernet PLCs.

**Record**: The timestamp when the first device transitions to "established". This is the
moment anomaly detection becomes active.

### Step A2-3: Record the function code profile per device

Navigate to each device's detail page: `http://localhost:8090/assets/{device-id}`.

The FC Distribution section shows a histogram of which function codes were received by this
device during the observation window, with counts and percentages.

For each of the 6 devices, record in the baseline observation template:
- Which function codes appear in the distribution
- Whether FC 5 (Write Single Coil), FC 6 (Write Single Register), FC 15 (Write Multiple
  Coils), or FC 16 (Write Multiple Registers) appear -- these are the write function codes
- The dominant function code (highest count)

The write function code presence determines the `IsWriteTarget` flag in the baseline engine.
A device that never received a write FC during the learning period will trigger a
`write_to_readonly` alert if it receives a write after baseline establishment.

### Step A2-4: Record response time ranges

On each device detail page, note the observed response time range for the device. The response
time measures the elapsed time from when the monitor sent the Modbus request to when it
received the response.

Expected ranges for this environment:

| Device | Expected Response Time | Notes |
|--------|----------------------|-------|
| wt-plc-01 | ~5ms | Ethernet, Level 1 network |
| wt-plc-02 | ~5ms | Ethernet, Level 1 network |
| wt-plc-03 | ~5ms | Ethernet, Level 1 network |
| mfg-gateway-01 | ~15ms | Ethernet, flat network, gateway processing overhead |
| mfg-plc-01 | ~65ms ±20ms | Serial via gateway (RS-485 at 9600 baud, SLC-500) |
| mfg-plc-02 | ~95ms ±50ms | Serial via gateway (RS-485 at 9600 baud, Modicon 984) |

The high jitter on mfg-plc-02 (Modicon 984) is normal for a 1988-era processor with
variable interrupt response time. It is not a network fault. The baseline engine learns
the expected response time range and will alert on `response_time_anomaly` if response
times deviate significantly from this learned range.

---

## Phase A3: Analyze Communication Patterns

### Step A3-1: Categorize devices by function code profile

Using the FC profiles recorded in Phase A2, categorize each device:

**Read-only devices** (sensors and monitoring-only endpoints): Devices that received only
read function codes (FC 1, FC 2, FC 3, FC 4) during the learning period. These devices
should never receive write commands during normal operations. Any write to a read-only
device is a potential unauthorized modification.

**Read-write devices** (actuators and controllers with writable setpoints): Devices that
received write function codes (FC 5, FC 6, FC 15, FC 16) during the learning period. These
devices receive control commands or setpoint updates as part of normal SCADA operations.

For the greenfield-water-mfg environment, the expected categorization is:

| Device | Category | Reason |
|--------|----------|--------|
| wt-plc-01 | Read-write | Intake pump speed setpoint (SC-101) is writable |
| wt-plc-02 | Read-write | Chemical feed rate setpoint (FIC-202) is writable |
| wt-plc-03 | Read-only | Distribution PLC; monitoring only in this scenario |
| mfg-gateway-01 | Read-only | Gateway diagnostics registers; no writable setpoints |
| mfg-plc-01 | Read-write | Conveyor speed and cycle time setpoints are writable |
| mfg-plc-02 | Read-only | Cooling system; monitoring only in this scenario |

**Note**: "Read-only" in the context of the monitor's active polling means the monitor sends
only read requests to that device. The device itself may be physically capable of accepting
writes -- the PLC hardware has no built-in protection against Modbus write commands. The
monitoring system is the detection layer, not the prevention layer. If the categorization
above does not match your observed FC profiles, document the discrepancy.

### Step A3-2: Identify write targets from the event log

Navigate to `http://localhost:8090/events` and apply the Write filter to display only
write-category events.

During normal monitoring operations (Phases A1-A3), you should see zero write events in the
filtered view. The monitoring module only sends read requests. All write operations are
initiated by the SCADA system or by a direct operator command.

This zero-write baseline is significant: when you inject a write in Phase A5, it will
immediately appear as the first write event in this device's history, triggering both the
`write_to_readonly` and `fc_anomaly` rules simultaneously.

### Step A3-3: Compare FC histograms across devices

Navigate to `/assets/{device-id}` for each device and compare the FC distribution
histograms. Observe:

- Water treatment PLCs: FC 3 and FC 1 dominate. FC 3 reads the holding registers containing
  process values (flow, pressure, pH, temperature). FC 1 reads coils (pump run state, alarm
  state).
- Moxa gateway: FC 3 only. The gateway status registers are all holding registers. No coils.
  FC 1 on the gateway returns exception 02 (Illegal Data Address).
- Serial PLCs via gateway: FC 3 and FC 1. Same pattern as Ethernet PLCs but with longer
  response times due to serial bus latency.

**Teaching point -- gateway and protocol translators**: The Moxa NPort 5150 gateway appears
as a single Modbus TCP device, but it bridges traffic to two serial PLCs (mfg-plc-01 and
mfg-plc-02) on the RS-485 bus behind it. A transaction to mfg-plc-01 travels: monitor -->
mfg-gateway-01 (TCP) --> RS-485 bus --> SLC-500 --> RS-485 bus --> mfg-gateway-01 (TCP) -->
monitor. The gateway has a broader role than its own register count suggests: it aggregates
access to all devices on the serial bus. Traffic to the gateway represents aggregated access
to every serial device behind it.

### Step A3-4: Identify the normal communication pattern

Summarize the normal communication pattern for this environment:

- Source: a single monitoring IP address
- Destination: 6 device endpoints (4 Ethernet, 2 serial via gateway)
- Protocol: Modbus TCP (port 502 in production; simulator ports 5020-5030)
- Function codes: FC 1 (coils) and FC 3 (holding registers) only
- Direction: all requests originate from the monitor; devices only respond
- Frequency: one poll cycle per 2 seconds per device
- Writes: zero during normal monitoring

This pattern is your behavioral baseline. Any deviation -- new source address, new function
code, write operation to a read-only device, or poll gap -- is anomalous.

---

## Phase A4: Configure SIEM Forwarding

### Step A4-1: Enable syslog forwarding

Open the monitoring configuration file at `monitoring/config/monitor.yaml`. Add the syslog
section:

```yaml
syslog:
  enabled: true
  target: "localhost:1514"
  protocol: "udp"
  facility: "local0"
  format: "cef"
```

**Configuration field notes**:

- `target` must be in `host:port` format -- not a URL. The value `"udp://localhost:1514"` is
  incorrect and will be rejected with a descriptive validation error.
- `protocol` accepts `"udp"` or `"tcp"`. UDP is appropriate for a lab environment. Production
  deployments should evaluate TCP for guaranteed delivery, or TLS syslog (RFC 5425) for
  encrypted transport across the IT/OT DMZ.
- `facility` sets the syslog facility code. `"local0"` (facility 16) is the conventional
  choice for application-generated security events.
- `format` must be `"cef"`. No other format is supported in Beta 0.6.

### Step A4-2: Start a local syslog receiver

Open a second terminal. Start a netcat listener on UDP port 1514:

```
nc -u -l 1514
```

On macOS, netcat may require different flags. If the above does not work:

```
nc -u -l -p 1514
```

Leave this terminal open and visible. Events will appear here in real time.

### Step A4-3: Restart the monitoring module

For the syslog configuration change to take effect, restart the monitor:

```
docker compose --profile monitor restart monitor
```

Or, if running the monitor binary directly:

```
./monitoring/monitor
```

After restart, events should begin flowing into the netcat terminal within 2-4 seconds
(the first poll cycle after startup).

### Step A4-4: Parse CEF format

Each Modbus transaction event arrives as one syslog line in CEF format. A typical read event
looks like:

```
<134>CEF:0|OTSimulator|Monitor|0.6|3|Read Holding Registers|1|src=127.0.0.1:52341 dst=127.0.0.1:5020 cs1=Read Holding Registers cs1Label=FunctionCode cn1=0 cn1Label=AddressStart cn2=5 cn2Label=AddressCount cs2=wt-plc-01 cs2Label=DeviceID cs3=greenfield-water-mfg cs3Label=Environment rt=1740873600000 outcome=success
```

A write event (when you perform the injection in Phase A5) will look like:

```
<130>CEF:0|OTSimulator|Monitor|0.6|6|Write Single Register|7|src=127.0.0.1:54201 dst=127.0.0.1:5022 cs1=Write Single Register cs1Label=FunctionCode cn1=2 cn1Label=AddressStart cn2=1 cn2Label=AddressCount cs2=wt-plc-03 cs2Label=DeviceID cs3=greenfield-water-mfg cs3Label=Environment rt=1740873720000 outcome=success
```

**CEF header breakdown**:

```
<priority>CEF:version|vendor|product|version|signatureId|name|severity|extensions
```

| Field | Example | Meaning |
|-------|---------|---------|
| `<priority>` | `<134>` | Syslog priority: facility*8 + syslog_severity |
| `CEF:0` | `CEF:0` | CEF format version (always 0) |
| vendor | `OTSimulator` | Tool vendor |
| product | `Monitor` | Tool name |
| version | `0.6` | Tool version |
| signatureId | `3` | Function code number |
| name | `Read Holding Registers` | Function code name |
| severity | `1` or `7` | CEF severity (1=read, 7=write) |

**CEF extensions** (key=value pairs after the final pipe):

| Key | Example | Meaning |
|-----|---------|---------|
| `src` | `127.0.0.1:52341` | Source IP and ephemeral port of the Modbus client |
| `dst` | `127.0.0.1:5020` | Destination IP and port of the Modbus device |
| `cs1` | `Read Holding Registers` | Function code name (string) |
| `cs1Label` | `FunctionCode` | Label for cs1 |
| `cn1` | `0` | Starting register address |
| `cn1Label` | `AddressStart` | Label for cn1 |
| `cn2` | `5` | Number of registers or coils in the request |
| `cn2Label` | `AddressCount` | Label for cn2 |
| `cs2` | `wt-plc-01` | Device ID from the environment definition |
| `cs2Label` | `DeviceID` | Label for cs2 |
| `cs3` | `greenfield-water-mfg` | Environment ID |
| `cs3Label` | `Environment` | Label for cs3 |
| `rt` | `1740873600000` | Event timestamp in milliseconds since Unix epoch |
| `outcome` | `success` or `failure` | Whether the device returned data or an exception |

**CEF severity levels** for Modbus events:

| Severity | Events | Syslog Level | Rationale |
|----------|--------|-------------|-----------|
| 7 (High) | Write operations (FC 5, 6, 15, 16) | Critical (2) | Writes have physical consequences |
| 5 (Medium) | Read failures (exception response) | Warning (4) | May indicate probing or misconfiguration |
| 3 (Medium-Low) | Diagnostic operations (FC 43) | Notice (5) | Reconnaissance vector |
| 1 (Low) | Read successes (FC 1, 2, 3, 4) | Informational (6) | Normal polling traffic |

**Priority calculation**: `<priority>` = facility_code * 8 + syslog_severity_code.
For facility `local0` (code 16) and a read success (syslog severity 6): 16*8+6 = 134.
For a write operation (syslog severity 2): 16*8+2 = 130. The write event priority tag is `<130>`.

**Security note on FC 43**: Function code 43 (MEI Transport -- Device Identification) appears
at CEF severity 3, above informational but below read failures. This is intentional. In the
2017 TRITON/TRISIS attack on a Schneider Electric Triconex safety PLC, the initial access
phase included device identification requests to enumerate the safety system's identity and
firmware version. FC 43 requests from an unknown source are a high-fidelity indicator of
reconnaissance. If you build a SIEM rule to alert on CEF severity >= 3, FC 43 events will
be included. If FC 43 events were severity 1 (informational), they would be silently discarded
by most SIEM noise filters.

**Security note on plaintext transport**: CEF syslog in this lab is transmitted in plaintext
over UDP. In production, this traffic traverses the IT/OT DMZ. An attacker who can capture
DMZ traffic gains the device topology (every device ID and address), the polling schedule,
which devices receive write commands, and response time signatures that can identify device
types. RFC 5425 defines TLS-encrypted syslog for production use. The lab uses plaintext to
avoid certificate management complexity; this is explicitly a technical debt item
(td-events-061) for a future milestone.

### Step A4-5: Identify severity levels in the live feed

Watch the syslog receiver for at least 30 seconds. Identify:

- Which severity value appears most frequently (expected: 1, for read successes)
- Whether any severity 5 events appear (read failures indicate polling errors)
- Whether any severity 7 events appear (write operations -- should be zero during A4)
- The `src` field value (this is the monitor's source address, confirming the star topology)

---

## Phase A5: Detect Unauthorized Activity

**Important**: This phase requires the baseline to be established on at least the target
device before alerts will fire. Confirm that the target device shows "Established" status
on `http://localhost:8090/assets` before proceeding.

### Step A5-1: Review current alerts

Navigate to `http://localhost:8090/alerts` (or `http://localhost:8091/api/alerts` for JSON).

Before the write injection, you may see:

- `value_out_of_range` alerts if any register value drifted outside the learned statistical
  range during the learning period. Sensor noise causes occasional minor fluctuations in
  temperature, pressure, and pH readings. These are low-severity alerts and do not indicate
  an attack.
- `response_time_anomaly` alerts if a device responded unusually slowly or quickly on a
  particular cycle.

These pre-existing alerts are normal. Note them before proceeding so you can distinguish them
from the alerts you will trigger.

### Step A5-2: Inject an unauthorized write

Select a device categorized as read-only in Phase A3. The recommended target is wt-plc-03
(the distribution PLC, which only receives read requests during normal monitoring operations).

Use `mbpoll` to write a single holding register value. Write to register address 2
(distribution pressure, HR[2]):

```
mbpoll -t 4 -r 2 -c 1 -1 localhost -p 5022 -- 9999
```

Flag reference:
- `-t 4` : holding registers (function code 03 for reads; this is a write, so the actual FC
  will be FC 6 Write Single Register)
- `-r 2` : register address 2
- `-c 1` : one register
- `-1` : single operation
- `-- 9999` : value to write (9999 is outside the normal operating range for distribution
  pressure, making the anomaly visible)

**Note on mbpoll write syntax**: mbpoll uses the `-t 4` flag for holding registers in both
read and write modes. When a value argument is provided after `--`, mbpoll sends FC 6 (Write
Single Register) for a single register or FC 16 (Write Multiple Registers) for multiple.

Expected mbpoll output:

```
Written 1 registers at address 2 on server localhost:5022
```

If you see an exception response instead, confirm the device is online and the register
address is correct.

### Step A5-3: Observe the alerts

Navigate immediately to `http://localhost:8090/alerts`.

You should see two new alerts within one polling cycle (2 seconds) of the write:

**Alert 1: write_to_readonly**
- Rule ID: `write_to_readonly`
- Severity: Critical
- Device: wt-plc-03 (or whichever device you targeted)
- Description: A write operation targeted a device with no write operations observed during
  the learning period. The baseline recorded zero write function codes for this device;
  the first write FC observed post-establishment triggered this rule.

**Alert 2: fc_anomaly**
- Rule ID: `fc_anomaly`
- Severity: High
- Device: wt-plc-03
- Description: Function code 6 (Write Single Register) was observed but was not present in
  the baseline function code set. During learning, this device only received FC 3 and FC 1.
  FC 6 is a new function code and triggered the anomaly rule.

**Why two alerts from one write**:

The `write_to_readonly` rule fires because the device was baselined as read-only
(`IsWriteTarget=false`). Any write FC triggers it, regardless of which write FC is used.

The `fc_anomaly` rule fires because FC 6 was not in the device's observed function code set
from the learning period. During learning, wt-plc-03 only received FC 3 and FC 1. FC 6 is
a new, unseen function code.

This is defense-in-depth through overlapping detection rules. An attacker who somehow
bypassed `write_to_readonly` (for example, by first writing to a device during the learning
period to establish it as a write target) would still trigger `fc_anomaly` if they used a
write FC not seen during learning.

### Step A5-4: Review alert details

Click on each alert (or inspect the JSON from the API) to review:

- `alert_id`: deterministic identifier, e.g., `write_to_readonly:wt-plc-03:-1`
- `rule_id`: the rule that fired
- `severity`: critical or high
- `device_id`: the targeted device
- `observed_value`: the write function code that triggered the alert
- `expected_value`: the empty write FC set from the baseline
- `first_seen`: timestamp of the first trigger
- `last_seen`: timestamp of the most recent trigger

### Step A5-5: Observe the write event in the syslog feed

In the syslog receiver terminal, locate the write event. It should have arrived within the
same second as the write. Identify:

- The severity field: 7 (High -- write operations always generate severity 7 regardless of
  the outcome)
- The `outcome` field: success or failure
- The `cn1` field: the register address you wrote (2)
- The `cs1` field: "Write Single Register" (FC 6)

### Step A5-6: Observe the `new_source` alert absence

Return to `http://localhost:8090/alerts`. You should NOT see a `new_source` alert. This is
expected, and understanding why is an important teaching point.

The `new_source` rule fires when a Modbus transaction originates from a source address not
observed during the learning period. During the learning period, only the monitoring module
sent Modbus requests -- all from its own IP address. When you ran mbpoll from the same
workstation, your mbpoll command used the same source IP as the monitoring module (both are
localhost in this lab).

The `new_source` alert becomes meaningful in Phase B (passive capture with a SPAN port):
passive monitoring observes all traffic on the network segment, including Modbus requests
from the SCADA server, engineering workstations, and any attacker who gains network access.
With passive capture, a new source IP addressing a PLC for the first time is a strong
indicator of unauthorized access.

This deliberate limitation teaches the boundary of what active polling can detect: it cannot
observe traffic from other sources, because it is itself the only source.

---

## What Comes Next

This exercise completes Phase A of Scenario 04. The monitoring foundation you have built --
transaction event log, behavioral baseline, CEF syslog forwarding, and anomaly detection
rules -- will support all subsequent phases.

**Phase B (Beta 0.7): Passive Network Capture**

Phase B deploys a TAP/SPAN port and a passive capture engine (Cisco CyberVision or Zeek).
Passive capture sees all network conversations, not just monitoring poll traffic. This makes
the `new_source` alert meaningful for the first time: you will observe Modbus requests from
the SCADA server, from engineering workstations during maintenance windows, and from any
unauthorized source. Phase B will also reveal device-to-device communications not visible
to active polling.

**Phase C (Beta 0.8): Dragos Platform Integration**

Phase C deploys the Dragos Platform against the passive capture feed. You will experience
what commercial tools automate from the manual process you performed here:

- Automated asset inventory (no manual mbpoll enumeration)
- Automated baseline learning with 30-90 day characterization periods
- Threat intelligence matching against known ICS adversary TTPs
- Built-in alert rules for common OT attack patterns (TRITON, CRASHOVERRIDE, STUXNET)

Understanding what these tools automate is only meaningful because you built the manual
equivalent in Phase A. Engineers who deploy Dragos without understanding behavioral baselines
cannot distinguish a false positive from a genuine threat when an alert fires at 2:00 AM.

**Phase D (Beta 0.9): Blastwave BlastShield Microsegmentation**

Phase D shifts from detection to prevention. BlastShield microsegmentation enforces
allowlists: only authorized source addresses can send Modbus requests to authorized devices.
An attacker who gained network access and sent the same unauthorized write you performed in
Phase A5 would have the connection rejected before it reached the device, rather than
detected after the write succeeded.

The progression from detection (Phase A) to prevention (Phase D) reflects the security
maturity model: you cannot build effective prevention rules without first understanding what
normal behavior looks like. The baseline you built in Phase A is the input to the
allowlist policies in Phase D.

---

## OT Security Concepts

### Why writes are the highest-risk operations in OT

In IT security, a write operation (database INSERT, file write, registry modification) is
risky but often reversible. In OT environments, a write to a process control register has
physical consequences:

- Writing 0 to a pump run coil stops a pump. In a water treatment plant, this can cause
  pressure loss, backflow, or treatment failure.
- Writing an out-of-range value to a chemical feed setpoint can cause over-dosing (toxicity)
  or under-dosing (contamination) of the treated water.
- Writing to conveyor speed registers can cause mechanical damage if the change exceeds the
  mechanical system's rated speed.

Physical consequences are often not immediately reversible. A pump that was stopped may take
minutes to restart safely. A chemically imbalanced batch of water must be detained and
re-treated. This is why FC 5, FC 6, FC 15, and FC 16 generate severity 7 (High) in CEF --
the act of writing, regardless of outcome, warrants immediate attention.

### Why behavioral baselines outperform signatures in OT

In IT security, signature-based detection matches known malicious patterns (malware hashes,
exploit strings, known bad IPs). Signatures are effective when attackers use known tools.

In OT environments, attackers increasingly use the legitimate Modbus protocol itself as the
attack vector -- there is no malware to detect, no exploit string to match. The TRITON/TRISIS
attack used legitimate Modbus function codes to communicate with Triconex safety PLCs. A
signature-based system looking for malware would have found nothing unusual.

Behavioral baselines detect deviations from normal, expected behavior. An OT environment
is highly deterministic: the same PLCs communicate with the same SCADA server using the same
function codes at the same intervals, day after day. Any deviation from this pattern --
a new source address, a new function code, an unexpected write -- is anomalous because it
was not part of the learned normal.

The 5-minute learning period in this lab is compressed for training purposes. Commercial
tools like Dragos recommend 30-90 days of observation to capture maintenance windows,
shift changes, and seasonal variation. The principle is identical: observe first, then detect
deviations.

### What commercial tools automate vs. what this manual layer teaches

The monitoring tools in this exercise are intentionally low-level. You manually:
- Navigated the event log to identify polling patterns
- Waited for the learning period and checked baseline status via an API endpoint
- Parsed raw CEF format in a netcat terminal
- Reviewed raw alert JSON via an API endpoint

Commercial tools (Dragos Platform, Cisco CyberVision) automate all of this:
- Asset inventory is built automatically from passive traffic observation
- Baselines are built with no operator interaction over weeks or months
- Alerts are presented with asset context, vulnerability cross-references, and remediation
  guidance pre-populated
- SIEM integration is handled by pre-built connectors

Why learn the manual layer first? Because automated tools fail in predictable ways. When
Dragos fires a false positive at 2:00 AM, the engineer on call must determine whether it is
a real attack or a false positive. That determination requires understanding what the tool
is doing and why. An engineer who has never built a behavioral baseline cannot evaluate
whether the automated baseline is correct.

The same principle applies in IT security: you learn to read firewall logs before deploying
a SIEM, and you learn SIEM correlation rules before deploying an XDR platform.
