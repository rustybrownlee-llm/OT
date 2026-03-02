# Solution: Scenario 01 - OT Asset Discovery

This document is the complete reference walkthrough. All commands use `mbpoll`. Expected output
patterns use ranges, not exact values, because the SOW-003.0 process simulation produces drift and
noise. Any value within the stated range is correct.

---

## Prerequisites

Verify the plant simulation is running before starting:

```
mbpoll -t 4 -r 0 -c 1 -1 localhost -p 5020
```

If you receive a value between 0 and 32767, the plant is running. If you get a connection refused
error, start the plant binary first.

---

## Phase A: Network Reconnaissance

### A1: Scan for open ports (simulator-specific range)

```
nmap -sV -p 5020-5039 localhost
```

Expected output:

```
PORT     STATE  SERVICE
5020/tcp open   unknown
5021/tcp open   unknown
5022/tcp open   unknown
5023/tcp closed unknown
...
5030/tcp open   unknown
5031/tcp closed unknown
...
```

**Result**: 4 open ports -- 5020, 5021, 5022, and 5030. Ports 5023-5029 and 5031-5039 are closed.

**Production note**: In a real OT facility, scan for port 502, not 5020. The port range 5020-5039
is specific to this simulator and does not occur in production Modbus TCP deployments.

---

## Phase B: Modbus Enumeration

### B1: Enumerate port 5020

Read holding registers 0-9:

```
mbpoll -t 4 -r 0 -c 10 -1 localhost -p 5020
```

Expected output (values vary with simulation time):

```
-- Polling slave 1 (unit ID 1) on /dev/localhost:5020... (1 poll)
[0]: 16100    (example: intake_flow_rate, ~49 L/s)
[1]: 21800    (example: intake_pump_speed, ~67%)
[2]: 14700    (example: raw_water_ph, ~6.3 pH)
[3]: 4200     (example: raw_water_turbidity, ~13 NTU)
[4]: 9800     (example: intake_water_temp, ~12 degC)
[5]: 0
[6]: 0
[7]: 0
[8]: 0
[9]: 0
```

Addresses 0-4 contain process data. Addresses 5-9 return zero (no registers defined there).

Read coils 0-9:

```
mbpoll -t 0 -r 0 -c 10 -1 localhost -p 5020
```

Expected output:

```
[0]: 1   (intake_pump_01_run: pump running)
[1]: 1   (intake_pump_02_run: pump running)
[2]: 0   (screen_wash_active: no wash cycle)
[3]: 0   (low_well_level_alarm: no alarm)
[4]: 0
...
[9]: 0
```

Coils 0-3 contain device status. Addresses 4-9 return zero.

**Device identified**: Allen-Bradley CompactLogix L33ER, water intake PLC.
**Summary**: 5 holding registers (addr 0-4), 4 coils (addr 0-3), ~5ms response.

---

### B2: Enumerate port 5021

Read holding registers 0-9:

```
mbpoll -t 4 -r 0 -c 10 -1 localhost -p 5021
```

Expected output (values vary):

```
[0]: 13100    (filter_inlet_pressure, ~200 kPa)
[1]: 12800    (filter_outlet_pressure, ~195 kPa)
[2]: 820      (filter_differential_pressure, ~1.3 kPa -- low, filter is clean)
[3]: 27500    (uv_intensity, ~84 mW/cm2)
[4]: 11200    (chemical_feed_rate, ~170 mL/min)
[5]: 3900     (chlorine_residual, ~0.60 mg/L)
[6]: 290      (turbidity_post_filter, ~0.04 NTU -- good, well below 1 NTU)
[7]: 0
[8]: 0
[9]: 0
```

Read coils 0-9:

```
mbpoll -t 0 -r 0 -c 10 -1 localhost -p 5021
```

Expected output:

```
[0]: 0   (filter_backwash_command: not backwashing)
[1]: 1   (chemical_feed_pump_run: dosing pump on)
[2]: 1   (uv_system_active: UV energized)
[3]: 0   (high_dp_alarm: no alarm)
[4]: 0
...
```

**Device identified**: Allen-Bradley CompactLogix L33ER, water treatment PLC.
**Summary**: 7 holding registers (addr 0-6), 4 coils (addr 0-3), ~5ms response.

---

### B3: Enumerate port 5022

Read holding registers 0-9:

```
mbpoll -t 4 -r 0 -c 10 -1 localhost -p 5022
```

Expected output (values vary):

```
[0]: 22400    (clear_well_level, ~68%)
[1]: 17800    (distribution_flow_rate, ~82 L/s)
[2]: 20500    (distribution_pressure, ~437 kPa)
[3]: 4200     (residual_chlorine, ~0.64 mg/L)
[4]: 8800     (distribution_water_temp, ~11 degC)
[5]: 0
...
```

Read coils 0-9:

```
mbpoll -t 0 -r 0 -c 10 -1 localhost -p 5022
```

Expected output:

```
[0]: 1   (distribution_pump_01_run: pump on)
[1]: 0   (distribution_pump_02_run: standby)
[2]: 1   (booster_pump_run: on, supplying cross-plant)
[3]: 0
...
```

**Device identified**: Allen-Bradley CompactLogix L33ER, water distribution PLC.
**Summary**: 5 holding registers (addr 0-4), 3 coils (addr 0-2), ~5ms response.
**Note**: 3 coils, not 4. Coil count is not uniform across these PLCs.

---

### B4: Enumerate port 5030

Read holding registers 0-9:

```
mbpoll -t 4 -r 0 -c 10 -1 localhost -p 5030
```

Expected output:

```
[0]: 1        (serial_port_status: 1=online)
[1]: 3        (serial_baud_rate: 3=9600 baud)
[2]: 0        (serial_data_format: 0=8N1)
[3]: 2        (serial_mode: 2=RS-485-2wire)
[4]: 1        (active_tcp_connections: your connection)
[5]: 2847     (serial_tx_count: messages forwarded to serial)
[6]: 2841     (serial_rx_count: messages received from serial)
[7]: 0        (serial_error_count: no errors)
[8]: 312      (uptime_hours: ~13 days uptime)
[9]: 0
```

Addresses 5 and 6 (TX/RX counts) will increment with each poll. Address 8 (uptime) is fixed
between polls but increases over hours.

Attempt coils:

```
mbpoll -t 0 -r 0 -c 10 -1 localhost -p 5030
```

Expected output:

```
ERROR: Modbus exception 02 (Illegal Data Address)
```

**Device identified**: Moxa NPort 5150, serial-to-Ethernet gateway.
**Summary**: 9 holding registers (addr 0-8), 0 coils (FC01 returns exception 02), ~15ms response.
**Observation**: No coils indicates this is a gateway or monitoring device, not a controller.

**You now have 4 devices. The facility manager indicated 5-6. Continue to Phase C.**

---

## Phase C: Gateway Discovery

Port 5030 is a Modbus TCP gateway. Its register contents describe a serial port (baud rate, data
format, RS-485 mode, TX/RX counts). Serial devices behind this gateway have no IP address of their
own. They are reached by specifying both the gateway IP/port and a Modbus unit ID.

### C1: Probe unit ID 0 (establishes baseline)

The default unit ID in mbpoll is 1. Probe unit ID 0 first to understand the gateway's behavior:

```
mbpoll -t 4 -r 0 -c 10 -1 -a 0 localhost -p 5030
```

Most serial devices do not respond to unit ID 0 (it is reserved as a broadcast address). Depending
on gateway configuration, you may receive timeout or the gateway's own simulated registers.

### C2: Probe unit ID 1

```
mbpoll -t 4 -r 1 -c 10 -1 -a 1 localhost -p 5030
```

The `-a 1` flag sets Modbus unit ID 1. The `-r 1` flag starts at address 1 because this is a
one-based device. If you had used `-r 0`, you would receive exception 02.

Expected output (values vary with simulation time):

```
-- Polling slave 1 on /dev/localhost:5030...
[1]: 14200    (conveyor_speed, ~87 ft/min)
[2]: 8900     (motor_current, ~8.1 A)
[3]: 1247     (product_count: 1247 units today -- incrementing)
[4]: 23       (reject_count: 23 rejected)
[5]: 14800    (line_temperature, ~91 degF)
[6]: 5300     (cycle_time, ~19 s)
[7]: 1        (status_word: bit0=1, machine running)
[8]: 0
[9]: 0
```

Note that `product_count` (address 3) should increase between polls -- the line is running.

Read coils at unit ID 1:

```
mbpoll -t 0 -r 1 -c 5 -1 -a 1 localhost -p 5030
```

Expected output:

```
[1]: 1   (conveyor_run: running)
[2]: 0   (conveyor_direction: forward)
[3]: 0   (e_stop_active: e-stop not tripped)
[4]: 0   (jam_detected: no jam)
[5]: 0
```

**Observe the response time.** This response arrives approximately 65ms after your request, with
variation of ±20ms (45-85ms range). Compare to the 5ms of the CompactLogix PLCs. The latency
reflects serial bus round-trip time and legacy processor overhead.

**Device identified**: Allen-Bradley SLC 500-05, manufacturing line A controller.
**Summary**: 7 holding registers (addr 1-7), 4 coils (addr 1-4), one-based addressing,
big-endian byte order, ~65ms response via gateway.

### C3: Probe unit ID 2

```
mbpoll -t 4 -r 1 -c 10 -1 -a 2 localhost -p 5030
```

Expected output (values vary):

```
[1]: 17800    (supply_temp, ~81 degF)
[2]: 20400    (return_temp, ~89 degF)
[3]: 16200    (flow_rate, ~247 GPM)
[4]: 11500    (pump_pressure, ~28 PSI)
[5]: 22900    (tank_level, ~70%)
[6]: 14700    (setpoint_temp, ~65 degF)
[7]: 52       (pump_runtime_hours: 52 hours since last power cycle)
[8]: 0
...
```

Read coils at unit ID 2:

```
mbpoll -t 0 -r 1 -c 5 -1 -a 2 localhost -p 5030
```

Expected output:

```
[1]: 1   (pump_1_run: lead pump on)
[2]: 0   (pump_2_run: lag pump off)
[3]: 0   (low_coolant_alarm: tank level OK)
[4]: 0   (high_temp_alarm: temperature OK)
[5]: 0
```

**Observe the response time.** This device responds in approximately 95ms (range: 45-145ms).
The jitter is approximately ±50ms -- much higher than the SLC-500 at unit ID 1. This variability
is normal for a 1988 processor with variable scan cycle timing. High jitter is a device age signal.

**Important: byte order difference.** The Modicon 984 uses little-endian (CDAB) word order. For
single 16-bit integer registers, byte order does not matter. For 32-bit float pairs spanning two
registers, you must apply CDAB decoding or values will be wrong. The SLC-500 uses big-endian.
Applying big-endian decoding to Modicon float data produces incorrect results.

**Device identified**: Schneider Electric Modicon 984, manufacturing cooling system controller.
**Summary**: 7 holding registers (addr 1-7), 4 coils (addr 1-4), one-based addressing,
little-endian (CDAB) byte order, ~95ms response via gateway.

### C4: Confirm the device boundary

```
mbpoll -t 4 -r 1 -c 5 -1 -a 3 localhost -p 5030
```

Expected output:

```
ERROR: Modbus exception 0x0B (Gateway Target Device Failed to Respond)
```

The gateway attempted to reach a serial device at unit ID 3 and received no response.

```
mbpoll -t 4 -r 1 -c 5 -1 -a 4 localhost -p 5030
```

Again exception 0x0B. Two consecutive failures confirm there are no more serial devices. The
device chain ends at unit ID 2.

---

## Phase D: Asset Documentation

You have found all 6 devices:

| # | Identifier | Type | Access Path |
|---|-----------|------|-------------|
| 1 | wt-plc-01 | CompactLogix PLC | localhost:5020 (IP: 10.10.10.10) |
| 2 | wt-plc-02 | CompactLogix PLC | localhost:5021 (IP: 10.10.10.11) |
| 3 | wt-plc-03 | CompactLogix PLC | localhost:5022 (IP: 10.10.10.12) |
| 4 | mfg-gateway-01 | Moxa NPort Gateway | localhost:5030 (IP: 192.168.1.20) |
| 5 | mfg-plc-01 | SLC-500 PLC | 192.168.1.20:5030, unit ID 1 (no IP) |
| 6 | mfg-plc-02 | Modicon 984 PLC | 192.168.1.20:5030, unit ID 2 (no IP) |

Fill in `reference/asset-inventory-template.md` and compare against
`reference/expected-asset-inventory.md`.

---

## Teaching Points Summary

This scenario demonstrates the following OT security concepts:

**1. Serial devices are invisible to network scanning.**
The SLC-500 and Modicon 984 have no IP addresses. No nmap scan, no ping sweep, no ARP table
inspection will reveal them. The only discovery method is Modbus unit ID probing through their
gateway. This is the most common discovery gap for IT engineers entering OT environments.

**2. Modbus unit ID routing.**
The gateway is a transparent Modbus TCP-to-RTU bridge. It passes requests through to the serial
bus unchanged. The unit ID field in every Modbus message is the addressing mechanism for serial
devices. Unit IDs 1-247 are valid; probe sequentially and stop when you see exception 0x0B twice.

**3. Response time as a fingerprinting signal.**
- Modern Ethernet PLC (CompactLogix): ~5ms, ±3ms jitter
- Gateway overhead: ~15ms
- Serial PLC via gateway (SLC-500, 1992): ~65ms, ±20ms jitter
- Very old serial PLC via gateway (Modicon 984, 1988): ~95ms, ±50ms jitter
Response time reveals device age, network topology, and whether a device is directly connected
or behind a gateway.

**4. One-based addressing on legacy devices.**
Polling address 0 on the SLC-500 or Modicon 984 returns exception 02. These devices use one-based
Modbus addressing -- the first valid register is address 1. Always probe from address 1 when
working with unknown legacy devices.

**5. Byte order variations.**
The SLC-500 uses big-endian byte order. The Modicon 984 uses little-endian (CDAB). Two devices on
the same serial bus can use different byte orders. Applying the wrong byte order to a multi-word
value produces silently incorrect data -- no error, just a wrong number.

**6. No coils implies no physical control.**
A device that responds to holding register reads but returns exception 02 on coil reads has no
output coils. Physical equipment always has coils for actuators (motor run, valve open). A Modbus
device with no coils is a gateway, sensor, or monitoring device -- not a controller.

**7. Port 502 is the real Modbus TCP port.**
In production, Modbus TCP always runs on port 502. The ports 5020-5039 used by this simulator
exist only to avoid requiring root privileges. In a real facility, scan for port 502.

---

## Verification Checklist

Before closing this scenario, verify:

- [ ] Asset inventory has exactly 6 devices
- [ ] All 4 IP addresses are recorded (10.10.10.10, 10.10.10.11, 10.10.10.12, 192.168.1.20)
- [ ] Both serial devices list "no IP address" with gateway path instead
- [ ] Register counts match: 5/7/5/9/7/7 (devices 1-6 in order)
- [ ] wt-plc-03 has 3 coils, not 4
- [ ] SLC-500 is marked as one-based, big-endian
- [ ] Modicon 984 is marked as one-based, little-endian (CDAB)
- [ ] Response times recorded: 5ms / 5ms / 5ms / 15ms / 65ms / 95ms
- [ ] Network topology diagram shows the cross-plant link

---

## Phase E: Dashboard-Assisted Discovery

This phase requires the monitoring module to be running. Restart the compose stack with the
monitor profile if it is not already active:

```
docker compose --profile water --profile monitor up
```

Wait 10-15 seconds for the initial discovery scan to complete.

### E1: Verify the monitor has discovered all 6 devices

Open the monitoring dashboard:

```
http://localhost:8090
```

Expected: Overview page shows 6 devices online.

Navigate to the Assets page:

```
http://localhost:8090/assets
```

Expected output (all rows present, all status Online):

```
Environment: greenfield-water-mfg

wt-plc-01     | 10.10.10.10:5020         | CompactLogix L33ER | Online
wt-plc-02     | 10.10.10.11:5021         | CompactLogix L33ER | Online
wt-plc-03     | 10.10.10.12:5022         | CompactLogix L33ER | Online
mfg-gateway-01| 192.168.1.20:5030        | Moxa NPort 5150    | Online
mfg-plc-01    | via gateway, unit ID 1   | SLC 500-05         | Online
mfg-plc-02    | via gateway, unit ID 2   | Modicon 984        | Online
```

**Compare to manual inventory**: The 6 devices should match exactly. The monitor discovered
the same devices you found manually in Phases A-D.

### E2: Verify live register values on a water treatment PLC

Click on the wt-plc-01 row to open the device detail page. The register table should show
all 5 holding registers and 4 coils, with values updating every 2 seconds.

Expected register values (approximate, values drift over time):

```
HR[0] intake_flow_rate:    ~16000-19000  (range 0-32767 maps to 0-100 L/s)
HR[1] intake_pump_speed:   ~20000-24000  (range 0-32767 maps to 0-100%)
HR[2] raw_water_ph:        ~13000-18000  (range 0-32767 maps to 0-14 pH)
HR[3] raw_water_turbidity: ~3000-6000    (range 0-32767 maps to 0-100 NTU)
HR[4] intake_water_temp:   ~7000-12000   (range 0-32767 maps to 0-40 degC)

Coil[0] intake_pump_01_run:   1 (running)
Coil[1] intake_pump_02_run:   1 (running)
Coil[2] screen_wash_active:   0 (inactive)
Coil[3] low_well_level_alarm: 0 (no alarm)
```

Values should match what you observed when polling port 5020 with mbpoll.

### E3: Follow the Design Library cross-link

From the wt-plc-01 detail page, click "View Atom" or the device type link to navigate to:

```
http://localhost:8090/design/devices/compactlogix-l33er
```

Expected: The page shows the full compactlogix-l33er.yaml with syntax highlighting. Scroll
through the YAML to verify the water-intake register map variant is listed with addresses 0-4
holding registers and addresses 0-3 coils -- matching exactly what the monitor observed.

**Note the page label**: This page should display a "Reference" badge or label to distinguish it
from live monitoring data. The design library shows the device specification, not the monitor's
observations.

Navigate to the environment definition:

```
http://localhost:8090/design/environments/greenfield-water-mfg
```

Expected: The environment.yaml shows all 6 placements with their network assignments, IP
addresses, and Modbus ports.

### E4: Check Alerts page and baseline status

Navigate to:

```
http://localhost:8090/alerts
```

Immediately after monitor startup, you should see no anomaly alerts. The baseline status on the
asset page should show "Learning" for all devices. The monitor is collecting polling data but
has not yet established behavioral baselines.

### E5 (Optional): Trigger an anomaly after baseline establishment

Wait for the baseline learning period to complete (approximately 5 minutes). When baseline status
transitions to "Established," write an unexpected value to a writable coil:

Stop intake pump 1 on wt-plc-01:

```
mbpoll -t 0 -r 0 -c 1 -1 -0 localhost -p 5020 -- 0
```

Wait approximately 2-4 seconds (one to two polling cycles), then check the Alerts page:

```
http://localhost:8090/alerts
```

Expected: An anomaly alert appears indicating an unexpected coil state change on wt-plc-01,
coil address 0 (intake_pump_01_run). Severity: High (unexpected write).

Restore the coil to its original state:

```
mbpoll -t 0 -r 0 -c 1 -1 -0 localhost -p 5020 -- 1
```

**What you have demonstrated**: The monitor detected a one-bit change (pump off vs pump on) on
a single coil within one polling cycle -- without any prior knowledge of what the normal value
should be, other than the baseline it observed during the learning period.

### Phase E Summary

| Manual Method (Phases A-D) | Dashboard Method (Phase E) |
|---------------------------|---------------------------|
| nmap scan: found 4 ports | Monitor: polled all configured endpoints |
| mbpoll per-device: found 6 devices | Monitor: enumerated all 6 devices automatically |
| One-time register snapshot | Continuous polling, 2-second interval |
| Discovered on first run | Baseline established after learning period |
| Manual comparison of values | Automated anomaly detection against baseline |
| No alert on unexpected change | Alert generated within 2 seconds of change |

The monitoring dashboard is not a replacement for manual discovery skills. Understanding manual
techniques is what allows you to interpret monitoring data correctly, design effective monitoring
configurations, and diagnose monitoring gaps. The two approaches are complementary, not
alternatives.

---

## Phase F: Hybrid Environment Discovery

This phase requires the wastewater environment profile and the monitor profile running:

```
docker compose --profile wastewater --profile monitor up
```

Wait 10-15 seconds for the initial scan to complete.

### F1: Verify wastewater environment is running

```
nmap -sV -p 5062-5064 localhost
```

Expected output:

```
PORT     STATE  SERVICE
5062/tcp open   unknown
5063/tcp open   unknown
5064/tcp open   unknown
```

**Result**: 3 open ports. The facility manager indicated approximately 5 devices. The same
discrepancy from Phase A applies: IP scanning does not find serial devices.

**Production note**: In the real wastewater facility, all devices would respond on port 502.
The ports 5062-5064 are simulator-specific. Port 5062 = CompactLogix aeration, 5063 = Moxa
gateway, 5064 = Cradlepoint cellular gateway.

---

### F2: Enumerate port 5062 (CompactLogix, zero-based)

Read input registers starting at address 0:

```
mbpoll -a 1 -t 3 -r 0 -c 12 localhost -p 5062
```

Expected output (values vary with simulation time):

```
[0]:  1         (blower_run_state: 1=running)
[1]:  21000     (blower_speed_pct: ~64%)
[2]:  18500     (air_flow_rate: ~565 scfm -- scaled 0-32767 = 0-1000 scfm)
[3]:  14200     (dissolved_oxygen: ~4.3 mg/L -- scaled 0-32767 = 0-10 mg/L)
[4]:  9800      (tank_temperature: ~12 degC -- scaled 0-32767 = 0-40 degC)
[5]:  12200     (air_pressure: ~37 kPa -- scaled)
[6]:  1         (blower_fault_state: 0=no fault)
[7]:  0         (inlet_valve_open: valve state)
[8]:  0         (outlet_valve_open: valve state)
[9]:  22000     (dissolved_oxygen_setpoint: ~6.7 mg/L -- writable)
[10]: 19000     (blower_speed_setpoint: ~58% -- writable)
[11]: 3847      (blower_run_hours: accumulating total -- the predictive maintenance data point)
```

The value at address 11 (`blower_run_hours`) is the data the Cradlepoint was installed to
transmit to the blower vendor. It accumulates over the life of the device.

**Device identified**: Allen-Bradley CompactLogix L33ER, aeration blower PLC (installed 2013).
**Summary**: Zero-based addressing, 12 registers at addresses 0-11, approximately 5ms response.

---

### F3: Identify the Cradlepoint at port 5064

Before examining the expected gateway (port 5063), probe the unknown endpoint at port 5064:

```
mbpoll -a 1 -t 3 -r 0 -c 7 localhost -p 5064
```

Expected output:

```
[0]:  1         (wan_connected: 1=connected to LTE network)
[1]:  23        (signal_strength_dbm: -23 dBm RSRP -- strong signal)
[2]:  3         (network_type: 3=LTE/4G)
[3]:  1247      (data_usage_mb: monthly usage, incrementing)
[4]:  1         (nat_enabled: 1=NAT active, internet-facing)
[5]:  0         (vpn_connected: 0=no VPN tunnel)
[6]:  14        (uptime_days: 14 days since last reboot)
```

The register values describe a cellular network management device, not a PLC or gateway. WAN
connected, LTE signal, NAT enabled, internet-facing, no VPN.

**Important**: This is an educational abstraction (TD-038). The real Cradlepoint IBR600 does
not run a Modbus TCP server. Its management interface uses HTTPS on port 443 and SNMP. The
simulator exposes these values via Modbus TCP to make the device discoverable and inspectable
with standard tools. The security findings from this device (HTTPS default credentials
admin/admin, internet exposure, NAT traversal) are covered in Scenario 02, Phase E.

**Device identified**: Cradlepoint IBR600 cellular gateway (installed 2022).
**Summary**: Not in any facility documentation. Internet-connected. No VPN. Default credentials
on HTTPS management interface never changed.

---

### F4: Enumerate the Moxa gateway at port 5063

Read the gateway's own status registers:

```
mbpoll -a 1 -t 3 -r 0 -c 9 localhost -p 5063
```

Expected output:

```
[0]:  1         (serial_port_status: 1=online)
[1]:  3         (serial_baud_rate: 3=9600 baud)
[2]:  0         (serial_data_format: 0=8N1)
[3]:  2         (serial_mode: 2=RS-485-2wire)
[4]:  1         (active_tcp_connections: your connection)
[5]:  4198      (serial_tx_count: messages forwarded to serial)
[6]:  4191      (serial_rx_count: messages received from serial)
[7]:  2         (serial_error_count: 2 errors in uptime period)
[8]:  52        (uptime_hours: ~2 days uptime)
```

No coils (FC01 returns exception 02). This is a gateway with no output actuators.

**Device identified**: Moxa NPort serial-to-Ethernet gateway (installed 2008).
**Summary**: 9 holding registers describing a serial port. Device category confirmed: gateway.

---

### F5: Probe SLC-500 unit IDs through the gateway

The SLC-500s on the DH-485 bus use one-based Modbus addressing via the ProSoft MVI46-MCM
modules. Start at address 1.

**Demonstrate the addressing difference first** -- attempt address 0 to confirm exception:

```
mbpoll -a 1 -t 3 -r 0 -c 5 localhost -p 5063
```

Expected output:

```
ERROR: Modbus exception 02 (Illegal Data Address)
```

Address 0 does not exist on the SLC-500. Now read from address 1:

```
mbpoll -a 1 -t 3 -r 1 -c 9 localhost -p 5063
```

Expected output (values vary):

```
[1]:  18200     (influent_flow_rate: ~840 m3/h -- scaled)
[2]:  3         (screen_differential_pressure: ~0.3 kPa -- low, screen clean)
[3]:  1         (screen_motor_run: 1=running)
[4]:  0         (screen_blockage_alarm: 0=no alarm)
[5]:  22000     (influent_pump_01_speed: ~67%)
[6]:  0         (influent_pump_02_speed: 0=standby)
[7]:  1         (pump_01_run: 1=lead pump running)
[8]:  0         (pump_02_run: 0=lag pump standby)
[9]:  0         (high_flow_alarm: 0=normal)
```

**Observe the response time.** This response arrives approximately 65-95ms after your request.
The DH-485 bus uses token-passing arbitration, which adds latency beyond simple RS-485 polling.

**Device identified**: Allen-Bradley SLC-500 (influent screening, installed 1997).
**Summary**: One-based addressing. 9 registers at addresses 1-9. Approximately 65-95ms response.

Now probe unit ID 2 (effluent SLC-500):

```
mbpoll -a 2 -t 3 -r 1 -c 8 localhost -p 5063
```

Expected output (values vary):

```
[1]:  17500     (effluent_flow_rate: ~805 m3/h -- scaled)
[2]:  3         (effluent_turbidity: ~0.09 NTU -- scaled 0-32767 = 0-10 NTU)
[3]:  4100      (effluent_ph: ~6.3 pH -- scaled 0-32767 = 0-14 pH)
[4]:  2900      (dissolved_oxygen: ~0.9 mg/L -- scaled)
[5]:  8800      (effluent_temperature: ~10.8 degC -- scaled)
[6]:  1         (discharge_pump_run: 1=pump running)
[7]:  0         (effluent_quality_alarm: 0=within limits)
[8]:  0         (discharge_valve_open: valve state)
```

**Device identified**: Allen-Bradley SLC-500 (effluent discharge, installed 1997).
**Summary**: One-based addressing. 8 registers at addresses 1-8. Approximately 65-95ms response.

Confirm the boundary:

```
mbpoll -a 3 -t 3 -r 1 -c 5 localhost -p 5063
```

Expected:

```
ERROR: Modbus exception 0x0B (Gateway Target Device Failed to Respond)
```

```
mbpoll -a 4 -t 3 -r 1 -c 5 localhost -p 5063
```

Again exception 0x0B. Device chain ends at unit ID 2. Two SLC-500s present.

---

### F5 Complete: Addressing contrast summary

| Device | Port | Unit ID | Start Address | Why |
|--------|------|---------|---------------|-----|
| CompactLogix (aeration) | 5062 | 1 | 0 | Ethernet-native, zero-based |
| Moxa gateway | 5063 | 1 | 0 | Gateway registers, zero-based |
| SLC-500 (influent) | 5063 | 1 | 1 | ProSoft MVI46-MCM, one-based |
| SLC-500 (effluent) | 5063 | 2 | 1 | ProSoft MVI46-MCM, one-based |
| Cradlepoint | 5064 | 1 | 0 | Abstracted registers, zero-based |

The one-based/zero-based contrast is the most operationally significant technical difference
between the 1997 SLC-500 hardware and the 2013 CompactLogix. Both use Modbus, but they use it
differently. A polling configuration that works for the CompactLogix will fail silently on the
SLC-500 if it starts at address 0.

---

### F6: Dashboard comparison

Open the monitoring dashboard:

```
http://localhost:8090/assets
```

Expected output for the brownfield-wastewater environment:

```
Environment: brownfield-wastewater

ww-plc-01      | localhost:5062           | CompactLogix L33ER   | Online
ww-gateway-01  | localhost:5063           | Moxa NPort 5150      | Online
ww-plc-02      | via gateway, unit ID 1   | SLC 500-05           | Online
ww-plc-03      | via gateway, unit ID 2   | SLC 500-05           | Online
ww-cradlepoint | localhost:5064           | Cradlepoint IBR600   | Online
```

All 5 devices present. The monitor scanned unit IDs 1 and 2 on the gateway, found responses at
both, found no response at unit ID 3, and stopped -- the same stopping rule you applied manually.

**Compare to nmap result**: nmap found 3 ports. The monitor found 5 devices. The 2-device gap
is the serial device gap, consistent across all OT environments with serial buses.

---

### F7-F8: Topology view

Navigate to `http://localhost:8090/topology/brownfield-wastewater`.

Expected visual characteristics:
- One solid horizontal boundary line at Level 3 (enforced by managed switch, 2018)
- Flat segment below containing all 5 operational devices
- Cradlepoint with a cellular WAN link extending outward
- Era marker badges: 1997, 1997, 2008, 2013, 2022
- No Level 2 layer (absent boundary noted)

Compare to the other environments:
- greenfield-water-mfg: 3-level vertical stack, all solid boundaries, single era (2024)
- brownfield-pipeline-station: single horizontal plane, no boundaries, satellite backhaul link
- brownfield-wastewater: partial stack (1 of 3 levels), mixed eras, internet link

The visual shape tells the security story before you read a single register.

---

### F9: Era marker analysis

The five devices span 25 years of installation history. The threat model collision is visible in
the era markers:

| Device | Year | Threat Model of Its Era |
|--------|------|------------------------|
| SLC-500 (x2) | 1997 | Physical serial bus access is the security boundary |
| Moxa gateway | 2008 | IP network access is the security boundary |
| CompactLogix | 2013 | Managed switch segmentation available but not used here |
| Cradlepoint | 2022 | Internet-connected, cloud-managed, remote access by design |

The 2022 Cradlepoint's design assumption is internet connectivity. The 1997 SLC-500's design
assumption is a closed serial bus. Placing them on the same flat network means the SLC-500
inherits the Cradlepoint's threat model -- and the SLC-500 has no defenses against it.

---

### F10: Asset inventory update

Your completed wastewater inventory should contain:

| # | Identifier | Type | Access Path | Addressing | Era |
|---|-----------|------|-------------|------------|-----|
| 1 | ww-plc-01 | CompactLogix L33ER | localhost:5062 | Zero-based | 2013 |
| 2 | ww-gateway-01 | Moxa NPort | localhost:5063 | Zero-based | 2008 |
| 3 | ww-plc-02 | SLC-500 (influent) | 5063 unit ID 1 (no IP) | One-based | 1997 |
| 4 | ww-plc-03 | SLC-500 (effluent) | 5063 unit ID 2 (no IP) | One-based | 1997 |
| 5 | ww-cradlepoint | Cradlepoint IBR600 | localhost:5064 | Zero-based (abstracted) | 2022 |

Serial backbone note: DH-485 (Allen-Bradley proprietary, chassis backplane) bridged by ProSoft
MVI46-MCM (one-based Modbus RTU) bridged by Moxa NPort (Modbus TCP). Only the Moxa layer is
accessible to standard tools.

Internet exposure note: Only the Cradlepoint. Cellular WAN, LTE, NAT enabled, no VPN.

---

### Phase F Summary

| Method | Devices Found |
|--------|--------------|
| nmap -p 5062-5064 | 3 (CompactLogix, Moxa gateway, Cradlepoint) |
| mbpoll unit ID probing | +2 (SLC-500 influent, SLC-500 effluent) |
| Monitoring dashboard | 5 (all) |

The hybrid environment demonstrates the same serial device gap as the greenfield environment,
but adds two discovery challenges that do not exist in the greenfield: (1) an undocumented
internet-connected device (Cradlepoint) that requires separate investigation and (2) a two-layer
serial backbone (DH-485 plus ProSoft RS-485) with addressing characteristics that differ from
the Ethernet devices on the same segment.
