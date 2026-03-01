# Asset Inventory Template

**Facility**: Greenfield Water and Manufacturing
**Date**: _______________
**Assessed by**: _______________
**Environment**: greenfield-water-mfg

Fill in this template as you work through Scenario 01. Compare your completed inventory against
`expected-asset-inventory.md` to verify completeness.

---

## Section 1: IP-Addressable Devices

These devices have their own Ethernet interface and appear in network scans.

| Field | Device 1 | Device 2 | Device 3 | Device 4 |
|-------|----------|----------|----------|----------|
| Placement ID | | | | |
| Simulator Port | | | | |
| IP Address | | | | |
| Network | | | | |
| Vendor | | | | |
| Model | | | | |
| Category | | | | |
| Vintage (year) | | | | |
| Holding Registers | | | | |
| Register Addresses | | | | |
| Coils | | | | |
| Coil Addresses | | | | |
| FC01 Result | | | | |
| Response Time | | | | |
| Response Jitter | | | | |
| Role / Function | | | | |

---

## Section 2: Serial Devices (No IP Address)

These devices are not IP-addressable. They are reached via a gateway IP and a Modbus unit ID.
They will not appear in any network scan.

| Field | Serial Device 1 | Serial Device 2 |
|-------|----------------|----------------|
| Placement ID | | |
| Gateway IP | | |
| Gateway Port | | |
| Modbus Unit ID | | |
| Network | | |
| Vendor | | |
| Model | | |
| Category | | |
| Vintage (year) | | |
| Holding Registers | | |
| Register Addresses | | |
| Coils | | |
| Coil Addresses | | |
| Addressing Convention | | |
| Byte Order | | |
| Response Time | | |
| Response Jitter | | |
| Role / Function | | |

---

## Section 3: Network Topology

Describe the network architecture in your own words. Include:

- Subnet assignments for each network segment
- Which devices belong to each network
- Any cross-network connections observed

```
[Sketch or describe your topology here]




```

---

## Section 4: Discovery Gaps

Document anything that could not be confirmed:

| Question | Finding | Confidence |
|----------|---------|------------|
| Are there additional serial devices at unit IDs > 2? | | |
| Are there devices on other subnets not scanned? | | |
| Are there wireless or cellular-connected devices? | | |

---

## Section 5: Notes

Record any observations that do not fit the tables above:

```
[Notes]
```
