# Expected Asset Inventory

**Facility**: Greenfield Water and Manufacturing
**Environment**: greenfield-water-mfg
**Reference Answer for Scenario 01**

This is the complete reference inventory. Use it to verify your discovered inventory is accurate
and complete. A completed scenario requires all 6 devices with correct register counts and
addresses.

---

## Section 1: IP-Addressable Devices

| Field | wt-plc-01 | wt-plc-02 | wt-plc-03 | mfg-gateway-01 |
|-------|-----------|-----------|-----------|----------------|
| Placement ID | wt-plc-01 | wt-plc-02 | wt-plc-03 | mfg-gateway-01 |
| Simulator Port | 5020 | 5021 | 5022 | 5030 |
| IP Address | 10.10.10.10 | 10.10.10.11 | 10.10.10.12 | 192.168.1.20 |
| Network | wt-level1 (10.10.10.0/24) | wt-level1 (10.10.10.0/24) | wt-level1 (10.10.10.0/24) | mfg-flat (192.168.1.0/24) |
| Vendor | Allen-Bradley | Allen-Bradley | Allen-Bradley | Moxa |
| Model | CompactLogix 5380 L33ER | CompactLogix 5380 L33ER | CompactLogix 5380 L33ER | NPort 5150 |
| Category | PLC | PLC | PLC | Gateway |
| Vintage (year) | 2024 | 2024 | 2024 | 2010 |
| Holding Registers | 5 | 7 | 5 | 9 |
| Register Addresses | 0-4 (zero-based) | 0-6 (zero-based) | 0-4 (zero-based) | 0-8 (zero-based) |
| Coils | 4 | 4 | 3 | 0 |
| Coil Addresses | 0-3 (zero-based) | 0-3 (zero-based) | 0-2 (zero-based) | none |
| FC01 Result | Valid data | Valid data | Valid data | Exception 02 |
| Response Time | ~5ms | ~5ms | ~5ms | ~15ms |
| Response Jitter | ±3ms | ±3ms | ±3ms | ±10ms |
| Role / Function | Water Intake | Water Treatment | Water Distribution | Serial-to-Ethernet Gateway |

### Register Details: wt-plc-01 (Water Intake, port 5020)

Holding registers (zero-based addressing, big-endian byte order):

| Address | Name | Unit | Scale Range | Writable | Description |
|---------|------|------|-------------|----------|-------------|
| 0 | intake_flow_rate | L/s | 0-100 | No | Raw water intake flow rate |
| 1 | intake_pump_speed | % | 0-100 | Yes | Intake pump speed setpoint |
| 2 | raw_water_ph | pH | 0-14 | No | Raw water pH at intake |
| 3 | raw_water_turbidity | NTU | 0-100 | No | Raw water turbidity (untreated) |
| 4 | intake_water_temp | degC | 0-40 | No | Raw water temperature |

Coils (zero-based addressing):

| Address | Name | Writable | Description |
|---------|------|----------|-------------|
| 0 | intake_pump_01_run | Yes | Intake pump 1 run command (0=stop, 1=run) |
| 1 | intake_pump_02_run | Yes | Intake pump 2 run command (0=stop, 1=run) |
| 2 | screen_wash_active | No | Travelling screen wash cycle in progress |
| 3 | low_well_level_alarm | No | Intake well level below minimum threshold |

### Register Details: wt-plc-02 (Water Treatment, port 5021)

Holding registers (zero-based addressing, big-endian byte order):

| Address | Name | Unit | Scale Range | Writable | Description |
|---------|------|------|-------------|----------|-------------|
| 0 | filter_inlet_pressure | kPa | 0-500 | No | Pressure upstream of filter media |
| 1 | filter_outlet_pressure | kPa | 0-500 | No | Pressure downstream of filter media |
| 2 | filter_differential_pressure | kPa | 0-50 | No | Differential pressure across filter (backwash trigger >15-25 kPa) |
| 3 | uv_intensity | mW/cm2 | 0-100 | No | UV sterilizer lamp intensity |
| 4 | chemical_feed_rate | mL/min | 0-500 | Yes | Sodium hypochlorite dosing rate |
| 5 | chlorine_residual | mg/L | 0-5 | No | Post-treatment chlorine residual |
| 6 | turbidity_post_filter | NTU | 0-5 | No | Post-filtration turbidity (target <1 NTU) |

Coils (zero-based addressing):

| Address | Name | Writable | Description |
|---------|------|----------|-------------|
| 0 | filter_backwash_command | Yes | Initiate filter backwash sequence |
| 1 | chemical_feed_pump_run | Yes | Sodium hypochlorite dosing pump run command |
| 2 | uv_system_active | No | UV sterilization system energized (status) |
| 3 | high_dp_alarm | No | Filter differential pressure exceeds threshold |

### Register Details: wt-plc-03 (Water Distribution, port 5022)

Holding registers (zero-based addressing, big-endian byte order):

| Address | Name | Unit | Scale Range | Writable | Description |
|---------|------|------|-------------|----------|-------------|
| 0 | clear_well_level | % | 0-100 | No | Clear well water level |
| 1 | distribution_flow_rate | L/s | 0-150 | No | Outbound distribution flow rate |
| 2 | distribution_pressure | kPa | 0-700 | No | Distribution header pressure |
| 3 | residual_chlorine | mg/L | 0-5 | No | Finished water chlorine residual |
| 4 | distribution_water_temp | degC | 0-40 | No | Finished water temperature leaving plant |

Coils (zero-based addressing):

| Address | Name | Writable | Description |
|---------|------|----------|-------------|
| 0 | distribution_pump_01_run | Yes | Distribution pump 1 run command |
| 1 | distribution_pump_02_run | Yes | Distribution pump 2 run command |
| 2 | booster_pump_run | Yes | Booster pump run command for cross-plant supply |

Note: This device has 3 coils, not 4. Coil counts are not uniform across devices.

### Register Details: mfg-gateway-01 (Moxa NPort Gateway, port 5030)

Holding registers (zero-based addressing, big-endian byte order):

Note: Real NPort 5150 devices have no Modbus registers. These simulated registers are an
educational abstraction representing gateway status.

| Address | Name | Unit | Scale Range | Writable | Description |
|---------|------|------|-------------|----------|-------------|
| 0 | serial_port_status | enum | 0-2 | No | 0=offline, 1=online, 2=error |
| 1 | serial_baud_rate | enum | 0-7 | No | 0=1200, 1=2400, 2=4800, 3=9600, 4=19200, 5=38400, 6=57600, 7=115200 |
| 2 | serial_data_format | enum | 0-3 | No | 0=8N1, 1=8E1, 2=8O1, 3=7E1 |
| 3 | serial_mode | enum | 0-3 | No | 0=RS-232, 1=RS-422, 2=RS-485-2wire, 3=RS-485-4wire |
| 4 | active_tcp_connections | count | 0-4 | No | Current active TCP client connections |
| 5 | serial_tx_count | msgs | 0-65535 | No | Messages forwarded from TCP to serial |
| 6 | serial_rx_count | msgs | 0-65535 | No | Messages received from serial port |
| 7 | serial_error_count | count | 0-65535 | No | Serial parity, framing, and overrun errors |
| 8 | uptime_hours | hours | 0-65535 | No | Device uptime since last power cycle |

Coils: None (FC01 returns exception 02 -- Illegal Data Address)

---

## Section 2: Serial Devices (No IP Address)

| Field | mfg-plc-01 | mfg-plc-02 |
|-------|------------|------------|
| Placement ID | mfg-plc-01 | mfg-plc-02 |
| Gateway IP | 192.168.1.20 | 192.168.1.20 |
| Gateway Port | 5030 | 5030 |
| Modbus Unit ID | 1 | 2 |
| Network | mfg-serial-bus (RS-485) | mfg-serial-bus (RS-485) |
| Vendor | Allen-Bradley | Schneider Electric |
| Model | SLC 500-05 | Modicon 984 |
| Category | PLC | PLC |
| Vintage (year) | 1992 | 1988 |
| Holding Registers | 7 | 7 |
| Register Addresses | 1-7 (one-based) | 1-7 (one-based) |
| Coils | 4 | 4 |
| Coil Addresses | 1-4 (one-based) | 1-4 (one-based) |
| Addressing Convention | One-based (ProSoft MVI46-MCM module) | One-based (Modicon native) |
| Byte Order | Big-endian | Little-endian (CDAB) |
| Response Time | ~65ms | ~95ms |
| Response Jitter | ±20ms | ±50ms |
| Role / Function | Line A Conveyor and Assembly | Cooling Water System |

### Register Details: mfg-plc-01 (SLC-500 Line A, unit ID 1)

Note: Modbus RTU capability is provided by a ProSoft MVI46-MCM communication module.
The SLC-500's native Channel 0 port speaks DH-485, not Modbus. RS-485 is an electrical standard;
it does not imply Modbus protocol.

Holding registers (one-based addressing, big-endian byte order):

| Address | Name | Unit | Scale Range | Writable | Description |
|---------|------|------|-------------|----------|-------------|
| 1 | conveyor_speed | ft/min | 0-200 | Yes | Conveyor belt speed setpoint |
| 2 | motor_current | A | 0-30 | No | Conveyor drive motor current draw |
| 3 | product_count | units | 0-10000 | No | Completed units today (resets at shift change) |
| 4 | reject_count | units | 0-500 | No | Rejected units today |
| 5 | line_temperature | degF | 0-200 | No | Assembly station ambient temperature |
| 6 | cycle_time | s | 0-120 | No | Current assembly station cycle time |
| 7 | status_word | bitmask | 0-65535 | No | bit0=running, bit1=faulted, bit2=jammed, bit3=e-stop, bit4=VFD-fault, bit5=overtemp |

Coils (one-based addressing):

| Address | Name | Writable | Description |
|---------|------|----------|-------------|
| 1 | conveyor_run | Yes | Conveyor run/stop command |
| 2 | conveyor_direction | Yes | 0=forward, 1=reverse |
| 3 | e_stop_active | No | Emergency stop circuit active (hardwired, read-only per NFPA 79) |
| 4 | jam_detected | No | Product jam sensor triggered |

Note: Uses imperial units (ft/min, degF) -- manufacturing floor legacy convention.

### Register Details: mfg-plc-02 (Modicon 984 Cooling, unit ID 2)

Note: The Modicon 984 is historically significant. Modicon (now Schneider Electric) created the
Modbus protocol in 1979. This unit dates from 1988 and uses the original Modbus conventions.

Holding registers (one-based addressing, little-endian CDAB byte order):

| Address | Name | Unit | Scale Range | Writable | Description |
|---------|------|------|-------------|----------|-------------|
| 1 | supply_temp | degF | 40-120 | No | Coolant supply temperature (to equipment) |
| 2 | return_temp | degF | 40-120 | No | Coolant return temperature (from equipment) |
| 3 | flow_rate | GPM | 0-500 | No | Cooling loop flow rate |
| 4 | pump_pressure | PSI | 0-80 | No | Cooling loop discharge pressure |
| 5 | tank_level | % | 0-100 | No | Coolant reservoir level |
| 6 | setpoint_temp | degF | 40-80 | Yes | Cooling loop temperature setpoint (ATTACK SURFACE) |
| 7 | pump_runtime_hours | hours | 0-65535 | No | Lead pump cumulative runtime |

Coils (one-based addressing):

| Address | Name | Writable | Description |
|---------|------|----------|-------------|
| 1 | pump_1_run | Yes | Primary cooling pump run command (lead pump) |
| 2 | pump_2_run | Yes | Backup cooling pump run command (lag pump) |
| 3 | low_coolant_alarm | No | Coolant reservoir below minimum level |
| 4 | high_temp_alarm | No | Coolant return temperature exceeds safe threshold |

Note: Uses imperial units (degF, PSI, GPM) -- manufacturing floor convention.
Note: CDAB byte order means the two bytes of each word are swapped from Modbus standard.
If you apply big-endian decoding from the SLC-500 to this device, values will appear incorrect.

---

## Section 3: Network Topology

```
Water Treatment Plant (Purdue Model)
+----------------------------------------+
| wt-level1 (10.10.10.0/24)              |
|  10.10.10.10  wt-plc-01  port 5020     |
|  10.10.10.11  wt-plc-02  port 5021     |
|  10.10.10.12  wt-plc-03  port 5022     |
|               (also on cross-plant      |
|                172.16.0.2)              |
+----------------------------------------+
             |
        cross-plant (172.16.0.0/30)
        wt-plc-03: 172.16.0.2
        mfg-gateway-01: 172.16.0.3
             |
+----------------------------------------+
| mfg-flat (192.168.1.0/24)              |
|  192.168.1.20  mfg-gateway-01  port 5030|
|               (also on cross-plant      |
|                172.16.0.3)              |
+--------------------+-------------------+
                     |
              mfg-serial-bus (RS-485)
              |               |
        unit ID 1        unit ID 2
        mfg-plc-01       mfg-plc-02
        SLC-500-05       Modicon 984
        (no IP)          (no IP)
```

---

## Section 4: Discovery Gaps

| Question | Finding | Confidence |
|----------|---------|------------|
| Additional serial devices at unit IDs > 2? | Unit IDs 3 and 4 both return exception 0x0B. No additional devices found. | High |
| Devices on other subnets not scanned? | Cross-plant link 172.16.0.0/30 exists but its devices (wt-plc-03 and mfg-gateway-01) were already discovered via their primary interfaces. | High |
| Wireless or cellular-connected devices? | None observed in this environment version. | Medium (not scanned) |

---

## Scaling Notes

Register values are 16-bit unsigned integers (0-32767) mapped to engineering ranges. To convert
a raw register value to engineering units:

```
engineering_value = scale_min + (raw_value / 32767) * (scale_max - scale_min)
```

Example: Port 5020, address 0 (intake_flow_rate, range 0-100 L/s):
- Raw value 16384 -> engineering value = 0 + (16384 / 32767) * 100 = approximately 50 L/s

Values will drift over time due to the SOW-003.0 process simulation. Do not expect exact values
to match between polls. Look for value ranges consistent with normal operations.
