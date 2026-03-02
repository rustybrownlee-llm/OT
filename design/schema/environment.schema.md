# Environment YAML Schema Reference

Version: `0.1`
References: ADR-009 D5 (environment schema), ADR-010 D1/D3/D4 (archetype extensions)

## Top-Level Structure

```yaml
schema_version: "0.1"       # required
environment: { ... }        # required
networks: [ ... ]           # required; may be empty list
placements: [ ... ]         # required; may be empty list
boundaries: [ ... ]         # optional (ADR-010 D4)
```

---

## `environment` Block

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Unique identifier in kebab-case. Must match the directory name. |
| `name` | string | no | Human-readable display name for HMI and scenario descriptions. |
| `description` | string | no | Brief description of the facility and its training purpose. |
| `archetype` | string | no | Topology classification. See valid values below. (ADR-010 D1) |
| `era_span` | string | no | Installation era: `"YYYY"` or `"YYYY-YYYY"`. (ADR-010 D3) |

### `archetype` Valid Values

| Value | Meaning |
|-------|---------|
| `modern-segmented` | Purdue-model facility with VLANs, managed switches, and distinct network levels. |
| `legacy-flat` | Single flat network with no segmentation between device types. |
| `hybrid` | Mixed topology -- some segments have boundary controls, others do not. |

Validation rule ENV-020: invalid values produce an error.

### `era_span` Format

- Single year: `"2024"` -- the facility was built in this year.
- Range: `"1997-2022"` -- the facility was built or expanded across this period.
- Start year must be less than or equal to end year (ENV-021b).
- Invalid formats produce a warning (ENV-021), not an error.

---

## `networks` Block

A list of network references. Each `ref` must match a filename in `design/networks/`
(without the `.yaml` extension). Serial buses are networks; list them here even though
they have no IP subnet.

```yaml
networks:
  - ref: "wt-level1"
  - ref: "mfg-serial-bus"
```

Validation rule ENV-003: each `ref` must resolve to a file in `design/networks/`.

---

## `placements` Block

Each placement wires a device atom onto a network with specific addressing.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Unique placement identifier within this environment. |
| `device` | string | yes | Device atom reference (filename in `design/devices/` without `.yaml`). |
| `network` | string | yes | Network ref; must be listed in the `networks` block. |
| `ip` | string | conditional | IPv4 address within the network subnet. Required for Ethernet placements. |
| `modbus_port` | int | conditional | Modbus TCP listener port (range 5020-5039). Required for Ethernet Modbus TCP devices. |
| `serial_address` | int | conditional | Modbus RTU address (1-247). Required for serial placements. |
| `gateway` | string | conditional | Placement ID of the gateway bridging this serial device to Ethernet. Required for serial placements. |
| `role` | string | no | Human-readable role description for training context. |
| `register_map_variant` | string | yes | Selects the register map layout. Must match a key in the device's `register_map_variants` section. |
| `bridges` | list | conditional | Required for `gateway`-category devices. Lists serial networks this gateway bridges to. |
| `additional_networks` | list | no | Additional network attachments for multi-homed devices (e.g., cross-plant links). |
| `installed` | int | no | Year this device was first commissioned in this facility. Valid range: 1960 to current year + 2. (ADR-010 D3) |

### `installed` on placements

Validation rule ENV-022: the year must be in the range `[1960, current_year + 2]`.
The lower bound 1960 accommodates early PLC history (Modicon 084 shipped 1968).
The upper bound `+2` accommodates staged equipment ordered but not yet operational.

### `bridges` Entry

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `from_network` | string | yes | The Ethernet side (the network the gateway is on). |
| `to_network` | string | yes | The serial side (the RS-485 bus legacy devices are on). |

Validation rule ENV-012: `from_network` must be the Ethernet side and `to_network` must be
the serial side.

### `additional_networks` Entry

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `network` | string | yes | Network ref; must be listed in the `networks` block. |
| `ip` | string | yes | IPv4 address within the network subnet. |

---

## `boundaries` Block (ADR-010 D4)

Optional section documenting network segmentation state. Required when `archetype` is
`"hybrid"` (ENV-026 warning if absent). Supported for all archetypes.

Each entry describes the state of the boundary between exactly two networks.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `between` | list of 2 strings | yes | Network refs on each side of the boundary. Must each be listed in the `networks` block. |
| `state` | string | yes | Current enforcement state. See valid values below. |
| `infrastructure` | string | no | Type of boundary enforcement device. See valid values below. |
| `installed` | int | no | Year this boundary was commissioned. Same valid range as placement `installed`. |
| `notes` | string | no | Human-readable context for training scenarios. |

### `state` Valid Values

| Value | Meaning |
|-------|---------|
| `enforced` | Boundary device is present and actively filtering traffic. |
| `intended` | Boundary is in the network design but not yet enforced in hardware. |
| `absent` | No segmentation exists between these networks (flat topology). |

Validation rule ENV-025: invalid values produce an error.
Validation rule ENV-027: `modern-segmented` environments with an `absent` boundary receive
a warning suggesting reclassification as `"hybrid"`.

### `infrastructure` Valid Values

| Value | Meaning |
|-------|---------|
| `managed-switch` | Layer 2/3 managed switch with ACLs or port isolation. |
| `firewall` | Stateful firewall or next-generation firewall. |
| `ids-sensor` | Passive IDS/IPS sensor (monitoring, not enforcing). |
| `vlan-only` | VLAN segmentation without a separate firewall device. |
| `other` | Non-standard boundary control. Use specific values above when possible. |

Validation rule ENV-029: invalid values produce an error.
Validation rule ENV-030: `"other"` produces a warning recommending a more specific value.

---

## Validation Rules Summary

| Rule | Severity | Description |
|------|----------|-------------|
| ENV-001 | error | `schema_version` must be `"0.1"`. |
| ENV-002 | error | `environment.id` is required. |
| ENV-003 | error | Each network `ref` must resolve to a file in `design/networks/`. |
| ENV-004 | error | Each placement `device` must resolve to a file in `design/devices/`. |
| ENV-005 | error | `register_map_variant` must exist in the device's `register_map_variants`. |
| ENV-006 | error | Placement IDs must be unique within the environment. |
| ENV-007 | error | Each placement `network` must be listed in the environment `networks` block. |
| ENV-008 | error | Placement `ip` must be within the network's subnet. |
| ENV-009 | error | No two placements on the same network may share an IP address. |
| ENV-010 | error | No two placements may share a `modbus_port`. |
| ENV-011 | error | A placement's `gateway` must reference a valid placement ID in this environment. |
| ENV-012 | error | The gateway's `bridges` must include a `to_network` matching the serial device's network. |
| ENV-013 | error | Serial addresses must be unique per gateway. |
| ENV-014 | error | Each `additional_networks` entry `network` must be listed in the environment `networks` block. |
| ENV-015 | error | `additional_networks` `ip` must be within the additional network's subnet. |
| ENV-016 | error | `gateway`-category devices must declare a `bridges` section. |
| ENV-017 | error | Serial devices accessed via gateway must not have `ip` or `modbus_port` fields. |
| ENV-018 | error | Device port types must match the network type (e.g., Ethernet device on Ethernet network). |
| ENV-019 | error | `serial_address` must be in the range 1-247. |
| ENV-020 | error | `environment.archetype` must be one of: `modern-segmented`, `legacy-flat`, `hybrid`. |
| ENV-021 | warning | `environment.era_span` must match format `YYYY` or `YYYY-YYYY`. |
| ENV-021b | error | In a range `era_span`, the start year must be less than or equal to the end year. |
| ENV-022 | error | Placement `installed` year must be in range `[1960, current_year + 2]`. |
| ENV-023 | error | Boundary `between` must list exactly 2 network refs. |
| ENV-024 | error | Each network in boundary `between` must be listed in the environment `networks` block. |
| ENV-025 | error | Boundary `state` must be one of: `enforced`, `intended`, `absent`. |
| ENV-026 | warning | `hybrid` archetype without a `boundaries` section. |
| ENV-027 | warning | `modern-segmented` archetype with an `absent` boundary; consider reclassifying as `hybrid`. |
| ENV-028 | error | Boundary `installed` year must be in range `[1960, current_year + 2]`. |
| ENV-029 | error | Boundary `infrastructure` must be one of: `managed-switch`, `firewall`, `ids-sensor`, `vlan-only`, `other`. |
| ENV-030 | warning | Boundary `infrastructure: "other"` is imprecise; a more specific value is preferred. |
