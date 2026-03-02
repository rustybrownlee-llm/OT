# Beta 0.2 Integration Test Results

**Date**: 2026-03-02
**Milestone**: Beta 0.2 (Pipeline Composability)
**Docker Compose Version**: Both `--profile water` and `--profile pipeline` running simultaneously
**Tool**: `mbpoll` 1.0-0 (ModBus Master Simulator)
**Host**: macOS (localhost / 127.0.0.1)

## Purpose

These tests validate that the OT Simulator's two independent environments (Greenfield Water/Manufacturing and Pipeline Monitoring Station) can run simultaneously via Docker Compose profiles, serve correct Modbus TCP register data, and exhibit realistic OT device behavior. The tests are structured to be reproducible for training purposes.

## Prerequisites

- Docker and Docker Compose installed
- `mbpoll` installed (`brew install mbpoll` on macOS)
- OT Simulator repository cloned and built

## 1. Starting the Environments

### Start Both Profiles

```bash
docker compose --profile water --profile pipeline up --build -d
```

### Verify Containers Are Healthy

```bash
docker compose ps
```

**Expected output (3 containers)**:

| Container | Service | Status | Ports |
|-----------|---------|--------|-------|
| ot-plant-water | plant-water | Up (healthy) | 5020-5022, 5030 |
| ot-plant-pipeline | plant-pipeline | Up (healthy) | 5040-5043 |
| ot-monitor | monitor | Up (healthy) | 8090-8091 |

### Verify Docker Networks

```bash
docker network ls --filter "name=ot-"
```

**Expected: 7 networks**:

| Network | Purpose | Environment |
|---------|---------|-------------|
| ot-wt-level1 | Purdue Level 1 (PLCs) | Water/Mfg |
| ot-wt-level2 | Purdue Level 2 (HMI) | Water/Mfg |
| ot-wt-level3 | Purdue Level 3 (site ops) | Water/Mfg |
| ot-mfg-flat | Legacy flat network (no Purdue) | Water/Mfg |
| ot-ps-station-lan | Flat station LAN (all station devices) | Pipeline |
| ot-ps-wan-link | WAN backhaul to SCADA master | Pipeline |
| ot-cross-plant | Cross-environment connectivity | Shared |

**Key teaching point**: The water/mfg environment uses a Purdue model (3 hierarchical levels) while the pipeline station uses a flat LAN -- this reflects real-world differences between large facilities and remote field sites.

## 2. Port Map and Addressing Reference

Understanding the port and addressing layout is essential before running any Modbus commands.

### Water/Manufacturing Environment (Ports 5020-5030)

| Port | Device | Model | Addressing | Variant | Registers |
|------|--------|-------|------------|---------|-----------|
| 5020 | wt-plc-01 | CompactLogix L33ER | Zero-based | water-intake | 5 holding |
| 5021 | wt-plc-02 | CompactLogix L33ER | Zero-based | water-treatment | 7 holding |
| 5022 | wt-plc-03 | CompactLogix L33ER | Zero-based | water-distribution | 5 holding |
| 5030 | mfg-gw-01 | Moxa NPort 5150 | Gateway | serial-gateway | Unit ID routing |

**Devices behind water/mfg gateway (port 5030)**:

| Unit ID | Device | Model | Addressing | Variant | Registers |
|---------|--------|-------|------------|---------|-----------|
| 1 | mfg-plc-01 | SLC-500/05 | One-based | mfg-line-a | 7 holding |
| 2 | mfg-plc-02 | Modicon 984 | One-based | mfg-cooling | 7 holding |
| 247 | (gateway) | Moxa NPort 5150 | N/A | diagnostics | 5 holding |

### Pipeline Environment (Ports 5040-5043)

| Port | Device | Model | Addressing | Variant | Registers |
|------|--------|-------|------------|---------|-----------|
| 5040 | ps-plc-01 | CompactLogix L33ER | Zero-based | compressor-control | 9 holding |
| 5041 | ps-rtu-01 | Emerson ROC800 | One-based | pipeline-metering | 7 holding, 4 coils |
| 5042 | ps-rtu-02 | Emerson ROC800 | One-based | station-monitoring | 8 holding, 4 coils |
| 5043 | ps-gw-01 | Moxa NPort 5150 | Gateway | pipeline-serial | Unit ID routing |

**Device behind pipeline gateway (port 5043)**:

| Unit ID | Device | Model | Addressing | Variant | Registers |
|---------|--------|-------|------------|---------|-----------|
| 1 | ps-fc-01 | ABB TotalFlow G5 | One-based | gas-analysis | 9 holding, 3 coils |
| 247 | (gateway) | Moxa NPort 5150 | N/A | diagnostics | 5 holding |

### Addressing and mbpoll Reference Numbers

This is a critical concept for anyone working with Modbus:

- **mbpoll default** (no `-0` flag): Reference 1 = wire address 0. This is the Modbus convention.
- **Zero-based devices** (CompactLogix): Device register address 0 = wire address 0 = mbpoll reference 1. Use `-r 1`.
- **One-based devices** (ROC800, SLC-500, Modicon, TotalFlow): Device register address 1 = wire address 1 = mbpoll reference 2. Use `-r 2`.

**Common mistake**: Using `-r 1` for a one-based device reads wire address 0, which has no register mapped -- you get "Illegal data address". The fix is `-r 2`.

## 3. Test Results: Water/Manufacturing Environment

### Test 3.1: Water Treatment PLCs (Direct Ethernet)

These PLCs are CompactLogix L33ER units with zero-based addressing. Each serves a different register map variant.

```bash
# wt-plc-01: Water Intake (5 holding registers)
mbpoll -1 -t 4 -r 1 -c 5 -a 1 -p 5020 127.0.0.1

# wt-plc-02: Water Treatment (7 holding registers)
mbpoll -1 -t 4 -r 1 -c 7 -a 1 -p 5021 127.0.0.1

# wt-plc-03: Water Distribution (5 holding registers)
mbpoll -1 -t 4 -r 1 -c 5 -a 1 -p 5022 127.0.0.1
```

**Results** (values drift each tick -- exact numbers will vary):

```
wt-plc-01 (water-intake):     [1]=0  [2]=16383  [3]=16071  [4]=11497  [5]=14711
wt-plc-02 (water-treatment):  [1]=3420  [2]=3073  [3]=3474  [4]=0  [5]=16383  [6]=3774  [7]=26042
wt-plc-03 (water-distribution): [1]=32767  [2]=0  [3]=23552  [4]=3930  [5]=15623
```

**Status**: PASS -- All 3 water treatment PLCs responding with simulated process data.

### Test 3.2: Legacy Manufacturing Devices via Gateway

The gateway (Moxa NPort 5150) routes Modbus requests to serial devices based on the Modbus unit ID in the request header. This is a core OT networking concept.

```bash
# SLC-500 (unit ID 1) -- one-based, use -r 2
mbpoll -1 -t 4 -r 2 -c 7 -a 1 -p 5030 127.0.0.1

# Modicon 984 (unit ID 2) -- one-based, use -r 2
mbpoll -1 -t 4 -r 2 -c 7 -a 2 -p 5030 127.0.0.1

# Gateway diagnostics (unit ID 247) -- gateway's own registers
mbpoll -1 -t 4 -r 1 -c 5 -a 247 -p 5030 127.0.0.1
```

**Results**:

```
SLC-500 (mfg-line-a):   [2]=16383  [3]=0  [4]=0  [5]=0  [6]=7070  [7]=0  [8]=0
Modicon (mfg-cooling):  [2]=8050  [3]=14222  [4]=145  [5]=4057  [6]=26230  [7]=16383  [8]=2400
Gateway (diagnostics):  [1]=0  [2]=0  [3]=0  [4]=0  [5]=0
```

**Status**: PASS -- Both serial devices respond via gateway. Unit ID routing works correctly.

**Teaching point**: The same TCP port (5030) serves three different devices depending on the unit ID. This is how real serial-to-Ethernet gateways work -- one IP:port, multiple devices behind it. Unit 247 is the gateway's own diagnostic registers (a common convention for Moxa devices).

## 4. Test Results: Pipeline Monitoring Station

### Test 4.1: Compressor Control PLC

```bash
# ps-plc-01: CompactLogix, compressor-control (9 holding registers, zero-based)
mbpoll -1 -t 4 -r 1 -c 9 -a 1 -p 5040 127.0.0.1
```

**Results**:

```
ps-plc-01 (compressor-control):
[1]=0  [2]=13267  [3]=13267  [4]=11955  [5]=10550  [6]=0  [7]=0  [8]=13106  [9]=0
```

Register map: [1] compressor_running, [2] suction_pressure, [3] discharge_pressure, [4] bearing_temp, [5] vibration, [6] speed_setpoint, [7] speed_actual, [8] inlet_guide_vane, [9] estop_active.

**Status**: PASS

### Test 4.2: Pipeline Metering RTU (ROC800)

```bash
# ps-rtu-01: ROC800, pipeline-metering (7 holding, 4 coils, one-based: -r 2)
mbpoll -1 -t 4 -r 2 -c 7 -a 1 -p 5041 127.0.0.1
mbpoll -1 -t 0 -r 2 -c 4 -a 1 -p 5041 127.0.0.1
```

**Results**:

```
Holding registers:
[2]=0  [3]=0  [4]=13419  [5]=14148  [6]=7957  [7]=0  [8]=0

Coils:
[2]=0  [3]=0  [4]=0  [5]=0
```

Register map (holding): [2] meter_run_1_flow_rate, [3] meter_run_1_volume_today, [4] meter_run_1_pressure, [5] meter_run_1_temperature, [6] meter_run_1_differential_pressure, [7] station_total_flow, [8] station_total_volume_today.

Coils: [2] meter_run_1_enabled, [3] meter_run_2_enabled, [4] meter_run_3_enabled, [5] meter_run_4_enabled.

**Status**: PASS

### Test 4.3: Station Monitoring RTU (ROC800)

```bash
# ps-rtu-02: ROC800, station-monitoring (8 holding, 4 coils, one-based: -r 2)
mbpoll -1 -t 4 -r 2 -c 8 -a 1 -p 5042 127.0.0.1
mbpoll -1 -t 0 -r 2 -c 4 -a 1 -p 5042 127.0.0.1
```

**Results**:

```
Holding registers:
[2]=13378  [3]=18800  [4]=16309  [5]=20069  [6]=0  [7]=0  [8]=32767  [9]=0

Coils:
[2]=0  [3]=0  [4]=0  [5]=0
```

Register map (holding): [2] station_inlet_pressure, [3] station_outlet_pressure, [4] station_inlet_temperature, [5] station_outlet_temperature, [6] inlet_block_valve_position, [7] outlet_block_valve_position, [8] esd_valve_position (32767 = 100% open), [9] station_status_word.

Coils: [2] esd_activate (read-only), [3] inlet_block_valve_cmd, [4] outlet_block_valve_cmd, [5] esd_active_status (read-only).

**Status**: PASS

### Test 4.4: Gas Chromatograph via Gateway (TotalFlow G5)

```bash
# ps-fc-01: TotalFlow G5 via gateway (unit 1, port 5043, one-based: -r 2)
mbpoll -1 -t 4 -r 2 -c 9 -a 1 -p 5043 127.0.0.1
mbpoll -1 -t 0 -r 2 -c 3 -a 1 -p 5043 127.0.0.1

# Gateway diagnostics (unit 247)
mbpoll -1 -t 4 -r 1 -c 5 -a 247 -p 5043 127.0.0.1
```

**Results**:

```
TotalFlow G5 holding registers:
[2]=30145  [3]=6553  [4]=3276  [5]=3276  [6]=6553  [7]=13670  [8]=6113  [9]=6078  [10]=0

TotalFlow G5 coils:
[2]=0  [3]=0  [4]=0

Pipeline gateway diagnostics:
[1]=0  [2]=0  [3]=0  [4]=0  [5]=1
```

Register map (holding): [2] methane_pct, [3] ethane_pct, [4] propane_pct, [5] co2_pct, [6] nitrogen_pct, [7] btu_content, [8] specific_gravity, [9] analysis_cycle_time, [10] analysis_status.

Coils: [2] analysis_in_progress, [3] gc_alarm_active, [4] moisture_alarm_active.

**Status**: PASS

## 5. Register Drift Verification

Process simulation models continuously update register values to simulate real sensor readings. Reading the same registers 10 seconds apart should show different values for measurement registers.

### Test Procedure

```bash
# Read station monitoring registers
mbpoll -1 -t 4 -r 2 -c 8 -a 1 -p 5042 127.0.0.1 | grep "^\["
sleep 10
mbpoll -1 -t 4 -r 2 -c 8 -a 1 -p 5042 127.0.0.1 | grep "^\["
```

### Results

```
Read 1:  [2]=12876  [3]=18148  [4]=16320  [5]=20222  [6]=0  [7]=0  [8]=32767  [9]=0
Read 2:  [2]=12524  [3]=17397  [4]=16548  [5]=20552  [6]=0  [7]=0  [8]=32767  [9]=0
```

**Analysis**:

| Register | Name | Read 1 | Read 2 | Changed? | Expected? |
|----------|------|--------|--------|----------|-----------|
| [2] | station_inlet_pressure | 12876 | 12524 | Yes (drift) | Yes -- sensor measurement |
| [3] | station_outlet_pressure | 18148 | 17397 | Yes (drift) | Yes -- sensor measurement |
| [4] | station_inlet_temperature | 16320 | 16548 | Yes (drift) | Yes -- sensor measurement |
| [5] | station_outlet_temperature | 20222 | 20552 | Yes (drift) | Yes -- sensor measurement |
| [6] | inlet_block_valve_position | 0 | 0 | No | Yes -- valve is stationary |
| [7] | outlet_block_valve_position | 0 | 0 | No | Yes -- valve is stationary |
| [8] | esd_valve_position | 32767 | 32767 | No | Yes -- ESD valve fully open |
| [9] | station_status_word | 0 | 0 | No | Yes -- no alarms |

**Status**: PASS -- Measurement registers drift. Static values (valves, status) remain constant.

## 6. Response Time Comparison

Different OT devices have different response characteristics. This test demonstrates that the simulator models these differences.

### Test Procedure

```bash
time mbpoll -1 -t 4 -r 1 -c 1 -a 1 -p <PORT> -q 127.0.0.1
```

### Results

| Device | Type | Port | Response Time | Notes |
|--------|------|------|--------------|-------|
| CompactLogix (water) | PLC, direct Ethernet | 5020 | 39ms | Fast: modern PLC, 10ms base delay |
| CompactLogix (pipeline) | PLC, direct Ethernet | 5040 | 36ms | Fast: same device type |
| ROC800 (metering) | RTU, direct Ethernet | 5041 | 73ms | Slower: 20ms base + 30ms jitter |
| ROC800 (monitoring) | RTU, direct Ethernet | 5042 | 80ms | Slower: same device, different variant |
| SLC-500 via gateway | Legacy PLC, serial | 5030 (unit 1) | 155ms | Slowest: serial conversion overhead |
| Modicon via gateway | Legacy PLC, serial | 5030 (unit 2) | 149ms | Slowest: serial conversion overhead |
| TotalFlow via gateway | Flow computer, serial | 5043 (unit 1) | 117ms | Moderate: pipeline gateway (faster than water) |

**Analysis**:

- **Direct Ethernet PLCs** (~35-40ms): Fastest. Modern CompactLogix responds quickly over Ethernet.
- **Direct Ethernet RTUs** (~70-80ms): Moderate. ROC800 is an embedded device (PowerPC, 65MHz), not a high-speed PLC. Base delay 20ms + response jitter up to 30ms models burst-latency during AGA calculation cycles.
- **Serial devices via gateway** (~115-155ms): Slowest. The gateway must convert Ethernet-to-serial and back. The serial bus transmission time and potential retries add latency.

**Teaching point**: In real OT environments, response time differences like these affect SCADA polling strategies. Faster devices can be polled more frequently. Slow serial paths may need longer timeouts to avoid false communication failure alarms.

**Status**: PASS -- Response times reflect device characteristics.

## 7. Coil Write Test

Coils are single-bit outputs that control discrete states (on/off). This test verifies write capability on a writable coil.

### Test: Enable/Disable Meter Run 1 on ROC800

```bash
# Read initial value (expect 0 = disabled)
mbpoll -1 -t 0 -r 2 -c 1 -a 1 -p 5041 127.0.0.1

# Write 1 (enable meter run)
mbpoll -1 -t 0 -r 2 -a 1 -p 5041 127.0.0.1 1

# Read back (expect 1)
mbpoll -1 -t 0 -r 2 -c 1 -a 1 -p 5041 127.0.0.1

# Write 0 (disable meter run)
mbpoll -1 -t 0 -r 2 -a 1 -p 5041 127.0.0.1 0

# Read back (expect 0)
mbpoll -1 -t 0 -r 2 -c 1 -a 1 -p 5041 127.0.0.1
```

### Results

```
Initial:          [2] = 0
After write 1:    Written 1 references.
Read back:        [2] = 1
After write 0:    Written 1 references.
Read back:        [2] = 0
```

**Status**: PASS -- Coil writes persist and read back correctly.

**Teaching point**: In a real pipeline, disabling a meter run removes it from AGA flow calculations. An attacker who disables all meter runs could show zero measured flow while gas continues flowing -- a custody-transfer fraud scenario.

## 8. Cross-Environment Isolation

Both environments run simultaneously but are completely isolated. Requests to one environment's ports never reach the other.

### Test

```bash
# Water PLC on port 5020 returns water-intake data (5 registers)
mbpoll -1 -t 4 -r 1 -c 5 -a 1 -p 5020 -q 127.0.0.1

# Pipeline PLC on port 5040 returns compressor-control data (different register layout)
mbpoll -1 -t 4 -r 1 -c 9 -a 1 -p 5040 -q 127.0.0.1
```

### Results

```
Port 5020 (water):    [1]=0  [2]=16383  [3]=16788  [4]=11327  [5]=14458
Port 5040 (pipeline): [1]=0  [2]=13267  [3]=13267  [4]=11955  [5]=10550  [6]=0  [7]=0  [8]=13106  [9]=0
```

The register counts (5 vs 9) and values are distinct. Each environment serves its own device data independently.

**Status**: PASS

## 9. Stopping the Environments

### Stop specific profile

```bash
docker compose --profile pipeline down
```

### Stop all profiles

```bash
docker compose --profile water --profile pipeline down
```

### Start only one environment

```bash
# Water/manufacturing only
docker compose --profile water up -d

# Pipeline only
docker compose --profile pipeline up -d
```

## 10. Common Errors and Troubleshooting

### "Illegal data address"

**Cause 1: Wrong addressing offset for one-based devices**

```bash
# WRONG -- sends wire address 0, but one-based devices start at wire address 1
mbpoll -1 -t 4 -r 1 -c 7 -a 1 -p 5041 127.0.0.1
# Error: Illegal data address

# CORRECT -- -r 2 sends wire address 1, which is device address 1
mbpoll -1 -t 4 -r 2 -c 7 -a 1 -p 5041 127.0.0.1
```

**Cause 2: Reading more registers than the device has**

```bash
# WRONG -- water-intake only has 5 holding registers, not 6
mbpoll -1 -t 4 -r 1 -c 6 -a 1 -p 5020 127.0.0.1
# Error: Illegal data address

# CORRECT -- read exactly 5 registers
mbpoll -1 -t 4 -r 1 -c 5 -a 1 -p 5020 127.0.0.1
```

### "Connection refused" on gateway port

The gateway routes by unit ID. If you don't specify a valid unit ID, the request may fail.

```bash
# WRONG -- unit ID 3 has no device mapped behind the water gateway
mbpoll -1 -t 4 -r 1 -c 1 -a 3 -p 5030 127.0.0.1

# CORRECT -- unit 1 = SLC-500, unit 2 = Modicon, unit 247 = gateway diagnostics
mbpoll -1 -t 4 -r 2 -c 7 -a 1 -p 5030 127.0.0.1
```

### "Connection refused" on any port

Check that containers are running and healthy:

```bash
docker compose ps
```

If a container shows "unhealthy", check its logs:

```bash
docker compose logs plant-water
docker compose logs plant-pipeline
```

## Appendix: mbpoll Quick Reference

```
mbpoll [options] host

Key flags:
  -1            Poll once and exit (default: poll continuously)
  -t 4          Read holding registers (16-bit unsigned)
  -t 0          Read/write coils (discrete outputs, 0 or 1)
  -r N          Start reference number (default 1; wire address = N-1)
  -c N          Number of values to read (1-125)
  -a N          Modbus slave/unit address (1-255)
  -p N          TCP port (default 502)
  -q            Quiet mode (minimal output)
  -0            Use PDU addressing (reference = wire address, not reference-1)

Write syntax:
  mbpoll -1 -t 0 -r 2 -a 1 -p 5041 127.0.0.1 1    # Write coil=1
  mbpoll -1 -t 4 -r 2 -a 1 -p 5041 127.0.0.1 500   # Write register=500
```

## Appendix: Docker Compose Profiles

The OT Simulator uses Docker Compose profiles to select which environments to run:

```bash
# Start water/manufacturing only
docker compose --profile water up -d

# Start pipeline only
docker compose --profile pipeline up -d

# Start both environments simultaneously
docker compose --profile water --profile pipeline up -d

# Rebuild and start
docker compose --profile water --profile pipeline up --build -d

# Stop everything
docker compose --profile water --profile pipeline down
```

With no `--profile` flag, `docker compose up` starts nothing (all services require a profile).
