# ADR-009: Design Layer and Composable Environment Architecture

**Status**: Proposed
**Date**: 2026-03-01
**Decision Makers**: Rusty Brownlee
**Supersedes**: None (new ADR; refines ADR-003, ADR-004, ADR-007)

---

## Context

The original project architecture assumed a single hardcoded facility: a water treatment plant connected to a legacy manufacturing floor. All configuration, scenarios, and code were built around this specific topology. While educationally effective, this approach limits the project's ability to model different OT environments without code changes.

Real OT environments vary dramatically. A pharmaceutical plant looks different from a water utility. A power substation has different devices, protocols, and network patterns than a manufacturing floor. Security overlay products like Blastwave/BlastShield and Dragos must adapt to whatever environment they're deployed into -- they see network endpoints and communication patterns, not specific facility types.

To support evolving educational goals and avoid a costly future refactoring, the project needs a composable architecture where OT environments are assembled from atomic elements defined in configuration, not code.

### How Security Tools See OT Environments

The design layer must model what security tools actually observe, because that is what the target audience (IT engineers) will interact with:

**Network-level tools (Blastwave/BlastShield)**:
- See IP endpoints and MAC addresses
- See traffic patterns between endpoints
- Apply identity-based microsegmentation at network boundaries
- Cannot see behind serial-to-Ethernet gateways
- Cannot see devices without network interfaces

**Passive monitoring tools (Dragos, Zeek)**:
- See every packet on monitored segments (via SPAN port or TAP)
- Perform deep packet inspection of OT protocols (Modbus function codes, register addresses, values)
- Build behavioral baselines: "Device A polls Device B for registers 0-10 every 500ms"
- Detect anomalies: new device, unusual function code, write to normally-read-only register
- Cannot monitor segments without SPAN capability (unmanaged switches)

**The visibility gap**: A 1965 hydraulic press with no digital interface is invisible to all network-based tools. If it has a retrofitted PLC on an RS-485 bus behind a Moxa gateway, security tools see the gateway's IP address -- not the PLC, not the press. This visibility hierarchy is fundamental to the design layer schema.

---

## Decision

### D1: Three-Layer Project Architecture

**Decision**: The project is organized into three independent layers with strict boundaries.

| Layer | Directory | Purpose | Technology |
|-------|-----------|---------|------------|
| Design | `design/` | Device library, network templates, environment definitions | Pure YAML, documented schema |
| Plant | `plant/` | Runtime engine that instantiates any environment definition | Go module |
| Monitoring | `monitoring/` | External observation of the running plant | Go module (independent) |

**Boundaries**:
- Design is pure configuration. No Go code, no runtime logic. Plant owns all parsing and validation.
- Plant reads design layer artifacts and instantiates them. It does not assume any specific facility type.
- Monitoring cannot import plant packages. It observes over the network only (unchanged from ADR-005).
- A future Go module in design/ may provide guided editing of YAML files, but this is deferred.

**Information flow**:
```
design/ ──YAML──> plant/ ──network──> monitoring/
  (what to build)   (running sim)     (observation)
```

### D2: Three Atomic Element Types

**Decision**: The design layer defines three atomic element types that compose into environments.

**Devices** -- What exists in the facility. Defined once in the device library, referenced by many environments.
```
design/devices/{device-id}.yaml
```

**Networks** -- Communication media. Ethernet segments, serial buses, cross-plant links.
```
design/networks/{network-id}.yaml
```

**Environments** -- Topology templates that wire devices onto networks with specific addressing. An environment is what the plant binary actually instantiates.
```
design/environments/{environment-id}/environment.yaml
```

### D3: Device Atom Schema

**Decision**: A device atom describes what a device *is* and what it *can do*, independent of where it is deployed. The schema is versioned and layered to support incremental expansion.

**v0.1 (implemented now)**:

```yaml
schema_version: "0.1"

device:
  id: "compactlogix-l33er"
  vendor: "Allen-Bradley"
  model: "CompactLogix 5380 L33ER"
  category: "plc"          # plc | gateway | hmi | sensor | relay | safety-controller
  vintage: 2024
  description: "Modern Ethernet-native PLC for water treatment Level 1"

connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
    - type: "rs485"
      protocols: ["modbus-rtu"]
  response_delay_ms: 5
  response_jitter_ms: 3
  concurrent_connections: 10

# Register capabilities define what this device type supports.
# The device atom defines capacity and conventions, NOT role-specific register names.
# Named register maps (e.g., "water-intake", "water-distribution") are defined
# as register_map_variants and selected by the environment template placement.
registers:
  max_holding: 4096
  max_coils: 2048
  max_input_registers: 4096
  max_discrete_inputs: 2048
  addressing: "zero-based"
  float_byte_order: "big-endian"
  max_registers_per_read: 125

# Register map variants define role-specific register layouts.
# The same device type can serve different roles by loading different variants.
# Each variant must stay within the device's register capabilities above.
register_map_variants:
  water-intake:
    holding:
      - address: 0
        name: "intake_flow_rate"
        unit: "L/s"
        scale_min: 0.0
        scale_max: 100.0
        writable: false
    coils:
      - address: 0
        name: "intake_pump_01_run"
        writable: true
  water-distribution:
    holding:
      - address: 0
        name: "clear_well_level"
        unit: "%"
        scale_min: 0.0
        scale_max: 100.0
        writable: false
    coils:
      - address: 0
        name: "distribution_pump_01_run"
        writable: true

diagnostics:
  web_server: true
  web_port: 80
  snmp: false
  syslog: false
  fault_log: true
  fault_log_depth: 100
```

The environment template selects a variant via `register_map_variant: "water-intake"` in the placement. If no variant is specified, the device exposes empty registers within its declared capacity (useful for generic testing).

**v0.2 (future -- security profile)**:

```yaml
security_profile:
  authentication: "none"        # none | default-credentials | basic | certificate
  default_credentials:
    username: "admin"
    password: "admin"           # Only for devices with default creds (Moxa, etc.)
  agent_capable: false          # Can a security agent (BlastShield, etc.) run on this device?
  firmware_updatable: true      # Can firmware be updated remotely?
  network_discoverable: true    # Responds to ICMP, ARP, or active scanning
  encryption_capable: false     # Can the device encrypt protocol traffic?
```

**v0.3 (future -- physical asset mapping)**:

```yaml
physical_assets:
  - id: "intake-pump-01"
    type: "centrifugal-pump"
    description: "Raw water intake pump, 50HP"
    controlled_by:
      registers:
        run_command: "coil:0"
        speed_setpoint: "holding:1"
        flow_feedback: "holding:0"
  - id: "intake-pump-02"
    type: "centrifugal-pump"
    description: "Raw water intake pump, 50HP (redundant)"
    controlled_by:
      registers:
        run_command: "coil:1"
        speed_setpoint: "holding:6"
        flow_feedback: "holding:5"
```

### D4: Network Atom Schema

**Decision**: A network atom describes a communication medium and its properties, independent of which devices are connected to it.

**v0.1 (implemented now)**:

```yaml
schema_version: "0.1"

network:
  id: "wt-level1"
  name: "Water Treatment Level 1"
  type: "ethernet"            # ethernet | serial-rs485 | serial-rs232
  description: "Basic Process Control - PLCs and I/O modules"

properties:
  subnet: "10.10.10.0/24"    # Only for ethernet type
  vlan: 10                    # Optional VLAN tag
  managed_switch: true        # Does the switch support SPAN/mirroring?
  span_capable: true          # Can passive monitoring tap this segment?
```

**v0.2 (future -- monitoring overlay points)**:

```yaml
monitoring:
  span_ports_available: 2
  tap_insertion_points:
    - between: ["firewall-l2-l1", "switch-l1"]
      description: "TAP between L2 firewall and L1 switch"
  syslog_sources:
    - device: "switch-l1"
      format: "rfc5424"
```

**v0.3 (future -- access control)**:

```yaml
access_control:
  firewall: true
  acl_rules:
    - from: "wt-level2"
      to: "wt-level1"
      allow: ["modbus-tcp"]
      deny: ["ssh", "http"]
```

### D5: Environment Template Schema

**Decision**: An environment template wires devices onto networks with specific addressing. This is the artifact that the plant binary instantiates.

**v0.1 (implemented now)**:

```yaml
schema_version: "0.1"

environment:
  id: "greenfield-water-mfg"
  name: "Greenfield Water and Manufacturing Facility"
  description: "Dual-network facility with modern water treatment and legacy manufacturing"

# All networks used by this environment must be explicitly listed.
# This includes serial buses -- they are networks too, just non-IP ones.
networks:
  - ref: "wt-level3"
  - ref: "wt-level2"
  - ref: "wt-level1"
  - ref: "mfg-flat"
  - ref: "mfg-serial-bus"       # RS-485 bus behind the Moxa gateway
  - ref: "cross-plant"

# Placements assign devices to networks with specific addressing.
# A placement is the primary way devices appear in the environment.
# For devices on IP networks: ip + modbus_port.
# For devices on serial buses: serial_address + gateway reference.
placements:
  - id: "wt-plc-01"
    device: "compactlogix-l33er"
    network: "wt-level1"
    ip: "10.10.10.10"
    modbus_port: 5020
    role: "Water Treatment PLC 1 - Intake"
    register_map_variant: "water-intake"

  - id: "mfg-plc-01"
    device: "slc-500-05"
    network: "mfg-serial-bus"
    serial_address: 1
    gateway: "mfg-gateway-01"
    role: "Line A Conveyor and Assembly"
    register_map_variant: "mfg-line-a"

  - id: "mfg-gateway-01"
    device: "moxa-nport-5150"
    network: "mfg-flat"
    ip: "192.168.1.20"
    modbus_port: 5030
    role: "Serial-to-Ethernet Gateway"
    bridges:
      - from_network: "mfg-flat"
        to_network: "mfg-serial-bus"
    additional_networks:             # Gateway also on cross-plant link
      - network: "cross-plant"
        ip: "172.16.0.3"

  # The cross-plant link is modeled by placing devices on the cross-plant network.
  # Any two devices on the same network can communicate -- no separate
  # "connections" concept is needed. The network atom defines the medium;
  # the placements define who is on it.
  - id: "wt-plc-03"
    device: "compactlogix-l33er"
    network: "wt-level1"
    ip: "10.10.10.12"
    modbus_port: 5022
    role: "Water Treatment PLC 3 - Distribution"
    register_map_variant: "water-distribution"
    additional_networks:             # PLC also on cross-plant link
      - network: "cross-plant"
        ip: "172.16.0.2"
```

**Design note -- why no `connections` section**: An earlier draft included a separate `connections` block to model links between devices. This was removed because it duplicates what network membership already expresses. Two devices on the same network can communicate -- that is the definition of a network. The cross-plant link is modeled as a `cross-plant` network atom with two devices placed on it via `additional_networks`. This keeps the schema simpler: placements are the only way devices enter an environment, and networks are the only way devices communicate.

For cases where a device spans multiple networks (like wt-plc-03 on both wt-level1 and cross-plant, or a dual-homed engineering workstation), the `additional_networks` field on a placement lists secondary network attachments beyond the primary `network` field.

**v0.2 (future -- communication patterns)**:

```yaml
communication_patterns:
  - id: "hmi-polling-wt-plc-01"
    type: "periodic-poll"
    source: "wt-hmi-01"
    target: "wt-plc-01"
    protocol: "modbus-tcp"
    function_code: 3              # Read Holding Registers
    start_register: 0
    register_count: 6
    interval_ms: 1000
    description: "HMI polls intake PLC for process values every second"

  - id: "plc-interlock-wt"
    type: "event-driven"
    source: "wt-plc-03"
    target: "wt-plc-01"
    protocol: "modbus-tcp"
    function_code: 6              # Write Single Register
    trigger: "clear_well_level > 95%"
    description: "Distribution PLC signals intake to reduce flow when clear well is near full"
```

**v0.3 (future -- security overlay attachment)**:

```yaml
security_overlays:
  - id: "blastshield-cross-plant"
    product: "blastwave-blastshield"
    insertion_point: "cross-plant"
    mode: "inline"
    policy: "deny-all-allow-list"
    description: "BlastShield microsegmentation at cross-plant boundary"

  - id: "dragos-wt-level3"
    product: "dragos-platform"
    insertion_point: "wt-level3"
    mode: "passive-span"
    description: "Dragos sensor on SPAN port at Level 3"
```

### D6: Schema Versioning Strategy

**Decision**: All design layer YAML files include a `schema_version` field. The plant binary supports parsing all schema versions up to its current release. Older environments continue to work without modification when the schema evolves.

**Rules**:
- Adding optional sections (security_profile, communication_patterns) is a minor version bump
- Changing the meaning of existing fields is a major version bump (avoided when possible)
- The plant binary logs a warning when loading an older schema version
- The future design-layer Go tool validates and optionally migrates schema versions

### D7: Design Layer Directory Structure

**Decision**: The design layer follows a flat, discoverable structure.

```
design/
  schema/                         # Schema documentation and JSON Schema files (future)
    device.schema.md
    network.schema.md
    environment.schema.md
  devices/                        # Device library (atomic, reusable)
    compactlogix-l33er.yaml
    slc-500-05.yaml
    modicon-984.yaml
    moxa-nport-5150.yaml
    cradlepoint-ibr600.yaml       # Cellular modem (ADR-006)
  networks/                       # Network templates (atomic, reusable)
    wt-level1.yaml
    wt-level2.yaml
    wt-level3.yaml
    mfg-flat.yaml
    mfg-serial-bus.yaml
    cross-plant.yaml
  environments/                   # Complete environment templates
    greenfield-water-mfg/
      environment.yaml            # The topology definition
      README.md                   # Human-readable description of this environment
    manufacturing-only/           # Future: isolated manufacturing floor
      environment.yaml
      README.md
```

### D8: Plant Binary Generalization

**Decision**: The plant binary reads an environment template and instantiates it without assuming any specific facility type. The binary's configuration points to a design layer environment, not to facility-specific config sections.

**Plant startup flow**:
1. Parse `--environment` flag (path to environment directory in design/)
2. Load environment.yaml
3. Resolve device references from design/devices/
4. Resolve network references from design/networks/
5. Validate all placements (device ports match network types, no IP conflicts, gateways exist)
6. Instantiate virtual devices, bind to networks, start protocol servers
7. Begin simulation loop

**Breaking change from SOW-000.0**: The current plant binary uses `--config` pointing to `plant/config/plant.yaml` with facility-specific config structs (`WaterTreatmentConfig`, `ManufacturingConfig`). This ADR replaces that with `--environment` pointing to a design layer environment directory. The plant config structs must be generalized to load arbitrary topologies. The `plant/config/` directory is replaced by `design/`. This is a known refactoring cost of the architectural pivot and should be addressed in the next SOW.

---

## Consequences

### Positive
- Any OT environment can be modeled without code changes
- Device library grows organically as new device types are researched
- Scenarios can compose different environments for different teaching objectives
- Security overlay integration points are defined in config, enabling realistic deployment exercises
- Schema versioning prevents breaking changes from blocking existing environments
- The three-layer separation makes the project easier to understand and contribute to
- Future design-layer tooling has a clean, well-defined schema to build against

### Negative
- More YAML files to maintain (device library, network templates, environment definitions)
- Plant binary must implement generic topology instantiation instead of hardcoded facility setup
- Schema evolution requires careful version management
- Initial complexity is higher than a single-facility approach

### Risks
- **Risk**: Schema design may not accommodate unforeseen OT patterns
- **Mitigation**: Schema versioning allows non-breaking additions. Start minimal (v0.1), expand based on real experience.
- **Risk**: Device library may become inconsistent as it grows
- **Mitigation**: Future design-layer Go tool validates all YAML against schema. Schema documentation serves as source of truth.
- **Risk**: Environment templates may become complex for large facilities
- **Mitigation**: Environments can reference other environments (composition) in a future schema version. Start with flat, self-contained templates.

---

## References
- ADR-003: Dual-Network Architecture (refined by this ADR)
- ADR-004: Device Profiles and Vendor Simulation (refined by this ADR)
- ADR-007: Educational Scenario Framework (refined by this ADR)
- Blastwave BlastShield: Network cloaking and microsegmentation for OT
- Dragos Platform: Passive OT network monitoring and threat detection
- IEC 62443: Industrial communication networks -- Network and system security
