# Environment Authoring Guide

> **Schema version**: This guide covers schema v0.1. If examples do not match your files, check
> the schema version. Future SOWs that modify referenced YAML must update the examples in this guide.

This guide walks you through creating a new OT environment from scratch using the design layer's
YAML schema, the `ot-design` scaffold tool, and the validation CLI. You do not need to read any
Go source code or reverse-engineer existing YAML files to follow it.

The pipeline monitoring station (natural gas compressor station) is the primary worked example
because it exercises every placement pattern the schema supports: direct Ethernet devices,
serial devices behind a gateway, a multi-homed device, and a flat station LAN contrasted with
the Purdue-segmented water treatment network.

The Quick Start section at the end is a condensed hands-on walkthrough that takes you from an
empty directory to a validated, runnable environment in about 15 minutes.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Concepts](#concepts)
3. [Step 1: Plan Your Facility](#step-1-plan-your-facility)
4. [Step 2: Create Device Atoms](#step-2-create-device-atoms)
5. [Step 3: Create Network Atoms](#step-3-create-network-atoms)
6. [Step 4: Create the Environment](#step-4-create-the-environment)
7. [Step 5: Validate](#step-5-validate)
8. [Step 6: Run It](#step-6-run-it)
9. [Quick Start: Minimal Environment](#quick-start-minimal-environment)
10. [Reference](#reference)

---

## Prerequisites

Before authoring an environment, you need the following tools installed and working:

| Tool | Purpose | Notes |
|------|---------|-------|
| Go toolchain (1.21+) | Build the `ot-design` CLI | `go version` to check |
| Docker and Docker Compose v2 | Run the environment | `docker compose version` to check |
| Text editor with YAML support | Edit `.yaml` files | VS Code, vim, nano, any editor |
| `mbpoll` | Verify Modbus TCP responses | Install via package manager |

### Building the ot-design CLI

```
cd /path/to/ot-simulator/design
go build -o ot-design ./cmd/ot-design/
```

After building, verify it works:

```
$ ./ot-design --help
ot-design - Design layer validation and scaffolding for the OT simulator

Usage:
  ot-design validate <path>           Validate a device atom, network atom, or environment
  ot-design scaffold --device         Write a device atom skeleton to stdout
  ot-design scaffold --network        Write a network atom skeleton to stdout
  ot-design scaffold --environment    Write an environment skeleton to stdout
  ot-design --help                    Print this usage information
...
```

Throughout this guide, commands assume the `ot-design` binary is in your working directory or on
your `PATH`. Commands run from the OT simulator project root (`/path/to/ot-simulator/`).

---

## Concepts

This section gives you enough background to follow the guide. For architectural rationale, see the
referenced ADRs. This guide shows you *how* to create environments -- for *why* the schema is
designed the way it is, see ADR-009.

### Three-Layer Architecture

```
design/ --YAML--> plant/ --network--> monitoring/
```

- **design/**: Pure YAML. Defines *what* to simulate: device atoms, network atoms, environment
  definitions. No Go code. This is what you edit as an environment author.
- **plant/**: The runtime engine. Reads the design layer YAML and instantiates Modbus TCP servers
  for each device, producing realistic OT protocol traffic on the network.
- **monitoring/**: Observes the plant externally over the network. It cannot import plant packages.
  All interaction is over Modbus TCP and packet capture.

When you author a new environment, you only touch `design/`. The plant binary picks it up via the
`--environment` flag with no code changes required.

### Device Atoms

A **device atom** (`design/devices/*.yaml`) describes a specific hardware model -- what it is, how
it connects, and what register addresses it exposes. It is a reusable blueprint, not a deployed
instance. The same `compactlogix-l33er.yaml` device atom is placed three times in the water
treatment environment (intake, treatment, distribution) and three times in the quickstart lab --
six deployed instances from one atom. See ADR-004 for device atom schema design.

### Network Atoms

A **network atom** (`design/networks/*.yaml`) describes a network segment: its type (Ethernet or
serial), subnet, VLAN, and monitoring capability. Network atoms are also reusable. The `wt-level1`
network atom serves both the greenfield water/manufacturing environment and the quickstart lab --
same subnet, different placements.

### Environments and Placements

An **environment** (`design/environments/<id>/environment.yaml`) wires device atoms onto network
atoms with specific addressing. Each **placement** assigns one device atom to one network with a
unique IP address, Modbus port, register map variant, and role description. The environment is
the instantiation layer -- it is where abstract device blueprints become specific deployed devices.
See ADR-009 for environment schema design.

### Register Map Variants

Most device types serve multiple roles. The `compactlogix-l33er` device atom defines three variants:
`water-intake`, `water-treatment`, and `water-distribution`. Each variant specifies a different set
of holding registers and coils appropriate to that role. The placement selects a variant via the
`register_map_variant` field. If no variant is specified, the plant exposes empty registers within
the device's capacity limits.

### Modbus Master/Slave (Client/Server) Polling Model

Modbus is a request/response protocol. Devices (slaves/servers) do not transmit data on their own
-- they wait for a master (client) to poll them. In a real OT environment, the SCADA system or HMI
acts as the master, continuously polling PLCs and RTUs for register values. In the simulator, your
Modbus client (`mbpoll`) plays the SCADA master role.

**The simulator produces no Modbus traffic until you poll it.** If you start the environment, open
a packet capture, and see no traffic, that is correct behavior -- not a fault. Traffic appears only
when a client sends a request.

This is why real OT security tools like Dragos and Zeek rely on SPAN-port capture of existing SCADA
polling traffic, not on passive device discovery. The SCADA master initiates all communication.

---

## Step 1: Plan Your Facility

Before writing any YAML, answer four questions about the facility you want to model:

1. What industry and site type?
2. What devices exist at the site?
3. What network topology connects them?
4. What Modbus port range will you use?

### Planning the Pipeline Monitoring Station

Here is how these questions were answered for the pipeline station:

**Industry and site type**: Natural gas transmission pipeline. The site is a remote, unmanned
compressor station that boosts gas pressure along the pipeline. Operators monitor it from a
central control room over a WAN link.

**Devices at the site**:

| Device | Role | Protocol |
|--------|------|----------|
| Allen-Bradley CompactLogix L33ER | Station PLC and SCADA gateway | Modbus TCP (zero-based) |
| Emerson ROC800 (x2) | Pipeline metering RTU, station monitoring RTU | Modbus TCP (one-based) |
| Moxa NPort 5150 | Serial-to-Ethernet gateway | Modbus TCP (gateway) |
| ABB TotalFlow XFC G5 | Gas flow computer (GC interface) | Modbus RTU via gateway |

**Network topology**:

```
[SCADA Master - not simulated]
       |
 ps-wan-link (satellite/cellular, 10.20.0.0/30)
       |
  [ps-plc-01]  CompactLogix (station PLC / SCADA gateway, dual-homed)
       |
ps-station-lan (flat Ethernet, 10.20.1.0/24)
/      |      \
[ps-rtu-01] [ps-rtu-02] [ps-gw-01]  Moxa NPort 5150
ROC800        ROC800           |
metering      monitoring  ps-serial-bus (RS-485 serial link)
                               |
                          [ps-fc-01]  TotalFlow G5
                          gas flow computer
```

**Why flat station LAN?** A flat network is appropriate for small, remote OT sites (like an
unmanned pipeline station) where the cost and complexity of Purdue-segmented architecture is not
justified. The station has six IP devices on one subnet with an unmanaged switch. This is a known
and accepted security tradeoff, not an oversight: "A flat network at a remote station means any
device compromise gives lateral access to all other devices. This is accepted because the station
is air-gapped or VPN-connected to SCADA, has few devices, and the cost of managed switches and
firewalls exceeds the risk reduction." The `greenfield-water-mfg` environment shows what proper
Purdue segmentation looks like when facility size and risk warrant it. Network topology is a
risk-informed engineering decision, not a one-size-fits-all template.

**Modbus port range**: Port range 5040-5049 is reserved for pipeline. This avoids collision with
water/mfg (5020-5039). See the Port Range Assignments table in the Reference section.

---

## Step 2: Create Device Atoms

If existing device atoms cover your hardware, skip ahead to Step 3. Check `design/devices/` for
existing atoms before creating new ones.

### Scaffold a Device Skeleton

The scaffold command writes a complete, commented YAML skeleton to stdout:

```
ot-design scaffold --device > design/devices/emerson-roc800.yaml
```

The skeleton output begins:

```yaml
# Device Atom Skeleton - generated by ot-design scaffold --device
# Edit all placeholder values before use.
# See design/devices/compactlogix-l33er.yaml for a complete zero-based example.
# See design/devices/slc-500-05.yaml for a complete one-based example.
# Reference: ADR-009 D3 - Device Atom Schema v0.1

schema_version: "0.1"

device:
  id: "your-device-id"
  vendor: "Your Vendor Name"
  model: "Your Model Name"
  category: "plc"
  vintage: 2020
  description: "Brief description of what this device does and why it matters for OT security"
...
```

Edit every placeholder value. The `id` field must match the filename without `.yaml`. Use
kebab-case for both.

### Worked Example: Emerson ROC800

Here is the connectivity and capabilities section of the ROC800 device atom, showing the key
decisions made when authoring it:

```yaml
connectivity:
  ports:
    # Ethernet: Modbus TCP slave on port 502.
    # The SCADA master polls this device; the ROC800 does not initiate connections.
    - type: "ethernet"
      protocols: ["modbus-tcp"]
    # RS-232 COMM2 port: Modbus RTU slave for local serial connections.
    # RS-232 (not RS-485) -- point-to-point only, not multidrop.
    - type: "rs232"
      protocols: ["modbus-rtu"]
  response_delay_ms: 20     # 15-30ms typical: embedded PowerPC, not a high-speed PLC
  response_jitter_ms: 30    # Models burst-latency during AGA calculation cycles
  concurrent_connections: 4 # ROC800 Ethernet supports max 4 TCP connections

registers:
  max_holding: 2000
  max_coils: 500
  max_input_registers: 2000
  max_discrete_inputs: 500
  addressing: "one-based"   # Emerson convention: registers start at address 1
  float_byte_order: "big-endian"
  max_registers_per_read: 125
```

Key decisions:
- **`addressing: "one-based"`**: The ROC800 firmware and ROCLink 800 configuration software both
  use one-based register numbering. This is the Emerson convention.
- **`response_jitter_ms: 30`**: The ROC800 runs AGA gas flow calculations on its embedded
  PowerPC processor. During calculation cycles, response time spikes. The jitter models this.
- **`concurrent_connections: 4`**: One connection is typically reserved for ROCLink 800
  configuration software. Effective SCADA polling connections are 3.

### Defining Register Map Variants

Register map variants define role-specific layouts. The ROC800 has two variants: `pipeline-metering`
(AGA-3 custody transfer) and `station-monitoring` (pressures, temperatures, valve control).

Here is the beginning of the `pipeline-metering` variant:

```yaml
register_map_variants:
  pipeline-metering:
    # Custody-transfer gas metering station: meter run 1 with full AGA-3 inputs.
    # Addresses are one-based per Emerson ROC800 convention.
    holding:
      - address: 1
        name: "meter_run_1_flow_rate"
        description: "Meter run 1 instantaneous flow rate (AGA-3 calculated from DP, pressure, temp)"
        unit: "MSCFH"
        scale_min: 0
        scale_max: 5000
        writable: false
      - address: 2
        name: "meter_run_1_volume_today"
        description: "Meter run 1 accumulated volume since contract day rollover"
        unit: "MCF"
        scale_min: 0
        scale_max: 100000
        writable: false
      - address: 3
        name: "meter_run_1_pressure"
        description: "Meter run 1 static line pressure upstream of orifice plate"
        unit: "PSIG"
        scale_min: 0
        scale_max: 1500
        writable: false
      - address: 4
        name: "meter_run_1_temperature"
        description: "Meter run 1 flowing gas temperature"
        unit: "degF"
        scale_min: -20
        scale_max: 150
        writable: false
      - address: 5
        name: "meter_run_1_differential_pressure"
        description: "AGA-3 primary measurement input. DP across orifice plate is the
          financial manipulation attack surface: spoofing DP alters calculated volume."
        unit: "inH2O"
        scale_min: 0
        scale_max: 200
        writable: false
      - address: 6
        name: "station_total_flow"
        description: "Sum of all active meter run flow rates (station aggregate)"
        unit: "MSCFH"
        scale_min: 0
        scale_max: 20000
        writable: false
      - address: 7
        name: "station_total_volume_today"
        description: "Station accumulated total volume since contract day rollover"
        unit: "MCF"
        scale_min: 0
        scale_max: 100000
        writable: false
```

**Key observations about this register map**:

- **One-based addresses (1-7)**: The first valid address is 1, not 0. This matches the ROC800
  firmware documentation and ROCLink 800 configuration software.
- **Non-contiguous by design**: Real PLC register maps are not contiguous arrays. The ROC800 full
  register map would continue beyond address 7 with meter run 2, 3, 4 registers (addresses 8
  onward), with gaps between groups. Each group corresponds to a different section of the PLC
  program, and gaps reflect registers used by other program sections or reserved for future use.
- **Scale values are engineering ranges, not raw register limits**: `scale_min: 0` and
  `scale_max: 100000` on `meter_run_1_volume_today` means a raw register value of 0 maps to
  0 MCF and a raw register value of 32767 maps to 100000 MCF. The register resets to 0 at the
  contract day rollover -- this is normal behavior, not a malfunction.

### Zero-Based vs One-Based Addressing: The Most Common IT-to-OT Mistake

Different device manufacturers use different register addressing conventions. The CompactLogix
uses zero-based addressing (first register is address 0). The ROC800 and TotalFlow G5 use
one-based addressing (first register is address 1).

**This mismatch causes the most common error for IT engineers polling OT devices.**

**Scenario**: You are using `mbpoll` (a zero-based client by default) to read the ROC800's
`meter_run_1_flow_rate`, which the device documentation says is at register address 1.

**Incorrect** (zero-based client reading address 0 on a one-based device):

```
$ mbpoll -a 1 -t 4 -r 0 -c 1 10.20.1.20 -p 5041
-- Polling slave 1... Ctrl-C to stop)
[1]: [ERROR] Illegal data address (0x02)
```

The device returns an `Illegal Data Address` exception (Modbus exception code 0x02) because
address 0 does not exist in the one-based ROC800 register map. The device's first valid holding
register is address 1.

**Correct** (reading address 1, which is the first valid register):

```
$ mbpoll -a 1 -t 4 -r 1 -c 1 10.20.1.20 -p 5041
-- Polling slave 1... Ctrl-C to stop)
[1]: 3247
```

Register value 3247 maps to approximately 487 MSCFH using the scale range (0-5000 MSCFH across
the 0-32767 raw register range).

**The fix**: When using a zero-based client against a one-based device, subtract 1 from every
documented register address. Or, if your client supports it (some Modbus clients have an
`--offset` or `--one-based` mode), configure the client for one-based addressing.

Check the `addressing` field in the device atom before polling:
- `addressing: "zero-based"` -- poll at the documented address directly
- `addressing: "one-based"` -- subtract 1 when using a zero-based client

### Sidebar: Reusing Existing Device Atoms

If a device atom already exists in `design/devices/` for your hardware, you do not need to create
a new one. You can reuse it directly in your environment with a new placement. The quickstart lab
demonstrates this: it places `compactlogix-l33er` three times without modifying the device atom.

If you need a new register map variant on an existing device (for example, a CompactLogix
configured as a compressor controller rather than a water treatment PLC), add the variant to
the existing device atom. The `compressor-control` variant was added to `compactlogix-l33er.yaml`
for the pipeline station rather than creating a new device atom.

---

## Step 3: Create Network Atoms

### Scaffold a Network Skeleton

```
ot-design scaffold --network > design/networks/ps-station-lan.yaml
```

The skeleton:

```yaml
# Network Atom Skeleton - generated by ot-design scaffold --network
schema_version: "0.1"

network:
  id: "your-network-id"
  name: "Your Network Name"
  type: "ethernet"    # ethernet | serial-rs485 | serial-rs232 | serial-rs422
  description: "Brief description of this network segment and its role in the facility"

properties:
  subnet: "10.0.0.0/24"   # Required for ethernet; omit for serial types
  # vlan: 10              # Optional: present only on managed switches
  managed_switch: false
  span_capable: false
```

### Ethernet Networks

The pipeline station LAN is an unmanaged Ethernet switch with no VLANs and no SPAN port. Here
is the complete `ps-station-lan.yaml`:

```yaml
# ps-station-lan.yaml - Pipeline Station Flat Ethernet Network Atom
# Flat Ethernet for a remote, unmanned compressor station.
# No VLANs, no managed switch, no segmentation -- all IP devices share a single subnet.
# A flat network at a remote station means any device compromise gives lateral access
# to all other devices. This is a known and accepted security tradeoff, not an oversight.
schema_version: "0.1"

network:
  id: "ps-station-lan"
  name: "Pipeline Station LAN"
  type: "ethernet"
  description: "Flat Ethernet for remote compressor station -- single subnet, no segmentation"

properties:
  subnet: "10.20.1.0/24"
  managed_switch: false   # Unmanaged switch: no SPAN port, no port mirroring
  span_capable: false     # Passive monitoring tools cannot observe this segment
```

The `/24` subnet holds 254 usable host addresses for 4 devices -- intentionally oversized because
operators leave room for growth. This is realistic OT network design.

Contrast this with `wt-level1`, which has a managed switch, VLAN 10, and SPAN capability --
features that cost more and require a managed switch, but enable Dragos or Zeek to observe traffic
passively:

```yaml
# wt-level1.yaml - Water Treatment Level 1 Network Atom
schema_version: "0.1"

network:
  id: "wt-level1"
  name: "Water Treatment Level 1"
  type: "ethernet"
  description: "Basic Process Control - PLCs and I/O modules"

properties:
  subnet: "10.10.10.0/24"
  vlan: 10
  managed_switch: true    # Supports SPAN port for passive monitoring
  span_capable: true      # Dragos or Zeek can observe this segment
```

### Serial Networks

Serial networks have `type: "serial-rs485"` (or `serial-rs232`) and **no `subnet` field**. Serial
buses have no IP addressing -- they are physical electrical buses, not IP networks.

Here is `ps-serial-bus.yaml`:

```yaml
# ps-serial-bus.yaml - Pipeline Station RS-485 Serial Link Network Atom
schema_version: "0.1"

network:
  id: "ps-serial-bus"
  name: "Pipeline Station RS-485 Serial Link"
  type: "serial-rs485"
  description: "RS-485 serial link from Moxa gateway to TotalFlow G5 flow computer"

properties:
  managed_switch: false   # Serial link: no switch, no monitoring tap point
  span_capable: false     # No passive monitoring -- gateway is the only IP endpoint
```

> **Security Implication -- Serial Bus Asset Visibility**
>
> OT asset inventory tools that rely on IP discovery (Nmap, Shodan, Dragos asset identification)
> will not find devices on the serial bus. Only the gateway IP is visible to network scanners.
> This is a systematic blind spot in OT security tooling -- the serial bus may contain the
> majority of field devices at a site, yet none appear in IP-based asset inventories. Physical
> site surveys or gateway configuration audits are the only reliable discovery methods for serial
> devices.
>
> In the pipeline station, the TotalFlow G5 gas flow computer is completely invisible to IP-based
> tools. Only `ps-gw-01` (the Moxa NPort 5150 at `10.20.1.30`) is discoverable. An attacker who
> reaches the gateway gains Modbus RTU access to the serial bus and all devices on it.

---

## Step 4: Create the Environment

### Scaffold an Environment Skeleton

```
mkdir -p design/environments/pipeline-monitoring
ot-design scaffold --environment > design/environments/pipeline-monitoring/environment.yaml
```

The scaffold writes a stderr message reminding you of the target path -- the environment YAML must
be at `design/environments/<your-env-id>/environment.yaml`, and `id` must match the directory name.

### Add Network References

Every network used by any placement must appear in the `networks:` list. Networks not referenced
by any placement should still be listed if they are needed to validate cross-network placement
features (such as multi-homed devices or gateway bridges).

```yaml
networks:
  - ref: "ps-station-lan"   # Flat Ethernet: ROC800 RTUs, Moxa gateway, CompactLogix
  - ref: "ps-serial-bus"    # RS-485: TotalFlow G5 flow computer (behind Moxa gateway)
  - ref: "ps-wan-link"      # WAN backhaul: CompactLogix dual-homed for SCADA access
```

Each `ref` must match a filename in `design/networks/` without the `.yaml` extension.

### Add Placements

#### Ethernet Device Placement

A direct Modbus TCP device on an Ethernet network. Requires `ip` and `modbus_port`.

```yaml
placements:
  # Pipeline metering RTU -- custody-transfer flow measurement.
  - id: "ps-rtu-01"
    device: "emerson-roc800"
    network: "ps-station-lan"
    ip: "10.20.1.20"
    modbus_port: 5041
    role: "Pipeline metering RTU -- custody-transfer flow measurement (AGA-3)"
    register_map_variant: "pipeline-metering"
```

- `id`: Unique within this environment. Convention: `<facility-prefix>-<type>-<number>`.
- `device`: Matches a filename in `design/devices/` without `.yaml`.
- `network`: Must be listed in the `networks:` section above.
- `ip`: IPv4 address within the network's subnet (`10.20.1.0/24` for `ps-station-lan`).
- `modbus_port`: Host-accessible port. Must be unique across all placements in all environments
  that will run simultaneously. Use the port range reserved for your environment.
- `register_map_variant`: Must match a key in the device atom's `register_map_variants` section.

#### Gateway Placement (bridges Field)

A gateway device bridges an Ethernet network to a serial bus. It requires `bridges` in addition
to the standard Ethernet placement fields.

```yaml
  # Serial-to-Ethernet gateway for the TotalFlow G5 flow computer.
  - id: "ps-gw-01"
    device: "moxa-nport-5150"
    network: "ps-station-lan"
    ip: "10.20.1.30"
    modbus_port: 5043
    role: "Serial-to-Ethernet gateway for TotalFlow G5 gas analyzer"
    register_map_variant: "pipeline-serial"
    bridges:
      # from_network: always the Ethernet side (the IP network the gateway is on).
      # to_network: always the serial side (the RS-485 bus the devices are on).
      - from_network: "ps-station-lan"
        to_network: "ps-serial-bus"
```

The `bridges` field is required for devices with `category: "gateway"`. The validator enforces
this (ENV-016). The `from_network` must be an Ethernet network; the `to_network` must be the
serial network.

#### Serial Device Placement (gateway + serial_address)

A serial device is accessed via the gateway's IP and port. It does **not** have `ip` or
`modbus_port` fields -- it has no IP address.

```yaml
  # TotalFlow G5 gas flow computer -- serial device, accessed via ps-gw-01.
  # No IP, no modbus_port. Clients reach it via gateway IP 10.20.1.30, port 5043.
  - id: "ps-fc-01"
    device: "abb-totalflow-g5"
    network: "ps-serial-bus"
    serial_address: 1
    gateway: "ps-gw-01"
    role: "Gas flow computer -- gas composition analysis and BTU/SG calculation"
    register_map_variant: "gas-analysis"
```

- `network`: Must be the serial network (the one the gateway bridges *to*).
- `serial_address`: Modbus RTU unit address, range 1-247. Address 0 is broadcast (reserved).
- `gateway`: Must match a placement `id` of a gateway device in this environment.

To reach this device with `mbpoll`, you use the gateway's IP and port with the serial device's
unit address:

```
$ mbpoll -a 1 -t 4 -r 1 -c 3 10.20.1.30 -p 5043
```

The `-a 1` specifies unit address 1 (the TotalFlow G5's `serial_address`). Modbus TCP wraps the
unit address in the MBAP header -- the gateway routes the request to the serial device at that
unit address.

#### Multi-Homed Device (additional_networks)

A device with interfaces on more than one network. The primary network is in the main `network:`
field; secondary networks use `additional_networks`.

```yaml
  # Station PLC -- dual-homed on station LAN and WAN link.
  # Primary network: ps-station-lan (communicates with RTUs and gateway)
  # Secondary network: ps-wan-link (SCADA master polls this IP over satellite backhaul)
  - id: "ps-plc-01"
    device: "compactlogix-l33er"
    network: "ps-station-lan"
    ip: "10.20.1.10"
    modbus_port: 5040
    role: "Station PLC and SCADA gateway -- compressor control and data aggregation"
    register_map_variant: "compressor-control"
    additional_networks:
      # WAN-facing interface. The SCADA master polls this IP over the WAN link.
      # The CompactLogix listens on this IP but does NOT route between WAN and station LAN.
      - network: "ps-wan-link"
        ip: "10.20.0.2"
```

Each entry in `additional_networks` must specify a `network` that is listed in the environment's
`networks:` section. The IP must be within that network's subnet.

The security implication: compromising `ps-plc-01` exposes compressor control registers to the
WAN. But the CompactLogix does not route IP packets -- the attacker gains register-level access
to the PLC but not IP-level access to `10.20.1.20` (ps-rtu-01) or other station devices.

---

## Step 5: Validate

### Running ot-design validate

Validate an environment by pointing to its directory:

```
$ ot-design validate design/environments/pipeline-monitoring/
Validation passed: design/environments/pipeline-monitoring/
```

Exit code 0 means all checks passed. The validator resolves device and network references by
walking up the directory tree to find the design root (the directory containing `devices/`,
`networks/`, and `environments/`).

You can also validate individual device or network atoms:

```
$ ot-design validate design/devices/emerson-roc800.yaml
Validation passed: design/devices/emerson-roc800.yaml

$ ot-design validate design/networks/ps-station-lan.yaml
Validation passed: design/networks/ps-station-lan.yaml
```

### Common Validation Errors and Fixes

The validator reports errors in this format:

```
[ERROR] <file>: <field>: <message> [<rule-id>]
```

Each error includes a rule ID (ENV-xxx, DEV-xxx, NET-xxx) for traceability. Here are the five
most common errors and how to fix them.

---

**Error 1: Missing device reference (ENV-004)**

A placement references a device that does not exist in `design/devices/`.

```yaml
# Broken placement:
- id: "my-plc-01"
  device: "nonexistent-plc"    # No design/devices/nonexistent-plc.yaml exists
  network: "wt-level1"
  ip: "10.10.10.50"
  modbus_port: 5050
  register_map_variant: "water-intake"
```

Validation output:

```
[ERROR] design/environments/my-env/environment.yaml: placements[0].device:
  device "nonexistent-plc" not found in design/devices/ [ENV-004]
Validation complete: 1 error(s), 0 warning(s)
```

Fix: Check the filename in `design/devices/`. The `device` field must match the filename
without `.yaml`. If the device atom does not exist, create it with `ot-design scaffold --device`.

---

**Error 2: IP address outside subnet (ENV-008)**

A placement's IP is not within the network's subnet.

```yaml
# Broken placement:
- id: "my-rtu-01"
  device: "emerson-roc800"
  network: "ps-station-lan"   # subnet: 10.20.1.0/24
  ip: "192.168.1.10"          # Wrong network entirely
  modbus_port: 5041
  register_map_variant: "pipeline-metering"
```

Validation output:

```
[ERROR] design/environments/my-env/environment.yaml: placements[0].ip:
  address "192.168.1.10" is not within subnet "10.20.1.0/24" of network "ps-station-lan" [ENV-008]
Validation complete: 1 error(s), 0 warning(s)
```

Fix: Use an IP address within the network's subnet. Check the network atom's `subnet` field.

---

**Error 3: Modbus port collision (ENV-010)**

Two placements in the same environment use the same `modbus_port`.

```yaml
# Broken placements:
- id: "ps-plc-01"
  device: "compactlogix-l33er"
  network: "ps-station-lan"
  ip: "10.20.1.10"
  modbus_port: 5040
  register_map_variant: "compressor-control"

- id: "ps-rtu-01"
  device: "emerson-roc800"
  network: "ps-station-lan"
  ip: "10.20.1.20"
  modbus_port: 5040        # Collision with ps-plc-01
  register_map_variant: "pipeline-metering"
```

Validation output:

```
[ERROR] design/environments/my-env/environment.yaml: placements[1].modbus_port:
  port 5040 already used by placement "ps-plc-01" [ENV-010]
Validation complete: 1 error(s), 0 warning(s)
```

Fix: Assign a unique port to each Ethernet placement within the port range for your environment.

---

**Error 4: Serial device with IP field (ENV-017)**

A serial device (accessed via gateway) erroneously includes an `ip` or `modbus_port` field.

```yaml
# Broken placement:
- id: "ps-fc-01"
  device: "abb-totalflow-g5"
  network: "ps-serial-bus"
  ip: "10.20.1.40"          # Serial devices have no IP address
  modbus_port: 5044         # Serial devices have no modbus_port
  serial_address: 1
  gateway: "ps-gw-01"
  register_map_variant: "gas-analysis"
```

Validation output:

```
[ERROR] design/environments/my-env/environment.yaml: placements[4]:
  serial device accessed via gateway must not have an ip or modbus_port
  (serial devices are IP-invisible) [ENV-017]
Validation complete: 1 error(s), 0 warning(s)
```

Fix: Remove `ip` and `modbus_port` from serial placements. Serial devices are accessed via the
gateway's IP and port. Add `serial_address` and `gateway` instead.

---

**Error 5: Missing register map variant (ENV-005)**

A placement references a variant that does not exist in the device atom.

```yaml
# Broken placement:
- id: "qs-plc-01"
  device: "compactlogix-l33er"
  network: "wt-level1"
  ip: "10.10.10.50"
  modbus_port: 5050
  register_map_variant: "nonexistent-variant"   # Not in compactlogix-l33er.yaml
```

Validation output:

```
[ERROR] design/environments/my-env/environment.yaml: placements[0].register_map_variant:
  variant "nonexistent-variant" not found in device "compactlogix-l33er"
  (available: [water-intake water-treatment water-distribution compressor-control]) [ENV-005]
Validation complete: 1 error(s), 0 warning(s)
```

Fix: Use one of the available variant names listed in the error message. Check the device atom's
`register_map_variants` section to see all available variants.

---

## Step 6: Run It

### Adding a Docker Compose Profile

The committed `docker-compose.yml` defines profiles for `water` and `pipeline`. To run a new
environment, add a service definition and supporting network definitions to `docker-compose.yml`.

Here is a complete service definition for a hypothetical new environment named `my-station`,
modeled on the `plant-pipeline` service pattern:

```yaml
# In docker-compose.yml, add this network definition (networks: section):
my-station-lan:
  name: ot-my-station-lan
  driver: bridge
  ipam:
    config:
      - subnet: 10.30.0.0/24
        gateway: 10.30.0.1
  labels:
    ot.environment: "my-station"
    ot.description: "My station LAN - flat Ethernet"

# In docker-compose.yml, add this service definition (services: section):
plant-my-station:
  profiles: ["my-station"]
  build:
    context: .
    dockerfile: plant/Dockerfile
  container_name: ot-plant-my-station
  restart: unless-stopped
  command: ["--environment", "/design/environments/my-station", "--log-level", "info"]
  networks:
    my-station-lan:
      ipv4_address: 10.30.0.10
  ports:
    # Modbus TCP ports -- one per Ethernet placement in the environment.
    - "5060:5060"
    - "5061:5061"
  environment:
    - OTS_LOG_LEVEL=info
    # OTS_HEALTH_PORTS: list all Modbus TCP ports this environment exposes.
    # The plant binary health check probes these ports to confirm the server is listening.
    - OTS_HEALTH_PORTS=5060,5061
  healthcheck:
    test: ["CMD", "/plant", "--health"]
    interval: 5s
    timeout: 3s
    retries: 3
    start_period: 10s
  volumes:
    # Mount the design directory read-only. The plant binary reads environment YAML at startup.
    - ./design:/design:ro
  labels:
    ot.component: "plant-simulator"
    ot.environment: "my-station"
    ot.description: "My station plant simulation"
```

> **Port uniqueness across profiles**: All host-side port mappings must be globally unique across
> all Compose profiles, since multiple profiles can run simultaneously. Before assigning host
> ports, check the Port Range Assignments table in the Reference section. Docker maps container
> port 502 to unique host ports -- if two services map to the same host port, only the first to
> start will succeed. **Port collisions are a runtime error with no validation-time warning from
> `ot-design validate`.** The validator checks for collisions within a single environment; it does
> not cross-check against other environments or `docker-compose.yml`.

### Starting the Environment

Once the Compose service is defined:

```
$ docker compose --profile my-station up -d
[+] Running 1/1
 - Container ot-plant-my-station  Started
```

Check that it is healthy:

```
$ docker compose --profile my-station ps
NAME                    STATUS
ot-plant-my-station     running (healthy)
```

### Verifying with a Modbus Client

Poll a register to confirm the device is responding. For a CompactLogix (`addressing: "zero-based"`),
the first holding register is at address 0:

```
$ mbpoll -t 4 -r 0 -c 5 localhost -p 5060
-- Polling slave 1... Ctrl-C to stop)
[1]: 16000
[2]: 8192
[3]: 26000
[4]: 5
[5]: 12000
```

Each response is a 16-bit scaled integer (0-32767 range). To convert to engineering units, apply
the scale mapping from the device atom's register map variant:

```
engineering_value = scale_min + (raw_value / 32767.0) * (scale_max - scale_min)
```

For a ROC800 (`addressing: "one-based"`), subtract 1 when using `mbpoll` (which is zero-based by
default). The first one-based register is address 1 in the device documentation, but you request
it with `-r 0` when using a zero-based client:

```
$ mbpoll -a 1 -t 4 -r 0 -c 5 localhost -p 5041
```

Or, poll at address 1 explicitly if your client has a `--one-based` option.

See also the zero-based vs one-based addressing example in Step 2 and the Addressing Modes
reference table.

---

## Quick Start: Minimal Environment

> **Purdue context warning**: The 3-PLC, single-network topology in this Quick Start is a
> deliberate simplification. It models a single Purdue Level 1 segment, intended only for
> learning the authoring workflow. A production water treatment facility would isolate this
> network behind Purdue Level 2 (site operations) and Level 3 (site business) boundaries with
> firewalls and DMZs. For the complete Purdue model, see
> `design/environments/greenfield-water-mfg/environment.yaml`.

### What You Will Build

A minimal training lab with three CompactLogix PLCs on a single Ethernet network:

| Placement | Device | Variant | IP | Port |
|-----------|--------|---------|-----|------|
| qs-plc-01 | compactlogix-l33er | water-intake | 10.10.10.50 | 5050 |
| qs-plc-02 | compactlogix-l33er | water-treatment | 10.10.10.51 | 5051 |
| qs-plc-03 | compactlogix-l33er | water-distribution | 10.10.10.52 | 5052 |

This environment reuses `compactlogix-l33er.yaml` (existing device atom) and `wt-level1.yaml`
(existing network atom). No new atoms are needed. The IPs use the `.50-.52` range to avoid
collision with `greenfield-water-mfg`, which uses `.10-.12` on the same network atom.

### Step 1: Create the Environment Directory

```
mkdir -p design/environments/quickstart-example
```

### Step 2: Write the Environment YAML

Create `design/environments/quickstart-example/environment.yaml` with this content:

```yaml
# environment.yaml - Quickstart Training Lab
# IMPORTANT: This is a Purdue Level 1 simplification, not a production reference architecture.
# Three PLCs on a single flat network model one Level 1 segment only. A production water
# treatment facility would isolate this network behind Purdue Level 2 (site operations) and
# Level 3 (site business) boundaries with firewalls and DMZs. See
# design/environments/greenfield-water-mfg/environment.yaml for the complete Purdue model.
schema_version: "0.1"

environment:
  id: "quickstart-example"
  name: "Quickstart Training Lab"
  description: "Minimal 3-PLC environment for learning the authoring workflow."

networks:
  # wt-level1 subnet is 10.10.10.0/24.
  # Also used by greenfield-water-mfg with IPs .10-.12.
  # This environment uses .50-.52 to avoid address collision.
  - ref: "wt-level1"

placements:
  - id: "qs-plc-01"
    device: "compactlogix-l33er"
    network: "wt-level1"
    ip: "10.10.10.50"   # .50 -- avoids collision with greenfield-water-mfg (.10-.12)
    modbus_port: 5050   # Port range 5050-5059 reserved for quickstart/examples
    role: "Quickstart PLC 1 - Water Intake"
    register_map_variant: "water-intake"

  - id: "qs-plc-02"
    device: "compactlogix-l33er"
    network: "wt-level1"
    ip: "10.10.10.51"
    modbus_port: 5051
    role: "Quickstart PLC 2 - Water Treatment"
    register_map_variant: "water-treatment"

  - id: "qs-plc-03"
    device: "compactlogix-l33er"
    network: "wt-level1"
    ip: "10.10.10.52"
    modbus_port: 5052
    role: "Quickstart PLC 3 - Water Distribution"
    register_map_variant: "water-distribution"
```

### Step 3: Validate

```
$ ot-design validate design/environments/quickstart-example/
Validation passed: design/environments/quickstart-example/
```

Exit code 0. If you see errors, compare your file against the example above. The most common
mistakes are: wrong device ID (check `design/devices/` filenames), IP outside subnet (`wt-level1`
is `10.10.10.0/24` -- addresses must be `10.10.10.x`), and duplicate ports.

### Step 4: Run and Verify

The committed `docker-compose.yml` does not include a quickstart profile (adding it is Step 6
in the main guide). To run the quickstart environment, add the service and network definitions
from Step 6's example to `docker-compose.yml` with your chosen port range (5050-5052) and
the `wt-level1` Docker network.

Once running, verify with `mbpoll`. The CompactLogix uses zero-based addressing, so register
addresses start at 0:

```
# Read 5 holding registers from qs-plc-01 (water-intake variant)
$ mbpoll -t 4 -r 0 -c 5 localhost -p 5050
-- Polling slave 1... Ctrl-C to stop)
[1]: 16000   # intake_flow_rate (address 0)
[2]: 8192    # intake_pump_speed (address 1)
[3]: 26000   # raw_water_ph (address 2)
[4]: 5000    # raw_water_turbidity (address 3)
[5]: 12000   # intake_water_temp (address 4)
```

The registers are described in `design/devices/compactlogix-l33er.yaml` under the `water-intake`
variant. Apply the scale formula to convert raw values to engineering units:

```
intake_flow_rate = 0 + (16000 / 32767.0) * (100 - 0) = 48.8 L/s
```

---

## Reference

### Device Atom Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `schema_version` | Yes | string | Must be `"0.1"` |
| `device.id` | Yes | string | Unique kebab-case identifier; must match filename without `.yaml` |
| `device.vendor` | Yes | string | Manufacturer name |
| `device.model` | Yes | string | Specific model designation |
| `device.category` | Yes | string | One of: `plc`, `gateway`, `hmi`, `sensor`, `relay`, `safety-controller`, `rtu`, `flow-computer` |
| `device.vintage` | No | integer | Year hardware was released |
| `device.description` | No | string | Human-readable description for training context |
| `connectivity.ports` | Yes | list | At least one port with at least one protocol |
| `connectivity.ports[].type` | Yes | string | `ethernet`, `rs485`, `rs232`, `rs422`, `dh485` |
| `connectivity.ports[].protocols` | Yes | list | `modbus-tcp`, `modbus-rtu`, `dh-485` |
| `connectivity.response_delay_ms` | No | integer | Typical round-trip latency in ms |
| `connectivity.response_jitter_ms` | No | integer | Timing variation around `response_delay_ms` |
| `connectivity.concurrent_connections` | No | integer | Maximum simultaneous TCP connections |
| `registers.max_holding` | Yes | integer | Maximum holding registers (4xxxx space) |
| `registers.max_coils` | Yes | integer | Maximum coils (0xxxx space) |
| `registers.max_input_registers` | No | integer | Maximum input registers (3xxxx space) |
| `registers.max_discrete_inputs` | No | integer | Maximum discrete inputs (1xxxx space) |
| `registers.addressing` | Yes | string | `"zero-based"` or `"one-based"` |
| `registers.float_byte_order` | Yes | string | `big-endian`, `little-endian`, `big-endian-byte-swap`, `little-endian-byte-swap` |
| `registers.max_registers_per_read` | No | integer | Modbus FC03 max count; Modbus spec maximum is 125 |
| `register_map_variants` | No | map | Role-specific register layouts; key is variant name |
| `diagnostics.web_server` | No | bool | Whether device exposes a web management interface |
| `diagnostics.web_port` | No | integer | Web management port (0 if no web server) |
| `diagnostics.snmp` | No | bool | Whether device supports SNMP |
| `diagnostics.syslog` | No | bool | Whether device sends syslog |
| `diagnostics.fault_log` | No | bool | Whether device maintains internal fault log |
| `diagnostics.fault_log_depth` | No | integer | Number of fault log entries retained |

### Network Atom Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `schema_version` | Yes | string | Must be `"0.1"` |
| `network.id` | Yes | string | Unique kebab-case identifier; must match filename without `.yaml` |
| `network.name` | No | string | Human-readable display name |
| `network.type` | Yes | string | `ethernet`, `serial-rs485`, `serial-rs232`, `serial-rs422` |
| `network.description` | No | string | Description for training context |
| `properties.subnet` | Yes for ethernet | string | CIDR notation (e.g., `"10.10.10.0/24"`); omit for serial types |
| `properties.vlan` | No | integer | VLAN tag; present only on managed switches |
| `properties.managed_switch` | No | bool | Whether switch supports SPAN/port mirroring |
| `properties.span_capable` | No | bool | Whether passive monitoring tool can observe this segment |

### Environment Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `schema_version` | Yes | string | Must be `"0.1"` |
| `environment.id` | Yes | string | Unique kebab-case identifier; must match directory name |
| `environment.name` | No | string | Human-readable display name |
| `environment.description` | No | string | Description for training context |
| `networks` | Yes | list | All networks used by any placement; each entry is `ref: "<network-id>"` |

### Placement Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `placements[].id` | Yes | string | Unique within environment; convention: `<prefix>-<type>-<number>` |
| `placements[].device` | Yes | string | References `design/devices/<device>.yaml` |
| `placements[].network` | Yes | string | Must be listed in `networks:` above |
| `placements[].ip` | Yes for Ethernet | string | IPv4 address within network's subnet |
| `placements[].modbus_port` | Yes for Ethernet | integer | Unique host port; see Port Range Assignments |
| `placements[].role` | No | string | Human-readable description of this placement's function |
| `placements[].register_map_variant` | No | string | Selects register layout; must match key in device atom |
| `placements[].serial_address` | Yes for serial | integer | Modbus RTU unit address, range 1-247 |
| `placements[].gateway` | Yes for serial | string | Placement ID of the gateway device |
| `placements[].bridges` | Yes for gateway | list | Serial networks this gateway bridges to |
| `placements[].bridges[].from_network` | Yes | string | Ethernet network ID (IP side) |
| `placements[].bridges[].to_network` | Yes | string | Serial network ID (RS-485 side) |
| `placements[].additional_networks` | No | list | Secondary network interfaces for multi-homed devices |
| `placements[].additional_networks[].network` | Yes | string | Network ID; must be in `networks:` above |
| `placements[].additional_networks[].ip` | Yes | string | IPv4 address within that network's subnet |

### Port Range Assignments

Host-side Modbus TCP port assignments are coordinated across all environments. Multiple profiles
can run simultaneously, so every port must be globally unique.

| Range | Environment | Notes |
|-------|-------------|-------|
| 5020-5029 | Water treatment (`greenfield-water-mfg`) | Water treatment PLCs |
| 5030-5039 | Manufacturing floor (`greenfield-water-mfg`) | Manufacturing devices |
| 5040-5049 | Pipeline monitoring (`pipeline-monitoring`) | Compressor station devices |
| 5050-5059 | Quickstart/examples | `quickstart-example` and ad-hoc learning environments |
| 5060+ | Available | Reserve a decade-aligned range for each new environment |

Assignments within the water/mfg range:

| Port | Placement | Device |
|------|-----------|--------|
| 5020 | wt-plc-01 | CompactLogix (water-intake) |
| 5021 | wt-plc-02 | CompactLogix (water-treatment) |
| 5022 | wt-plc-03 | CompactLogix (water-distribution) |
| 5030 | mfg-gateway-01 | Moxa NPort 5150 (serial gateway) |

Assignments within the pipeline range:

| Port | Placement | Device |
|------|-----------|--------|
| 5040 | ps-plc-01 | CompactLogix (compressor-control) |
| 5041 | ps-rtu-01 | ROC800 (pipeline-metering) |
| 5042 | ps-rtu-02 | ROC800 (station-monitoring) |
| 5043 | ps-gw-01 | Moxa NPort 5150 (pipeline-serial) |

### Addressing Modes

| Device | Convention | First register address | mbpoll flag to read register 1 |
|--------|-----------|----------------------|-------------------------------|
| CompactLogix L33ER | zero-based | 0 | `-r 0` |
| Modicon 984 | one-based | 1 | `-r 0` (mbpoll subtracts 1 for zero-based wire format) |
| Emerson ROC800 | one-based | 1 | `-r 0` (mbpoll subtracts 1) |
| ABB TotalFlow G5 | one-based | 1 | `-r 0` (mbpoll subtracts 1) |
| SLC-500-05 | one-based | 1 | `-r 0` (mbpoll subtracts 1) |
| Moxa NPort 5150 | zero-based | 0 | `-r 0` |

**Note**: `mbpoll` uses zero-based addressing by default (the Modbus wire protocol uses zero-based
register addresses in the PDU). When device documentation says "register 1", you request it with
`-r 0` in mbpoll because the wire protocol sends address 0. This matches the way one-based device
firmware works: a firmware that labels its first register as "register 1" maps to wire address 0.

The distinction between zero-based and one-based in device atom YAML is about how the device
*documents* its register addresses relative to the wire address:
- `zero-based`: Device documentation and wire address match (address 0 is the first register)
- `one-based`: Device documentation adds 1 to the wire address (document says "register 1",
  wire sends address 0)

When `ot-design validate` checks that register addresses are within `max_holding`, it uses the
addressing convention declared in the device atom. Zero-based: valid range is 0 to max-1.
One-based: valid range is 1 to max.

### Common Validation Errors

| Rule ID | Error | Fix |
|---------|-------|-----|
| ENV-004 | Device `"<id>"` not found in `design/devices/` | Check filename in `design/devices/`; create with `ot-design scaffold --device` |
| ENV-005 | Variant `"<name>"` not found in device `"<id>"` | Use a variant name from the device atom's `register_map_variants` section |
| ENV-006 | Duplicate placement ID `"<id>"` | Each placement `id` must be unique within the environment |
| ENV-007 | Network `"<id>"` not listed in environment networks | Add the network to the `networks:` section |
| ENV-008 | IP address not within subnet | Use an IP within the network's CIDR subnet |
| ENV-009 | Duplicate IP on network | Each IP must be unique within a network |
| ENV-010 | Port already used by placement `"<id>"` | Assign a unique port; check the Port Range Assignments table |
| ENV-012 | Gateway does not bridge to network | The gateway's `bridges.to_network` must match the serial device's `network` |
| ENV-013 | Serial address already used on gateway | Each `serial_address` must be unique per gateway |
| ENV-014 | `additional_networks` network not in environment | Add the network to the `networks:` section |
| ENV-015 | `additional_networks` IP not within subnet | Use an IP within that network's CIDR subnet |
| ENV-016 | Gateway device has no `bridges` field | Add the `bridges` list to gateway placements |
| ENV-017 | Serial device has `ip` or `modbus_port` | Remove those fields; serial devices are IP-invisible |
| ENV-018 | Device has no port matching network type | Device lacks an Ethernet/RS-485 port matching the network type |
| ENV-019 | Serial address outside range 1-247 | Use a valid Modbus RTU unit address (1-247) |
