# ADR-006: Airgap and Accidental Bridge Modeling

**Status**: Proposed (Revised 2026-03-01)
**Date**: 2026-02-28
**Decision Makers**: Rusty Brownlee
**See also**: ADR-009 (Design Layer and Composable Environments)

---

## Context

Many OT environments claim to be "airgapped" -- physically isolated from the internet and corporate IT networks. In practice, true airgaps are rare. Accidental or intentional bridges exist in most environments:

| Bridge Type | How It Gets There | How Common | Detection Difficulty |
|------------|-------------------|------------|---------------------|
| Cellular modem | Vendor installs for "temporary" remote support | Very common | Hard -- often undocumented, hidden in cabinets |
| Vendor laptop | Technician brings laptop previously connected to internet | Universal | Impossible until connected |
| USB drive | Firmware updates, data export, configuration changes | Universal | Varies -- some facilities prohibit, most don't enforce |
| Dual-homed workstation | Engineering workstation on both IT and OT networks | Common | Medium -- visible in network scans |
| Wireless access point | Added for "convenience" by plant staff | Common | Medium -- RF scan detectable |
| Unauthorized VPN | IT installs remote access without OT team knowledge | Occasional | Hard -- encrypted traffic looks normal |

These bridges are the primary attack vector for airgapped OT environments. Stuxnet entered Iran's Natanz uranium enrichment facility via a USB drive. The 2021 Oldsmar water treatment attack used a remote access tool (TeamViewer) installed on an HMI.

### Impact of Airgap on Operations

Beyond security, the airgap creates operational constraints that IT engineers rarely consider:

| IT Assumption | Airgapped Reality |
|---------------|-------------------|
| NTP for time sync | No NTP. Clocks drift. A PLC clock can be minutes or hours off after months of operation. |
| Centralized logging (Splunk, ELK) | No log aggregation. Logs exist on individual devices (if at all). |
| Patch management (WSUS, SCCM) | Updates arrive on USB drives, months after release, if ever. |
| DNS resolution | No DNS. Everything is static IP. Engineers memorize addresses or consult paper binders. |
| DHCP | No DHCP. Static assignments in a spreadsheet (maybe). IP conflicts happen. |
| Cloud-based monitoring | No cloud. Everything is local. Management interfaces are on the OT network. |
| Automated asset inventory | No discovery tools. The asset inventory is tribal knowledge. |

---

## Decision

### D1: Manufacturing Floor is Nominally Airgapped with Known Bridges

**Decision**: The manufacturing floor is configured as an airgapped network by default, with two pre-configured accidental bridges that create realistic attack surface.

**Bridge 1: Cellular modem (Cradlepoint IBR600)**

Per ADR-009, the cellular modem is a device atom (`cradlepoint-ibr600`) in the design layer device library, placed on the manufacturing flat network in the environment template. Its security profile (default credentials, NAT, port forwarding) is defined in the device atom schema v0.2.

- IP: 192.168.1.99
- Connected to the flat manufacturing network
- Provides internet access via cellular (4G/LTE)
- Installed by a vendor in 2019 for remote PLC programming
- Web management interface on port 443 with default credentials
- NAT enabled -- manufacturing devices can reach the internet but are not directly addressable from outside (unless port forwarding is configured, which it often is)
- The manufacturing team does not know this device exists

**Bridge 2: Vendor laptop scenario (event-based)**
- Not a permanent device
- Modeled as a scenario event: "A vendor arrives to update PLC #2 firmware. They connect their laptop to the manufacturing network. The laptop was on hotel WiFi last night."
- The laptop may carry malware, have cached credentials for other sites, or run unauthorized remote access tools
- This bridge appears and disappears based on scenario configuration

### D2: Clock Drift Simulation

**Decision**: Devices in the airgapped manufacturing environment have independently drifting clocks. No NTP is available.

**Implementation**:
- Each virtual PLC and HMI in the manufacturing zone has a simulated clock offset
- Offset starts at zero and drifts at a configurable rate (default: 0.5-2.0 seconds per hour, randomized per device)
- After 24 hours of simulation, device clocks can be 12-48 seconds apart
- After weeks of simulation time, clocks can diverge by minutes

**Teaching value**: When an IT engineer tries to correlate events across devices (e.g., "the Modbus write happened at the same time as the HMI alarm"), they discover the timestamps don't match. This is the reality of log correlation in environments without NTP -- and it's a critical lesson for incident response.

**Contrast**: The water treatment plant has NTP. All devices are synchronized. Timestamps correlate cleanly. This side-by-side comparison shows the value of time synchronization in OT environments.

### D3: No Centralized Logging on Manufacturing Side

**Decision**: The manufacturing floor has no centralized log collection. Individual devices may have local logs, but there is no mechanism to aggregate them.

**Available logs per device**:

| Device | Log Type | Location | Format | Persistence |
|--------|----------|----------|--------|-------------|
| Windows XP HMI | Windows Event Log | Local only | EVT format | Until disk full |
| Moxa NPort | Web access log | Internal, viewable via HTTP | Plaintext | Last 100 entries |
| SLC-500 | Fault log | Internal PLC memory | Proprietary (viewable via RSLogix) | Last 10 faults |
| Modicon 984 | None | -- | -- | -- |
| Cradlepoint modem | Connection log | Internal, viewable via HTTPS | Plaintext | Last 50 entries |

**Contrast**: The water treatment plant has managed switches generating syslog, a historian recording all process values, and an engineering workstation with centralized event logging.

### D4: USB/Sneakernet Modeling

**Decision**: The simulator models USB-based data transfer as the primary update mechanism for the airgapped environment.

**Scenarios involving USB**:
- Firmware update: A USB drive contains a PLC firmware update. The update requires connecting a programming laptop to the PLC via serial cable, loading the firmware from USB, and programming the PLC.
- Data export: An engineer exports historian data (from the water treatment side) to USB for analysis on a corporate laptop.
- Configuration backup: A PLC configuration is backed up to USB for disaster recovery.

**Risk modeling**: Each USB interaction is a potential malware vector. Scenarios can configure whether the USB drive is "clean" or "compromised," teaching engineers to think about supply chain risk in the data transfer process.

---

## Consequences

### Positive
- Engineers experience the operational reality of airgapped environments firsthand
- Clock drift creates tangible incident response challenges
- The cellular modem teaches that airgaps are often fictional
- USB/sneakernet scenarios connect to real-world attack vectors (Stuxnet)

### Negative
- Clock drift simulation requires tracking time offsets per device
- USB modeling is primarily scenario-narrative, not protocol-level simulation
- Vendor laptop scenarios are event-based, not persistent infrastructure

### Risks
- **Risk**: Clock drift may seem like a minor detail but significantly complicates the monitoring module
- **Mitigation**: This IS the point. If monitoring were easy, the training has no value.
- **Risk**: Cellular modem simulation could be used to demonstrate actual internet-facing attack techniques
- **Mitigation**: The modem connects to a simulated "internet" (another Docker network), not the real internet. No actual external connectivity.

---

## References
- ADR-009: Design Layer and Composable Environments
- NIST SP 800-82 Rev. 3: Guide to Operational Technology (OT) Security
- CISA Alert: Oldsmar Water Treatment Facility (AA21-042A)
- Stuxnet: Anatomy of a Computer Virus (documentary reference)
- ICS-CERT: Recommended Practice for Securing Remote Access
