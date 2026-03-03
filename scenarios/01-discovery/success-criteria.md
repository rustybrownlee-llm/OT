# Scenario 01: Success Criteria

This checklist defines completion for Scenario 01. Evaluate each item against your completed asset
inventory and your notes from the discovery session. All items must be satisfied.

---

## Device Discovery (6 items -- one per device)

- [ ] **SC-01**: PLC at port 5020 discovered. IP 10.10.10.10, 5 holding registers (addresses 0-4),
  4 coils (addresses 0-3), response time approximately 5ms.

- [ ] **SC-02**: PLC at port 5021 discovered. IP 10.10.10.11, 7 holding registers (addresses 0-6),
  4 coils (addresses 0-3), response time approximately 5ms.

- [ ] **SC-03**: PLC at port 5022 discovered. IP 10.10.10.12, 5 holding registers (addresses 0-4),
  3 coils (addresses 0-2), response time approximately 5ms. Note: 3 coils, not 4.

- [ ] **SC-04**: Moxa NPort gateway at port 5030 discovered. IP 192.168.1.20, 9 holding registers
  (addresses 0-8), 0 coils (FC01 returns exception 02), response time approximately 15ms.

- [ ] **SC-05**: SLC-500 at gateway unit ID 1 discovered. No independent IP address. Reached via
  192.168.1.20 port 5030, unit ID 1. 7 holding registers (addresses 1-7), 4 coils (addresses 1-4),
  response time approximately 65ms (±20ms jitter). One-based addressing confirmed.

- [ ] **SC-06**: Modicon 984 at gateway unit ID 2 discovered. No independent IP address. Reached via
  192.168.1.20 port 5030, unit ID 2. 7 holding registers (addresses 1-7), 4 coils (addresses 1-4),
  response time approximately 95ms (±50ms jitter). One-based addressing confirmed.

---

## Register Content (per device)

- [ ] **SC-07**: Port 5020 registers identified as water intake process data: intake flow rate,
  pump speed setpoint, raw water pH, turbidity, and water temperature.

- [ ] **SC-08**: Port 5021 registers identified as water treatment data: filter pressures (inlet,
  outlet, differential), UV intensity, chemical feed rate, chlorine residual, post-filter turbidity.

- [ ] **SC-09**: Port 5022 registers identified as distribution data: clear well level, distribution
  flow rate, distribution pressure, residual chlorine, and water temperature.

- [ ] **SC-10**: Port 5030 registers identified as gateway status data: serial port status, baud rate
  enum, data format enum, serial mode enum, active TCP connections, TX/RX message counts,
  error count, and uptime hours.

- [ ] **SC-11**: Unit ID 1 registers identified as manufacturing line data: conveyor speed, motor
  current, product count, reject count, line temperature, cycle time, and status word bitmask.

- [ ] **SC-12**: Unit ID 2 registers identified as cooling system data: supply temperature, return
  temperature, flow rate, pump pressure, tank level, temperature setpoint, and pump runtime hours.

---

## Technical Observations

- [ ] **SC-13**: Confirmed that polling address 0 on the SLC-500 (unit ID 1) or Modicon 984
  (unit ID 2) returns Modbus exception 02 (Illegal Data Address). First valid address is 1.
  One-based addressing is documented in the asset inventory.

- [ ] **SC-14**: Confirmed that FC01 (read coils) on port 5030 returns Modbus exception 02.
  The gateway has no coils. This is documented in the asset inventory.

- [ ] **SC-15**: Noted the response time difference between Ethernet PLCs (~5ms) and serial PLCs
  via gateway (~65ms SLC-500, ~95ms Modicon 984). Both observations recorded.

- [ ] **SC-16**: Noted the jitter difference: SLC-500 ±20ms, Modicon 984 ±50ms. High jitter on the
  Modicon is documented as normal for a legacy 1988 processor, not a network fault.

- [ ] **SC-17**: Confirmed that unit ID 3 and unit ID 4 on port 5030 return exception 0x0B
  (Gateway Target Device Failed to Respond). Device boundary established.

---

## Network Topology

- [ ] **SC-18**: Asset inventory correctly places the three CompactLogix PLCs on the water treatment
  Level 1 network (10.10.10.0/24).

- [ ] **SC-19**: Asset inventory correctly places the Moxa NPort gateway on the manufacturing flat
  network (192.168.1.0/24) at IP 192.168.1.20.

- [ ] **SC-20**: Asset inventory notes the cross-plant link (172.16.0.0/30) connecting the water
  treatment Distribution PLC (172.16.0.2) to the Moxa gateway (172.16.0.3).

- [ ] **SC-21**: Asset inventory uses the correct column structure for serial devices: no IP address
  field, but "Gateway IP + Unit ID" to identify the access path.

---

## Conceptual Understanding (self-assessment)

- [ ] **SC-22**: Can explain in one sentence why the SLC-500 and Modicon 984 do not appear in an
  nmap scan.

- [ ] **SC-23**: Can explain why a device with no Modbus coils is probably not a PLC controlling
  physical equipment.

- [ ] **SC-24**: Can explain why Modbus TCP uses port 502 in production and what the 5020+ ports in
  this simulator represent.

---

---

## Dashboard-Assisted Discovery (Phase E)

- [ ] **SC-25**: Monitoring dashboard opened at `http://localhost:8090`. Overview page shows 6
  devices online for the greenfield-water-mfg environment.

- [ ] **SC-26**: Assets page at `http://localhost:8090/assets` shows all 6 devices from your
  manual inventory: wt-plc-01, wt-plc-02, wt-plc-03, mfg-gateway-01, mfg-plc-01 (unit ID 1 via
  gateway), mfg-plc-02 (unit ID 2 via gateway). All are listed as Online. Dashboard discovery
  matches manual inventory with no discrepancies.

- [ ] **SC-27**: Design Library cross-link followed from a device detail page to the device atom
  YAML. The `compactlogix-l33er` atom page shows vendor, model, connectivity, register
  capabilities, and all register map variants. The page is labeled "Reference" to distinguish it
  from live monitoring data.

- [ ] **SC-28**: Can explain in one sentence the difference between "Observed" data on the asset
  detail page (live register values from network polling) and "Reference" data on the design
  library page (device specification from the design layer). Can state that a real OT security
  tool would have Observed data only.

- [ ] **SC-29**: Alerts page shows baseline status "Learning" for at least one device immediately
  after monitor startup. Baseline learning is in progress and anomaly detection is not yet active.

- [ ] **SC-30**: After the baseline learning period, baseline status transitions to "Established"
  for at least one device. If the optional write exercise was performed, an anomaly alert appeared
  on the Alerts page within one polling cycle of the coil write, and the alert was cleared after
  restoring the original value (or remains visible as a historical record).

---

---

## Hybrid Environment Discovery (Phase F)

- [ ] **SC-31**: Wastewater environment started and reachable. `nmap -sV -p 5062-5064 localhost`
  returns exactly 3 open ports (5062, 5063, 5064). Environment confirmed running before
  proceeding.

- [ ] **SC-32**: nmap scan finds 3 of the 5 wastewater devices. Discrepancy between scan count
  (3) and total device count (5) is documented. Can explain that the 2 missing devices are
  SLC-500 PLCs on a DH-485 serial bus behind the Moxa gateway at port 5063.

- [ ] **SC-33**: Cradlepoint IBR600 cellular gateway identified as an unexpected device at port
  5064. Device is not present in any facility documentation provided at the start of the
  engagement. Its presence on the flat OT network is flagged as an anomalous finding requiring
  further assessment.

- [ ] **SC-34**: Cradlepoint management registers read successfully at port 5064. Retrieved at
  least 7 register values including WAN connection status and signal strength. Can explain
  that these Modbus TCP registers are an educational simulator abstraction (TD-038) and that the
  real Cradlepoint IBR600 uses HTTPS (port 443) and SNMP for management, not Modbus TCP.

- [ ] **SC-35**: Both SLC-500 PLCs discovered via gateway port 5063: unit ID 1 (influent) and
  unit ID 2 (effluent). Confirmed that polling starts at address 1 (one-based) for both. Polling
  address 0 on either SLC-500 returns Modbus exception 02 (Illegal Data Address). Unit ID 3
  returns exception 0x0B (Gateway Target Device Failed to Respond), confirming the device
  boundary at unit ID 2.

- [ ] **SC-36**: Addressing contrast explicitly documented: CompactLogix at port 5062 uses
  zero-based addressing (first register at address 0); SLC-500s at unit IDs 1 and 2 via port
  5063 use one-based addressing (first valid register at address 1). Both device families are
  on the same network but require different polling start addresses.

- [ ] **SC-37**: Monitoring dashboard asset inventory opened and cross-referenced. Dashboard
  shows all 5 wastewater devices as Online. Manual discovery (nmap + mbpoll) found 3 IP
  endpoints; the monitor found the same 3 plus both serial SLC-500s (unit IDs 1 and 2) by
  scanning gateway unit IDs. The gap between nmap count (3) and dashboard count (5) is
  explained.

- [ ] **SC-38**: Topology view opened at `http://localhost:8090/topology/brownfield-wastewater`.
  Hybrid architecture identified: one enforced Level 3 boundary (solid line, managed switch,
  2018), absent Level 2, flat Level 1 segment containing all 5 operational devices. Can explain
  why the Level 3 boundary provides minimal operational security improvement when no operational
  device sits above it.

- [ ] **SC-39**: Era markers examined on the topology view. Five distinct installation years
  noted (1997, 1997, 2008, 2013, 2022). Can explain the threat model collision: the 1997
  SLC-500 was designed before widespread internet connectivity; the 2022 Cradlepoint assumes
  LTE connectivity and internet-facing access. Placing them on the same flat network means the
  SLC-500 is exposed to internet-connected adversaries its design did not anticipate.

- [ ] **SC-40**: All three environment topologies compared at:
  `http://localhost:8090/topology/greenfield-water-mfg` (segmented stack, single era),
  `http://localhost:8090/topology/brownfield-pipeline-station` (single flat plane, single era),
  and `http://localhost:8090/topology/brownfield-wastewater` (partial stack, 25-year era span).
  Can describe the visual shape of each and explain what architectural characteristic each shape
  represents.

- [ ] **SC-41**: Facility network map updated to include the wastewater environment. Map correctly
  documents: (a) all 5 devices with addressing conventions noted, (b) the two-layer serial
  backbone (DH-485 between SLC-500 chassis and ProSoft MVI46-MCM modules; ProSoft RS-485 to
  Moxa NPort to Modbus TCP), (c) the Cradlepoint's cellular WAN link as the only internet-
  connected path in any of the three environments, and (d) monitoring blind spots (no SPAN on
  flat Level 1 segment, no visibility into cellular traffic).

---

## Completion Threshold

A scenario is considered complete when all applicable criteria are satisfied. Partial completion
is valid for study purposes:

- SC-01 through SC-06: All devices found (core discovery)
- SC-07 through SC-17: Register contents and technical observations documented
- SC-18 through SC-21: Network topology correctly mapped
- SC-22 through SC-24: Conceptual understanding of key OT discovery concepts
- SC-25 through SC-30: Dashboard-assisted discovery completed and monitoring concepts understood
- SC-31 through SC-41: Hybrid environment discovery and architecture comparison completed
- SC-42 through SC-53: Process view context completed (Phase G -- optional extension)

Phases A-D can be completed without the monitoring module running. Phase E requires the monitor
to be running alongside the plant simulation. Phase F requires the wastewater environment
profile and the monitor profile both running. Phase G additionally requires the pipeline-monitoring
environment profile. Trainees who stop at Phase F still have a valid completion.

Compare your completed inventory against `reference/expected-asset-inventory.md` to verify
register counts and addresses are correct.

---

---

## Process View Context (Phase G)

These criteria apply only if Phase G (Process View Context) was completed. All three environment
process views must have been observed.

- [ ] **SC-42**: Process view opened at `http://localhost:8090/process`. Greenfield-water-mfg
  environment loads as the default. Three stages visible: Intake, Treatment, and Distribution.
  Each stage displays its controller PLC label (wt-plc-01, wt-plc-02, wt-plc-03) and the
  instruments associated with that stage. Values are updating approximately every 2 seconds.

- [ ] **SC-43**: Register-to-tag mapping completed for port 5020 (wt-plc-01). Can state from
  memory: HR[0] = FT-101 (Intake Flow Rate, L/s), HR[1] = SC-101 (Intake Pump Speed, writable
  setpoint, %), HR[2] = AT-101 (Raw Water pH), HR[3] = AT-102 (Raw Water Turbidity, NTU),
  HR[4] = TT-101 (Intake Water Temperature, degC).

- [ ] **SC-44**: Register-to-tag mapping completed for port 5021 (wt-plc-02). Can state: HR[4]
  is FIC-202 (Chemical Feed Rate, writable dosing setpoint, mL/min) and HR[6] is AT-201 (Turbidity
  Post-Filter, read-only measurement, NTU). Can explain the distinction: FIC-202 is writable
  (Flow Indicating Controller -- the dosing rate setpoint); AT-201 is read-only (Analyzer
  Transmitter -- the post-filter turbidity measurement downstream).

- [ ] **SC-45**: Can explain in one sentence the ISA-5.1 first-letter and suffix conventions.
  Correct answer includes: first letter encodes the measured variable (F=flow, A=analysis,
  T=temperature, P=pressure, L=level, Z=position); suffix encodes the function (T=transmitter
  means read-only measurement, IC=indicating controller means writable setpoint, S=switch means
  discrete status). Can give one example of each from the greenfield process view.

- [ ] **SC-46**: Brownfield-wastewater process view opened. Era mixing correctly identified: two
  1997 SLC-500 PLCs (ww-plc-01 and ww-plc-02) bracket a 2013 CompactLogix (ww-plc-03) in the
  aeration stage. Can explain that the 2013 modernization replaced only the aeration stage,
  leaving the original influent and effluent PLCs in place. Can state that this era mixing means
  addressing conventions differ by stage: SLC-500 stages use one-based addressing (ProSoft
  MVI46-MCM convention); the CompactLogix stage uses zero-based addressing.

- [ ] **SC-47**: Cradlepoint WAN callout located on the brownfield-wastewater process view near
  the aeration stage. Can explain that the Cradlepoint was installed in 2022 to read the blower
  run hours register (HR[11] on ww-plc-03) for predictive maintenance scheduling. Can state that
  because ww-plc-03 is on a flat network with no segmentation, the Cradlepoint's WAN link makes
  every register on every device on 192.168.10.0/24 reachable from the internet.

- [ ] **SC-48**: Pipeline-monitoring process view opened. Three stages identified: Gas Compression,
  Custody Transfer Metering, and Gas Quality Analysis. Can identify the controller for each stage:
  Gas Compression = ps-plc-01 (CompactLogix, WAN-reachable), Metering = ps-rtu-01 (ROC800,
  station-LAN-only), Gas Quality Analysis = ps-fc-01 (TotalFlow G5, serial via ps-gw-01).

- [ ] **SC-49**: ZT versus ZS distinction demonstrated using pipeline environment examples. Can
  state: ZT-101 (Inlet Block Valve Position) is an analog position transmitter (HR[5] on
  ps-rtu-02, 0-100%) that provides a continuous reading of valve travel; ZS-101 (ESD Active
  Status) is a discrete position switch (Coil[4] on ps-rtu-02, boolean) that indicates whether
  the Emergency Shutdown sequence is active. Can state that ZS-101 cannot be reset remotely
  via Modbus because it is a hardwired safety interlock per DOT 49 CFR 192.

- [ ] **SC-50**: FQ-250 (Station Total Volume Today) zero-reading behavior explained. Can state:
  a zero reading on FQ-250 at the contract hour rollover (often 9:00 AM per NAESB standards,
  not midnight) is normal behavior -- the totalizer resets to zero at each rollover. A zero reading
  does not indicate a meter failure, communication problem, or attack. Can distinguish between a
  rollover reset and a zero caused by disabling all meter runs via HS-201 through HS-204.

- [ ] **SC-51**: AGA-3 custody transfer metering explained in plain language. Can state: the
  AGA-3 orifice calculation uses three inputs -- differential pressure (PDT-201, dominant input
  because flow is proportional to sqrt(DP)), static pressure (PT-201), and flowing temperature
  (TT-201). Can state that PDT-201 is on ps-rtu-01 (ROC800, station-LAN-only at 10.20.1.20)
  and is NOT WAN-reachable. Reaching it from the WAN requires pivoting through the dual-homed
  ps-plc-01.

- [ ] **SC-52**: Gas chromatograph access chain documented. Can state the full chain: WAN access
  reaches ps-plc-01 only (dual-homed, WAN IP 10.20.0.2). The chromatograph (ps-fc-01, serial
  device) is accessible only via ps-gw-01 (Moxa gateway, 10.20.1.30:5043) on the station LAN.
  The full attack chain for chromatograph manipulation is: WAN -> ps-plc-01 (pivot) -> station
  LAN -> ps-gw-01 -> ps-fc-01.

- [ ] **SC-53**: Can articulate the core educational thesis demonstrated by comparing all three
  environments in the process view. Correct answer includes: the same Modbus TCP protocol --
  with the same absence of authentication and the same unauthenticated write capability --
  underlies water treatment (public health impact), wastewater treatment (environmental permit
  compliance impact), and natural gas pipeline operations (financial and physical safety impact).
  The process view transforms abstract register addresses into physical consequence descriptions
  that make vulnerability ratings meaningful.
