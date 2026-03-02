# Brownfield Wastewater Treatment Plant

## Facility Overview

Municipal wastewater treatment plant serving a population equivalent of 15,000-30,000 residents.
Design capacity: 2-7 million gallons per day (MGD). Constructed in 1997. Active as of 2022.

The facility treats raw sewage through bar screening, grit removal, primary clarification,
biological aeration, secondary clarification, UV disinfection, and final effluent discharge.
Discharge is regulated under an NPDES permit.

## Operational History

### 1997 -- Original Construction

The plant was built as a straightforward two-PLC, serial-networked facility. Two Allen-Bradley
SLC-500/05 PLCs control the influent screening and primary treatment (ww-plc-01) and effluent
secondary treatment and discharge (ww-plc-02). Both communicate via DH-485 serial bus -- Allen-
Bradley's proprietary token-passing protocol, not Modbus. A ProSoft MVI46-MCM communication
module in each SLC-500 chassis provides Modbus RTU capability. An unmanaged 24-port commodity
switch (Netgear or equivalent) forms the flat Ethernet backbone at 192.168.10.0/24. No VLANs,
no segmentation.

Technology added: SLC-500 (x2), DH-485 serial bus, flat Ethernet, unmanaged switch.

### 2008 -- Ethernet Bridge

City hall requested remote monitoring capability. A Moxa NPort 5150 serial-to-Ethernet gateway
was installed to bridge the SLC-500 ProSoft RS-485 ports to Ethernet, enabling Modbus TCP access
from the city operations center. No network redesign was performed -- the Moxa was simply plugged
into the existing flat switch. The DH-485 backbone remains, carrying its original traffic.

Technology added: Moxa NPort 5150 gateway.

### 2013 -- Partial Modernization

A new aeration blower system was installed to upgrade the biological treatment stage. The original
SLC-500 PLCs lacked the I/O density and Ethernet connectivity needed for variable-frequency drive
(VFD) communication. An Allen-Bradley CompactLogix 5380 L33ER was installed for the aeration
system. The CompactLogix is Ethernet-native and was connected directly to the existing flat switch.
No segmentation was added to separate the modern PLC from the legacy equipment.

Technology added: CompactLogix L33ER PLC.

### 2018 -- Audit Response

A state environmental compliance audit flagged the flat network topology as a cybersecurity risk.
The facility responded by installing a managed Cisco Catalyst 2960 switch and creating VLAN 30
(10.30.30.0/24) for a SCADA server. The original SLC-500s, the Moxa gateway, and the CompactLogix
remain on the flat Level 1 segment -- the audit response addressed only the SCADA server placement.
The segmentation project stalled at approximately 40% completion. No one completed Level 2.

Technology added: Managed switch, VLAN 30 (Level 3 only).

### 2022 -- Vendor Remote Access

The blower vendor (contracted for the 2013 CompactLogix system) required remote access to read
the CompactLogix blower_run_hours register for predictive maintenance scheduling. A Cradlepoint
IBR600 cellular modem was installed by a vendor technician. The technician connected it to the
flat switch because no one communicated the network architecture. Default credentials (admin/admin)
were never changed. NAT is enabled. The "temporary" device has remained for four years.

Technology added: Cradlepoint IBR600 cellular modem.

## Network Architecture

```
Level 3  +----------------------------------+
(2018)   |  [SCADA Server -- not modeled]   |
         |  VLAN 30, 10.30.30.0/24          |
         |  Managed switch (2018)            |
         +----------------+-----------------+
                          | enforced boundary (managed switch + VLAN)
- - - - - - - - - - - - -|- - - - - - - - - - - - -  Level 2 boundary: ABSENT
                          |                            (no Level 2 infrastructure)
                          |
Level 1  +----------------+------------------------------------------+
(1997+)  | FLAT SEGMENT: 192.168.10.0/24, unmanaged switch (1997)   |
         |                                                            |
         | [CompactLogix]  [Moxa NPort]====serial====+               |
         |  ww-plc-03       ww-gw-01     (1997)      |               |
         |  (2013)          (2008)                   |               |
         |  Ethernet-native             [SLC-500]  [SLC-500]         |
         |                               ww-plc-01  ww-plc-02        |
         |                               (1997)      (1997)          |
         |                                DH-485 bus                 |
         | [Cradlepoint]                                             |
         |  ww-gw-02                                                 |
         |  (2022)                                                   |
         |  Cellular WAN ---- 4G/LTE carrier ---- vendor cloud       |
         +------------------------------------------------------------+
```

## Device Inventory

| Placement ID  | Device                  | Era  | Role                              | Teaching Significance |
|---------------|-------------------------|------|-----------------------------------|-----------------------|
| ww-plc-01     | SLC-500/05 (ww-influent)| 1997 | Influent screening, primary treat | Writable totalizer (permit attack), serial-only access |
| ww-plc-02     | SLC-500/05 (ww-effluent)| 1997 | Secondary clarification, discharge| Multi-step interlock attack, WAS pump consequence |
| ww-plc-03     | CompactLogix (ww-aerat) | 2013 | Aeration blower DO control        | DO setpoint primary attack surface, VFD architecture |
| ww-gw-01      | Moxa NPort (ww-serial)  | 2008 | DH-485 to Ethernet bridge         | Protocol layering, ProSoft architecture |
| ww-gw-02      | Cradlepoint IBR600      | 2022 | Vendor cellular remote access     | Default credentials, internet exposure, flat network reach |

## Teaching Points

### 1. Partial Segmentation Is Common

Level 3 received a managed switch and VLAN after the 2018 compliance audit. Everything else
remained flat. The audit checkbox was checked. An attacker on ww-flat can reach four of five
simulated devices -- only the (unmodeled) SCADA server is protected. This pattern repeats across
real-world OT environments: segmentation projects start with the most visible asset and stall.

### 2. Era Mixing Creates Unpredictable Attack Surfaces

A 1997 SLC-500 shares a switch with a 2022 cellular gateway. Neither was designed with the
other's threat model in mind. The SLC-500 was designed for a world with no internet exposure.
The Cradlepoint was designed for enterprise IT. Placing the Cradlepoint on the flat OT segment
gives the internet a path to 1997-era devices with no authentication.

### 3. Unauthorized Additions Accumulate

The Cradlepoint was intended as a temporary vendor access device. Four years later, it is still
present, still using default credentials, and still NAT-forwarding access to the entire OT
segment. The plant staff who approved the temporary access may not know it is still running.
Network discovery exercises should highlight unknown devices as the first finding.

### 4. Serial Backbone Persistence

The DH-485 bus still carries process data from 1997. It cannot be replaced without a full plant
shutdown -- and even then, replacing it requires re-engineering all SLC-500 I/O wiring. The Moxa
NPort added Ethernet visibility in 2008 but added no security. An attacker who understands the
ProSoft MVI46-MCM architecture can poll the SLC-500 registers via Modbus TCP through the gateway
with zero authentication.

### 5. Managed Switch Provides Real But Incomplete Protection

The 2018 managed switch is not theater -- VLAN 30 is a real, enforced boundary. Traffic from the
flat Level 1 segment cannot reach the SCADA server without crossing the managed switch boundary.
But the managed switch protects exactly one device. The teaching point is not that the managed
switch is ineffective; it is that partial segmentation creates false confidence. The facility
passed its compliance audit. The actual security posture improvement was minimal.

## Systems Not Modeled

The following systems exist at any real municipal wastewater treatment plant of this size but are
excluded from simulation to keep the educational focus on network architecture, not process
completeness:

- **Odor control**: Hydrogen sulfide (H2S) scrubbing systems at headworks and sludge handling.
  PLCs monitor H2S levels and control chemical scrubber dosing. Excluded because the teaching
  focus is network architecture, not atmospheric hazard management.

- **Digester gas handling**: Anaerobic digesters produce methane that must be captured, flared,
  or used for cogeneration. Instrumentation includes gas flow meters, pressure regulators, and
  flame safety interlocks. Excluded because anaerobic digestion is an advanced process topic
  beyond the scope of this network architecture exercise.

- **Biosolids dewatering**: Belt filter presses or centrifuges reduce digested sludge water
  content for disposal or land application. Excluded because biosolids handling is operationally
  significant but adds no new network architecture teaching content at this stage.

These exclusions are acknowledged as technical debt TD-030 scope limitations. Future simulation
expansion could add these systems to create a more complete facility model.

## Contrast: All Three Environment Archetypes

| Aspect                 | Water Treatment        | Pipeline Station        | Wastewater (this env)       |
|------------------------|------------------------|-------------------------|-----------------------------|
| Archetype              | Modern segmented       | Legacy flat             | Frankenstein hybrid         |
| Era span               | 2024 (single build)    | 2015 (single build)     | 1997-2022 (25 years)        |
| Purdue compliance      | Full (L1-L3)           | None                    | Partial (L3 only)           |
| Segmentation           | Managed switches, VLANs| None                    | One managed switch (2018)   |
| Serial presence        | Via gateway (mfg side) | One serial link         | Original backbone, active   |
| Internet exposure      | None                   | None                    | Cellular gateway (vendor)   |
| Monitoring blind spots | Minimal (SPAN on all)  | Unmanaged switch        | Mixed: SPAN on L3, none L1  |
| Attack surface         | Smallest               | Medium (flat, air-gap)  | Largest (flat+internet+seg) |
