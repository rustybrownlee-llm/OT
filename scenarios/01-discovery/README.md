# Scenario 01: OT Asset Discovery

**Phase**: Discovery
**Environment**: greenfield-water-mfg
**Difficulty**: Beginner
**Estimated Time**: 60-90 minutes
**Prerequisites**: Basic TCP/IP networking, familiarity with nmap, ability to install command-line tools

---

## Background

You have been engaged as a security consultant for Greenfield Water and Manufacturing. The facility
manager knows there are "some PLCs on the network" but has no asset inventory and no network diagram.
Your engagement letter specifies that you must first document what exists before any assessment or
remediation work can begin.

The facility has two OT networks:

- **Water treatment plant**: A modern system installed in 2020, following the Purdue Reference Model
  with network segmentation between levels. The plant manager can confirm "three PLCs on the water
  side." These are on the Level 1 network (10.10.10.0/24).

- **Manufacturing floor**: A legacy system from 1993, running on a flat Ethernet network
  (192.168.1.0/24). The maintenance technician believes there is "a gateway box and maybe two old
  PLCs" but cannot confirm make, model, or addresses.

The two networks are connected by a cross-plant link (172.16.0.0/30) that allows the water treatment
system to supply process water to manufacturing. You have a workstation on the water treatment Level 1
network (10.10.10.0/24) with direct IP access to the Modbus TCP ports via localhost port mappings.

---

## Objectives

1. Discover all IP-addressable devices on both OT networks.
2. Enumerate the Modbus TCP registers on each discovered device.
3. Identify all devices behind serial gateways that do not have their own IP addresses.
4. Produce a complete asset inventory covering all 6 devices in the facility.

---

## Rules of Engagement

- Passive enumeration only: do not write to any holding registers or coils except where the scenario
  explicitly instructs you to probe response behavior.
- Poll registers in small batches (10-20 registers at a time). In a real facility, large sequential
  reads on legacy PLCs can cause scan time overrun and watchdog trips. Practice the discipline here.
- Do not attempt to crash or restart any device.
- Document everything, including devices that return errors. An exception code is information.

---

## Starting Conditions

The plant simulation is running with the following services active:

- Water treatment PLCs: responding on localhost ports 5020, 5021, and 5022
- Manufacturing gateway: responding on localhost port 5030
- All process values are changing (drift and noise from the SOW-003.0 process simulation)

**Important note on port numbers**: In this simulator, Modbus TCP services are bound to ports
5020-5039 to avoid requiring root privileges. In a real OT facility, Modbus TCP always runs on
**port 502**. When you perform discovery in a production environment, scan for port 502, not 5020.
The port mapping is a simulator-specific constraint only.

**Tools you need**:

- `nmap` (version 7.x or later) -- network scanning
- `mbpoll` (version 1.4 or later) -- Modbus TCP polling client. Stable C binary available at
  https://github.com/epsilonrt/mbpoll. Primary tool for this scenario.
- Alternative Modbus clients (if mbpoll is unavailable): `modpoll` (Modbusdriver) or the
  `pymodbus` Python library. Note that `pymodbus` changed its API between v2 and v3; verify your
  version before using the command-line examples in the solution.

**What you know vs. what you must discover**:

| Known | Must Discover |
|-------|---------------|
| Water treatment network: 10.10.10.0/24 | IP addresses of all devices |
| Manufacturing network: 192.168.1.0/24 | Device make and model |
| Cross-plant link: 172.16.0.0/30 | Register counts and contents |
| Simulator ports start at 5020 | Devices that are NOT IP-addressable |
| There are roughly 5-6 devices total | Unit IDs and serial device identities |

---

## Deliverables

At the end of this scenario you must produce a completed asset inventory using the template in
`reference/asset-inventory-template.md`. Compare your result against
`reference/expected-asset-inventory.md` to verify completeness.

Your completed inventory must include:

- All 6 devices (4 with IP addresses, 2 without)
- For each IP-addressable device: IP address, simulator port, device category, holding register count
- For each serial device: gateway IP, unit ID, device make/model, register count
- For each device: observed response time
- Network topology: which network each device belongs to

---

## Phase A: Network Reconnaissance

### Step A1: Scan for open Modbus TCP ports

In a real facility you would scan the network subnets for port 502. In this simulator, scan
localhost for the port range 5020-5039.

```
nmap -sV -p 5020-5039 localhost
```

Expected output pattern:

```
PORT     STATE SERVICE  VERSION
5020/tcp open  modbus?
5021/tcp open  modbus?
5022/tcp open  modbus?
5030/tcp open  modbus?
```

You should see exactly 4 open ports: 5020, 5021, 5022, and 5030. Ports 5023-5029 and 5031-5039
should appear closed.

**Teaching point**: You found 4 network endpoints. The facility manager said there are roughly
5-6 devices. There is a discrepancy. Resolve this before you close the inventory.

### Step A2: Note response characteristics

During the scan, observe timing differences in how quickly each port responds. You will return to
this in Phase B.

---

## Phase B: Modbus Enumeration

For each open port, attempt to read holding registers and coils. Read in small batches to mimic
careful field practice.

### Step B1: Enumerate Port 5020

Read holding registers 0-9 (addresses 0 through 9, batch of 10):

```
mbpoll -t 4 -r 0 -c 10 -1 localhost -p 5020
```

Flag reference:
- `-t 4` : holding registers (function code 03)
- `-r 0` : starting address 0 (zero-based)
- `-c 10` : read 10 registers
- `-1` : single poll (no repeat)
- `-p 5020` : connect to port 5020

You should receive 5 non-zero values followed by 5 zeros. The first 5 registers hold process data
(flow rate, pump speed, pH, turbidity, temperature). Values will drift over time -- exact values
depend on simulation timing.

Now attempt coils (function code 01):

```
mbpoll -t 0 -r 0 -c 10 -1 localhost -p 5020
```

Flag reference:
- `-t 0` : coils (function code 01)

You should receive 4 coil values (pump run states and status bits) and an exception or zeros for
addresses 4-9.

Record for your asset inventory:
- Holding registers: 5 populated (addresses 0-4)
- Coils: 4 populated (addresses 0-3)
- Response time: approximately 5ms

### Step B2: Enumerate Port 5021

Repeat the same two commands for port 5021:

```
mbpoll -t 4 -r 0 -c 10 -1 localhost -p 5021
mbpoll -t 0 -r 0 -c 10 -1 localhost -p 5021
```

You should receive 7 holding register values (filter pressures, UV intensity, chemical feed rate,
chlorine residual, post-filter turbidity) and 4 coil values (backwash command, chemical pump, UV
status, high-DP alarm).

Record for your asset inventory:
- Holding registers: 7 populated (addresses 0-6)
- Coils: 4 populated (addresses 0-3)
- Response time: approximately 5ms

### Step B3: Enumerate Port 5022

```
mbpoll -t 4 -r 0 -c 10 -1 localhost -p 5022
mbpoll -t 0 -r 0 -c 10 -1 localhost -p 5022
```

You should receive 5 holding register values (clear well level, distribution flow, pressure,
residual chlorine, water temperature) and 3 coil values (two distribution pumps and a booster pump).

**Note**: This device has 3 coils, not 4. Do not assume uniform coil counts across devices of the
same type.

Record for your asset inventory:
- Holding registers: 5 populated (addresses 0-4)
- Coils: 3 populated (addresses 0-2)
- Response time: approximately 5ms

### Step B4: Enumerate Port 5030

```
mbpoll -t 4 -r 0 -c 10 -1 localhost -p 5030
```

You should receive 9 holding register values (serial port status, baud rate, data format, mode,
active connections, TX/RX counts, error count, uptime).

Now attempt coils:

```
mbpoll -t 0 -r 0 -c 10 -1 localhost -p 5030
```

You will receive a Modbus exception code 02 (Illegal Data Address). This device has no coils.

**Teaching point**: A device that speaks Modbus but responds to coil reads with exception 02 has
reported that the requested address does not exist. A PLC controlling physical equipment always has
output coils for actuators. A device with no coils is likely a gateway, sensor, or monitoring device
-- not a controller. This alone tells you something about what this device is.

**Teaching point on response times**: Compare the response time of port 5030 against ports 5020-5022.
The water treatment PLCs respond in approximately 5ms. Port 5030 may show a different
characteristic, and if there are serial devices behind it, their response times will be longer still.
Response latency is a device fingerprinting signal.

Record for your asset inventory:
- Holding registers: 9 populated (addresses 0-8)
- Coils: 0 (exception 02 on FC01)
- Response time: approximately 15ms
- Category: gateway (inferred from no coils + register contents describing a serial port)

---

## Phase C: Gateway Discovery

You have 4 IP endpoints and 9 holding registers on port 5030 describing a serial port. The
facility manager said 5-6 devices. You are still short by at least one.

Modbus TCP supports a **unit ID** field in every request header. For devices connected directly to
Ethernet, unit ID is typically 0 or 1 and is ignored or treated as a broadcast. For gateways,
the unit ID is used to route the request to a specific downstream serial device. The gateway
forwards the request -- including the original unit ID -- unchanged onto the serial bus.

This means: the IP address gets you to the gateway; the unit ID gets you to the device behind it.

**Teaching point**: Devices connected through a serial-to-Ethernet gateway have no IP address. They
are completely invisible to network scanning. The only way to find them is to connect to the gateway
IP and probe unit IDs. This is one of the most common discovery gaps when IT engineers first assess
legacy OT networks.

### Step C1: Probe unit ID 1 on port 5030

```
mbpoll -t 4 -r 1 -c 10 -1 -a 1 localhost -p 5030
```

Flag reference:
- `-r 1` : starting address 1 (one-based addressing)
- `-a 1` : Modbus unit ID 1

**Important**: Legacy serial devices often use one-based addressing. Address 0 is not valid. If
you poll starting at address 0, you will receive a Modbus exception 02 (Illegal Data Address).
Always start at address 1 when probing serial devices whose addressing convention is unknown.

You should receive 7 holding register values representing conveyor and assembly line data: conveyor
speed, motor current, product count, reject count, line temperature, cycle time, and a status word.

The values will change slowly (product count increments as the line runs).

Now read the coils:

```
mbpoll -t 0 -r 1 -c 5 -1 -a 1 localhost -p 5030
```

You should receive 4 coil values: conveyor run, direction, e-stop active, and jam detected.

**Observe the response time.** This device responds in approximately 65ms (range: 45-85ms). The
jitter (variation between polls) is approximately 20ms. Compare this to the 5ms CompactLogix PLCs.
The latency comes from: Ethernet traversal to the gateway + gateway serial bus arbitration +
RS-485 serial transmission + legacy PLC processing overhead. This is a 1992 processor on a serial
bus -- response time tells you both the device age and the network topology.

Record for your asset inventory:
- Unit ID: 1
- Gateway IP: 192.168.1.20 (port 5030)
- Holding registers: 7 populated (addresses 1-7)
- Coils: 4 populated (addresses 1-4)
- Response time: approximately 65ms, jitter ±20ms

### Step C2: Probe unit ID 2 on port 5030

```
mbpoll -t 4 -r 1 -c 10 -1 -a 2 localhost -p 5030
```

You should receive 7 holding register values representing cooling water system data: supply
temperature, return temperature, flow rate, pump pressure, tank level, temperature setpoint,
and pump runtime hours.

Now read the coils:

```
mbpoll -t 0 -r 1 -c 5 -1 -a 2 localhost -p 5030
```

You should receive 4 coil values: pump 1 run, pump 2 run, low coolant alarm, and high temp alarm.

**Observe the response time.** This device responds in approximately 95ms (range: 45-145ms). The
jitter is approximately 50ms -- considerably higher than the SLC-500 at unit ID 1. This is normal
variability for a 1988 processor during a long scan cycle. The high jitter is a device
fingerprinting signal, not a network problem.

**Important note on byte order**: This device uses little-endian (CDAB) word order for multi-word
values. The SLC-500 at unit ID 1 uses big-endian. If you read a float pair from this device and
interpret it with big-endian byte order (as you would for the SLC-500), the value will appear
wrong. This is one of the most common sources of confusion when parsing data from a mixed OT
environment.

Record for your asset inventory:
- Unit ID: 2
- Gateway IP: 192.168.1.20 (port 5030)
- Holding registers: 7 populated (addresses 1-7)
- Coils: 4 populated (addresses 1-4)
- Response time: approximately 95ms, jitter ±50ms
- Byte order: little-endian (CDAB)

### Step C3: Probe unit ID 3 to establish the upper boundary

```
mbpoll -t 4 -r 1 -c 10 -1 -a 3 localhost -p 5030
```

You should receive a Modbus exception 0x0B (Gateway Target Device Failed to Respond). No device
is present at unit ID 3.

**Stopping rule**: Continue probing until you receive exception 0x0B twice consecutively. One
failure could be a timeout; two consecutive failures at consecutive unit IDs is a reliable
indicator that you have passed the end of the device chain.

Probe unit ID 4 to confirm:

```
mbpoll -t 4 -r 1 -c 10 -1 -a 4 localhost -p 5030
```

Again exception 0x0B. You have confirmed the boundary. Two serial devices exist: unit ID 1 and
unit ID 2.

---

## Phase D: Asset Documentation

You have now discovered all 6 devices in the facility. Fill in the asset inventory template at
`reference/asset-inventory-template.md`.

Your inventory should capture:

1. **All 4 IP-addressable devices**: 3 CompactLogix PLCs (ports 5020-5022) and the Moxa NPort
   gateway (port 5030). Each has an IP address on either the water treatment network or the
   manufacturing flat network.

2. **Both serial devices**: SLC-500 (unit ID 1) and Modicon 984 (unit ID 2), each reachable via
   gateway IP 192.168.1.20. These devices have no IP address of their own.

3. **Response times and jitter** for each device -- this data is useful for baseline deviation
   detection in later scenarios.

4. **Network topology**: which network each device belongs to, and the cross-plant link that
   connects the two networks.

When complete, compare your inventory against `reference/expected-asset-inventory.md` to verify
you have found everything.

---

## Phase E: Dashboard-Assisted Discovery

You have completed manual discovery using nmap and mbpoll. The monitoring module performs the same
discovery automatically -- connecting to each configured endpoint, enumerating registers, measuring
response times, and building an asset inventory. This phase walks you through the dashboard to see
what the monitor found and to understand how automated monitoring compares to manual enumeration.

**Starting condition**: The monitoring module must be running alongside the plant simulation. If
you started the plant with `docker compose --profile water up`, restart with both profiles:

```
docker compose --profile water --profile monitor up
```

Wait approximately 10-15 seconds for the monitor to complete its initial discovery scan.

### Step E1: Open the monitoring dashboard

Open a browser and navigate to:

```
http://localhost:8090
```

You should see the Overview page. The header shows the current environment name
("greenfield-water-mfg") and a summary count. Expect to see 6 devices reported as online.

**Teaching point**: The monitor discovers devices by actively polling each configured endpoint --
the same technique you used manually with mbpoll. The difference is that the monitor does this
continuously and records every response to build a behavioral history. Your manual polling was a
point-in-time snapshot. The monitor's picture improves with every polling cycle.

### Step E2: Navigate to the Assets page

Click "Assets" in the navigation bar, or go directly to:

```
http://localhost:8090/assets
```

You should see an asset inventory grouped by environment. The greenfield-water-mfg environment
should list all 6 devices:

| Device | Access Path | Status |
|--------|-------------|--------|
| wt-plc-01 | 10.10.10.10:5020 | Online |
| wt-plc-02 | 10.10.10.11:5021 | Online |
| wt-plc-03 | 10.10.10.12:5022 | Online |
| mfg-gateway-01 | 192.168.1.20:5030 | Online |
| mfg-plc-01 | via mfg-gateway-01, unit 1 | Online |
| mfg-plc-02 | via mfg-gateway-01, unit 2 | Online |

Notice that the monitor correctly identified the two serial PLCs as distinct devices accessed
through the gateway. It enumerated unit IDs 1 and 2 automatically, found no response at unit ID 3,
and stopped -- the same stopping rule you applied manually in Phase C.

**Compare to your manual inventory**: Do the devices match? The monitor should have found the same
6 devices you documented in Phase D. If there is a discrepancy, your manual inventory or the
monitor configuration has a gap.

**Teaching point**: The monitor's network position determines what it can see. It joins the same
networks as the devices it monitors. A monitoring tool placed only on the water treatment Level 1
network (10.10.10.0/24) would not reach the manufacturing gateway on 192.168.1.0/24 without
explicit network access. Network segmentation limits not just attackers -- it limits monitoring
tools too.

### Step E3: View live register values

Click on any device row to open the device detail view. The live register values update every 2
seconds via HTMX polling. Observe the following for wt-plc-01 (intake PLC):

- Register 0 (intake_flow_rate): changing slowly, range approximately 30-70 L/s
- Register 1 (intake_pump_speed): changing slowly, range approximately 50-80%
- Register 2 (raw_water_ph): changing slowly, range approximately 6.0-7.5 pH
- Coil 0 (intake_pump_01_run): value 1 (pump running)
- Coil 1 (intake_pump_02_run): value 1 (pump running)

These values change with each polling cycle because the plant simulation (SOW-003.0) applies drift
and noise to all process values. The monitor records each value with a timestamp, building a
time-series history that will become the baseline for anomaly detection.

### Step E4: Follow the Design Library cross-link

On the device detail page, look for a "View Atom" button or a "Reference" link next to the device
identification. Click it to navigate to the design-layer specification for this device type.

The Design Library page for compactlogix-l33er shows the full device atom YAML: vendor, model,
connectivity, register capabilities, and all register map variants. The page is labeled
"Reference" -- it is documentation about what the device is designed to do, not what the monitor
has observed.

**Teaching point**: This is the ADR-005 D4 boundary in action. The "Observed" data (live register
values on the asset detail page) comes from the monitor's network polling -- the monitor has no
knowledge of what the registers are supposed to contain. The "Reference" data (device atom YAML on
the design library page) is documentation describing the device's design specifications. A real
security tool has no equivalent of the design library -- it only has what it can observe on the
wire. The design library is an educational feature of this simulator, not a feature of real OT
monitoring tools.

You can navigate directly to the design library at:

```
http://localhost:8090/design/devices
http://localhost:8090/design/environments
```

### Step E5: View the environment definition

Navigate to the environment detail page:

```
http://localhost:8090/design/environments/greenfield-water-mfg
```

This page shows the environment.yaml -- the complete facility description including all placements,
network assignments, IP addresses, and port mappings. Compare the placements table to your manual
asset inventory from Phase D.

Notice that the environment definition specifies the register map variant for each device. This
is how the plant simulation knows which register layout to expose on each port. The monitor does
NOT use this information for polling or anomaly detection -- it discovers register counts empirically
by reading registers and observing where values stop. The environment definition is reference
documentation only.

### Step E6: Navigate to the Alerts page

Click "Alerts" in the navigation bar, or go directly to:

```
http://localhost:8090/alerts
```

Initially you should see no alerts, or possibly a brief period of baseline learning notices. The
monitor has just started and has not yet established behavioral baselines for any device.

The baseline status for each device should display as "Learning" on the asset page. The monitor
is collecting register values across polling cycles to calculate a mean and standard deviation for
each register. Once it has gathered enough samples (default: 5 minutes of polling), the status will
transition to "Established."

**Teaching point**: Anomaly detection requires a known-good baseline. The monitor cannot detect
abnormal behavior until it has observed enough normal behavior to characterize it. The baseline
learning period is the time window during which the monitor is building this characterization.
During this period, anomaly alerts are suppressed -- the monitor is not yet able to distinguish
abnormal from normal. This is an important operational consideration: if an attacker acts during
the baseline learning period, the attack will be incorporated into the baseline as normal behavior.

### Step E7: Observe the baseline transition

Wait for the baseline learning period to complete (approximately 5 minutes from startup). During
this time, continue polling devices manually with mbpoll to observe the same values the monitor
is recording. When the baseline learning period ends:

- The baseline status on the asset page transitions from "Learning" to "Established"
- The Alerts page becomes active for anomaly detection
- Register values that deviate significantly from the established baseline will generate alerts

**Optional exercise**: After baselines are established, write an unexpected value to a writable
register using mbpoll. For example, stop an intake pump by writing 0 to coil 0 on port 5020:

```
mbpoll -t 0 -r 0 -c 1 -1 -0 localhost -p 5020 -- 0
```

Then check the Alerts page. Within one polling cycle (approximately 2 seconds), an anomaly alert
should appear indicating an unexpected coil state change on wt-plc-01. This demonstrates the
monitor detecting a change that would be invisible without continuous polling.

**Note on rules of engagement**: Writing to a coil in this step intentionally triggers an alert.
Restore the original value after observing the alert:

```
mbpoll -t 0 -r 0 -c 1 -1 -0 localhost -p 5020 -- 1
```

**Teaching point**: The monitoring dashboard performs the same discovery that you did manually in
Phases A-D, but continuously and with historical context. Manual enumeration is a point-in-time
photograph. Continuous monitoring is a video. The photograph tells you what exists; the video
tells you how behavior changes over time -- and that is where anomalies become visible.

---

## Hints

If you are stuck at any point, read the hints in progressive order:

- `hints/hint-1.md` -- gentle nudge (read this first)
- `hints/hint-2.md` -- explains the underlying concept
- `hints/hint-3.md` -- gives the specific technique

The solution at `solutions/solution.md` contains the complete command-by-command walkthrough with
expected outputs.

---

## Learning Objectives Summary

After completing this scenario, you should be able to explain:

1. Why serial devices are invisible to IP-based network scanning.
2. How Modbus unit ID routing works through a serial-to-Ethernet gateway.
3. How response time and jitter differ between modern Ethernet PLCs and legacy serial PLCs.
4. Why one-based addressing is used by some legacy devices and how to detect it.
5. How byte order (endianness) differences between device families can cause incorrect data
   interpretation.
6. Why a device with no Modbus coils is likely a gateway or sensor rather than a controller.
7. Why Modbus TCP runs on port 502 in production and what the simulator port mapping means.
8. How the monitoring dashboard automates the discovery process performed manually in Phases A-D.
9. Why a monitoring tool's network position determines what devices it can observe.
10. The distinction between "Observed" data (live network monitoring) and "Reference" data (design
    layer documentation) and why real security tools only have the former.
11. Why anomaly detection requires a baseline learning period and what risks exist during that
    period.
