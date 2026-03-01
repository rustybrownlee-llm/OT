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

## Completion Threshold

A scenario is considered complete when all 24 criteria are satisfied. Partial completion is valid
for study purposes -- if you satisfy SC-01 through SC-06, you have found all devices. The remaining
items deepen understanding beyond basic enumeration.

Compare your completed inventory against `reference/expected-asset-inventory.md` to verify
register counts and addresses are correct.
