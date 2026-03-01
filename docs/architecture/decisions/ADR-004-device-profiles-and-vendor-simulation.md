# ADR-004: Device Atoms and Vendor Simulation

**Status**: Proposed (Revised 2026-03-01)
**Date**: 2026-02-28
**Decision Makers**: Rusty Brownlee
**Revised by**: ADR-009 (Design Layer and Composable Environments)

---

## Context

Real manufacturing floors are multi-vendor environments. One line runs Allen-Bradley, another runs Siemens, a third has Modicon PLCs from the 1990s. While these devices may all speak Modbus, their behavior, capabilities, and register conventions differ significantly.

### Vendor Differences That Matter for Security

| Characteristic | Modern PLC (CompactLogix) | Legacy PLC (SLC-500) | Ancient PLC (Modicon 984) |
|---------------|--------------------------|---------------------|--------------------------|
| Communication | Ethernet native | Serial only (RS-485/DH+) | Serial only (RS-485) |
| Modbus support | Via explicit message | Via serial interface | Native (Modbus inventor) |
| Memory | 750KB+ | 16-64KB | 4-16KB |
| Register count | Thousands | Hundreds | 256-999 |
| Diagnostics | Extensive (web server, fault log) | Basic (major/minor fault codes) | Minimal (LED indicators) |
| Firmware updates | Over Ethernet | Via serial + RSLogix 500 | Not field-updatable |
| Programming software | Studio 5000 (Windows 10+) | RSLogix 500 (Windows XP) | Modsoft (DOS/Windows 3.1) |
| Response time | 2-10ms | 10-50ms | 20-100ms |
| Web interface | Yes | No | No |
| SNMP | No (switch-level only) | No | No |
| Syslog | No | No | No |

### Why This Matters for Training

IT engineers need to understand that "PLC" is not a single category. A CompactLogix from 2024 and a Modicon 984 from 1994 are as different as a modern cloud server and a VAX/VMS minicomputer. The security assessment approach, available telemetry, and remediation options differ fundamentally.

### The Visibility Hierarchy

Security tools don't see devices directly. They see network endpoints and protocol behavior. What is observable depends on where a device sits in the connectivity hierarchy:

```
Physical Asset     "1965 Hydraulic Press"        <- what exists in the real world
  +- Controller    "SLC-500 PLC"                 <- what controls it
      +- Connectivity  "RS-485 at address 2"     <- how the controller is reachable
          +- Gateway    "Moxa NPort on Ethernet"  <- what makes it network-visible
              +- Network Endpoint  "192.168.1.20:502"  <- what security tools see
```

A device with only serial connectivity is invisible to network-based security tools unless a gateway bridges it to an IP network. Even then, the gateway's IP address is what appears -- not the device's serial address.

### Relationship to the Design Layer

Per ADR-009, devices are atomic elements defined in the design layer (`design/devices/`). This ADR defines what a device atom contains, how vendor-specific behavior is modeled, and how the device schema supports incremental expansion from basic simulation to full security profile modeling.

---

## Decision

### D1: Devices Are Atomic Design Elements

**Decision**: Each device type is defined as an atomic element in the design layer. A device atom describes what a device *is* and what it *can do*, independent of where it is deployed. Device atoms live in `design/devices/` and are referenced by environment templates.

**Key principle**: A device atom never knows where it will be deployed. Deployment-specific details (IP address, port assignment, network placement) belong in the environment template, not the device atom.

### D2: Generic Modbus First, Device Atoms as Behavioral Overlays

**Decision**: All virtual PLCs implement generic Modbus TCP/RTU behavior first. Device-specific behavior is applied from the device atom definition, which modifies response characteristics, register map conventions, and diagnostic capabilities.

**Rationale**: Building vendor-specific behavior into the PLC engine would create an unmaintainable mess. Device atoms allow adding new device types via configuration without code changes. Generic Modbus serves as the baseline -- if an engineer's tool works against generic Modbus, it works against any vendor's Modbus implementation.

### D3: Device Atom Schema Layers

**Decision**: The device atom schema is versioned and layered to support incremental expansion without breaking existing definitions. All layers are documented here even though v0.1 only implements the first three.

**Layer 1: Identity (v0.1)** -- What the device is.
```yaml
device:
  id: "compactlogix-l33er"
  vendor: "Allen-Bradley"
  model: "CompactLogix 5380 L33ER"
  category: "plc"          # plc | gateway | hmi | sensor | relay | safety-controller
  vintage: 2024
  description: "Modern Ethernet-native PLC for water treatment Level 1"
```

**Layer 2: Connectivity (v0.1)** -- How the device communicates.
```yaml
connectivity:
  ports:
    - type: "ethernet"
      protocols: ["modbus-tcp"]
    - type: "rs485"
      protocols: ["modbus-rtu"]
  response_delay_ms: 5
  response_jitter_ms: 3
  concurrent_connections: 10
```

**Layer 3: Registers (v0.1)** -- What data the device exposes via Modbus.
```yaml
registers:
  max_holding: 4096
  max_coils: 2048
  max_input_registers: 4096
  max_discrete_inputs: 2048
  addressing: "zero-based"
  float_byte_order: "big-endian"
  max_registers_per_read: 125
  register_map:
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
```

**Layer 4: Diagnostics (v0.1)** -- What telemetry the device exposes beyond Modbus.
```yaml
diagnostics:
  web_server: true
  web_port: 80
  snmp: false
  syslog: false
  fault_log: true
  fault_log_depth: 100
```

**Layer 5: Security Profile (v0.2 -- future)** -- How the device interacts with security tools.
```yaml
security_profile:
  authentication: "none"        # none | default-credentials | basic | certificate
  default_credentials:
    username: "admin"
    password: "admin"
  agent_capable: false          # Can a security agent (BlastShield) run on this device?
  firmware_updatable: true      # Can firmware be updated remotely?
  network_discoverable: true    # Responds to ICMP, ARP, or active scanning
  encryption_capable: false     # Can the device encrypt protocol traffic?
  patch_history:                # Known CVEs and patch availability
    last_patch: "2024-06"
    cve_exposure: []
```

**Layer 6: Physical Asset Mapping (v0.3 -- future)** -- What real-world equipment this controller manages.
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
    safety_implications:
      failure_mode: "pump cavitation, loss of intake water"
      safety_interlock: "low-suction-pressure-trip"
```

**Layer 7: Operational Constraints (v0.3 -- future)** -- Real-world operational realities that affect security decisions.
```yaml
operational_constraints:
  maintenance_window: "annual-shutdown-only"  # When can this device be patched/updated?
  programming_software: "Studio 5000"
  programming_os: "Windows 10+"
  backup_method: "project-file-export"        # How are PLC programs backed up?
  change_management: "manual-paper-log"       # How are changes tracked?
  mean_time_to_replace: "6-8 weeks"           # If this device dies, how long to replace?
```

### D4: Four Initial Device Atoms

**Decision**: Phase 1 ships with four device atoms representing the range of equipment found in the reference environment. Additional devices are added to the library as the project expands.

| Device Atom | Category | Environment | Key Characteristics |
|-------------|----------|-------------|-------------------|
| `compactlogix-l33er` | PLC | Water treatment L1 | Ethernet native, fast response (5ms), large register space, web diagnostics |
| `slc-500-05` | PLC | Manufacturing | Serial only (DH+/RS-485), 25ms response, limited registers (256), programmed via RSLogix 500 on Windows XP |
| `modicon-984` | PLC | Manufacturing | Serial only (RS-485), 50ms response, original Modbus device, 999 registers, DIP-switch configured |
| `moxa-nport-5150` | Gateway | Manufacturing | Not a PLC but critical infrastructure; converts Modbus RTU to TCP, adds 5-15ms latency, single connection at a time, default web credentials |

Future device atoms (not in v0.1 but anticipated):
- `cradlepoint-ibr600` -- Cellular modem/router (per ADR-006)
- `wonderware-intouch-9.5` -- Windows XP HMI
- Safety controllers (per ADR-008)

### D5: Device Atoms Affect What Monitoring Can See

**Decision**: Each device atom defines what telemetry is observable from the network, constraining what the monitoring layer can discover. This is determined by the device's connectivity, diagnostics, and (in future versions) security profile.

| Device Atom | Discoverable via network scan | Modbus readable | Other telemetry |
|-------------|------------------------------|-----------------|-----------------|
| CompactLogix | Yes (responds to ICMP, HTTP on port 80) | Yes (Modbus TCP) | Web server (device info, fault log) |
| SLC-500 | Only via Moxa gateway (appears as gateway's IP) | Yes (via gateway, Modbus TCP/RTU bridge) | None -- serial device invisible to network |
| Modicon 984 | Only via Moxa gateway | Yes (via gateway) | None |
| Moxa NPort | Yes (responds to ICMP, HTTP on port 80, Telnet on port 23) | Transparent bridge | Web management, Telnet management (often with default credentials) |

**Teaching value**: IT engineers learn that network scanning the manufacturing floor reveals the Moxa gateway and the Windows XP HMI, but NOT the PLCs behind the gateway. The PLCs are invisible to standard IT discovery tools because they are on a serial bus, not IP.

### D6: Register Maps Are Device-Specific but Variant-Capable

**Decision**: Each device atom defines its base register map. Environment templates can specify a `register_map_variant` to load a role-specific register layout for a given placement.

Example: The `compactlogix-l33er` device atom defines register capabilities (max counts, byte order). The environment template assigns `register_map_variant: "water-intake"` to one placement and `register_map_variant: "water-distribution"` to another. Both use the same device atom but expose different process values through their registers.

Register map variants are defined within the device atom file or in a companion file in the device library. This allows the same device type to serve different roles without duplicating the entire device definition.

---

## Consequences

### Positive
- Adding new device types is a configuration change, not a code change
- Engineers learn that different PLCs behave differently even when speaking the same protocol
- Discovery constraints force monitoring tools to work within realistic limitations
- Device atoms can be shared and contributed by other engineers as the library grows
- Schema versioning allows gradual enrichment (security profiles, physical assets) without breaking existing atoms
- The visibility hierarchy teaches engineers what security tools can and cannot see

### Negative
- More configuration complexity upfront
- Must validate that device atom properties are realistic (response times, register limits)
- No vendor-proprietary protocol simulation (per ADR-002 D4)
- Schema evolution requires careful versioning

### Risks
- **Risk**: Device atom YAML schema becomes too complex as layers are added
- **Mitigation**: Layers are additive and optional. A v0.1 device atom with only identity, connectivity, and registers is fully valid even when the schema supports v0.3 features. Keep atoms focused on observable differences.
- **Risk**: Register map variants may lead to proliferation of similar definitions
- **Mitigation**: Variants share the device atom's base constraints. Only the register assignments differ. Consider a register map template mechanism if variants proliferate.

---

## References
- ADR-009: Design Layer and Composable Environments
- Allen-Bradley SLC-500 Instruction Set Reference Manual (1747-RM001)
- Modicon 984 User Manual (840 USE 101)
- Moxa NPort 5100 Series User Manual
- Rockwell Automation CompactLogix 5380 System Reference Manual
