# Admin CLI Reference

The `admin` binary provides command-line administration for the OT Simulator platform: health
checks, event database management, monitoring configuration inspection, design layer validation,
and anomaly detection baseline management.

> **Binary name**: The current binary is named `admin`. A unified `otsim` wrapper is planned for a
> future release (td-admin-075). All examples in this document use `admin`.

**Audience**: IT engineers operating the OT Simulator for security testing and integration. No Go
knowledge required. OT-specific terms are explained in context where they first appear.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Quick Start](#2-quick-start)
3. [Global Flags](#3-global-flags)
4. [Commands Reference](#4-commands-reference)
5. [Environment Variables](#5-environment-variables)
6. [Exit Codes](#6-exit-codes)
7. [Troubleshooting](#7-troubleshooting)

---

## 1. Overview

The `admin` CLI reads the design layer (`design/` YAML files), the monitoring configuration
(`config/monitor.yaml`), and the event database (`data/events.db`). It does not replace the plant
or monitoring services -- it observes them externally over the network and filesystem.

**Build**: `cd admin && go build -o admin ./cmd/admin/`

A running Go toolchain (1.21+) is required for the build. All examples assume `./admin` is run
from the `admin/` directory.

---

## 2. Quick Start

### Step 1: Build

    $ cd admin && go build -o admin ./cmd/admin/

### Step 2: Check platform health

    $ ./admin health

    Service       Status   Details
    ------------  -------  ----------------------------
    PLC 5020      online   3ms
    PLC 5021      online   2ms
    PLC 5022      online   4ms
    Gateway 5030  online   5ms
    ...
    Monitor API   online   healthy, 14 devices online
    Event DB      online   12.4 MB, data/events.db

Exit 0 = all services online. Exit 1 = any service offline.

### Step 3: View monitoring configuration

    $ ./admin config view

    Configuration: ./config/monitor.yaml

      log_level:                    info
      poll_interval_seconds:        2
      baseline_learning_cycles:     150
      event_db_path:                data/events.db
      event_retention_days:         7
      ...

      Environments (1):
        greenfield-water-mfg (localhost): 6 endpoint(s)
          port 5020  PLC      unit_id=1    Water Treatment PLC 1 - Intake
          ...

### Step 4: Validate a device atom

    $ ./admin design validate design/devices/compactlogix-l33er.yaml
    design/devices/compactlogix-l33er.yaml: OK

### Step 5: Validate an entire environment

    $ ./admin design validate design/environments/greenfield-water-mfg/

    Validating environment: greenfield-water-mfg

      environment.yaml                 OK (schema)
      process.yaml                     OK (schema)
      Cross-references                 OK
      Device: compactlogix-l33er       OK (schema)
      Device: modicon-984              OK (schema)
      Device: moxa-nport-5150          OK (schema)
      Device: slc-500-05               OK (schema)

      Result: 7/7 passed, 0 errors

### Step 6: List all design elements

    $ ./admin design list

    Design Layer Elements (./design)

    Devices (7):
    ID                    Vendor              Model               Category
    --------------------  ------------------  ------------------  --------
    compactlogix-l33er    Allen-Bradley       CompactLogix L33ER  PLC
    modicon-984           Schneider Electric  Modicon 984         PLC
    moxa-nport-5150       Moxa                NPort 5150          Gateway
    ...

    Networks (13):
    ID               Type          Subnet
    ---------------  ------------  -----------------
    cross-plant      ethernet      172.16.0.0/30
    mfg-flat         ethernet      192.168.1.0/24
    mfg-serial-bus   serial-rs485  -
    ...

    Environments (4):
    ID                        Devices  Networks  Has Process
    ------------------------  -------  --------  -----------
    greenfield-water-mfg      5        6         yes
    quickstart-example        2        2         no
    ...

### Step 7: Check baseline status

    $ ./admin baseline status

    Device ID       Status    Samples  Required  Registers
    --------------  --------  -------  --------  ---------
    mfg-gateway-01  learned   150      150       4
    wt-plc-01       learned   150      150       12
    wt-plc-03       learning  87       150       14

`learned` = anomaly detection active. `learning` = still collecting the normal-traffic profile.

### Step 8: Export events for analysis

    $ ./admin db export --format csv --output events.csv
    Exported 4821 events to events.csv.

Events are Modbus transaction records (source/destination, function code, register address,
response time). Suitable for SIEM import or manual forensic analysis.

### Step 9: Start the admin dashboard

    $ ./admin web
    Admin dashboard listening on http://localhost:8095

---

## 3. Global Flags

Global flags precede the command name. Precedence: **CLI flag > environment variable > default**.

| Flag | Default | Env Variable | Description |
|------|---------|-------------|-------------|
| `--design-dir PATH` | `./design` | `OTS_DESIGN_DIR` | Design layer directory |
| `--config PATH` | `./config/monitor.yaml` | `OTS_MONITOR_CONFIG` | Monitoring config file |
| `--db PATH` | *(from config)* | | Override event database path |
| `--api-addr ADDR` | `localhost:8091` | `OTS_API_ADDR` | Monitoring API address |
| `--plant-ports LIST` | `5020,5021,...` (14 ports) | `OTS_HEALTH_PORTS` | Comma-separated Modbus TCP ports for health checks |

    # These three are equivalent:
    $ ./admin --design-dir /opt/ot/design health
    $ OTS_DESIGN_DIR=/opt/ot/design ./admin health
    $ ./admin health  # uses default ./design

The default `--plant-ports` list: `5020,5021,5022,5030,5040,5041,5042,5043,5050,5051,5052,5062,5063,5064`.

---

## 4. Commands Reference

### 4.1 admin health

**Synopsis**

    admin [global-flags] health

**Description**

Probes all configured plant ports (Modbus TCP), the monitoring API, and the event database.
Ports are probed concurrently with a 2-second TCP timeout. Use as a first-line check before
beginning a training scenario.

A PLC (Programmable Logic Controller) is a physical device that controls industrial processes.
In the simulator, each PLC is a listening Modbus TCP port.

**Examples**

    $ ./admin health

    Service       Status   Details
    ------------  -------  ----------------------------
    PLC 5020      online   3ms
    PLC 5021      offline  TCP connection refused
    Gateway 5030  online   5ms
    Monitor API   online   healthy, 5 devices online
    Event DB      online   8.2 MB, data/events.db

Service type labels by port range: 5020-5029 = PLC, 5030-5039 = Gateway, 5040-5049 = PLC,
5050-5059 = PLC, 5060-5065 = Modem.

**Exit Codes** | 0 = all services online | 1 = any service offline

**Notes**: A degraded plant is not healthy. If 13 of 14 PLCs respond, exit code is still 1.
One offline PLC may mean loss of control for an entire process stage.

---

### 4.2 admin db status

**Synopsis**

    admin [global-flags] db status

**Description**

Prints event store statistics: total event count, file size, timestamp range, retention window,
and pruneable event count.

**Example**

    $ ./admin db status

      Event count:              4821
      Database size:            12.34 MB
      Oldest event:             2026-03-01T08:00:00Z
      Newest event:             2026-03-03T14:22:11Z
      Retention window:         7 days
      Pruneable events:         0 (older than 7 days)
      Database path:            data/events.db

When the database is empty, `Oldest event` and `Newest event` show `none (empty database)`.

**Exit Codes** | 0 = success | 1 = database not found or unreadable

---

### 4.3 admin db validate

**Synopsis**

    admin [global-flags] db validate

**Description**

Runs SQLite integrity and foreign key checks. Use before exporting events for SIEM import.

**Examples**

    $ ./admin db validate
    Database integrity: OK
    Foreign key check: OK

    $ ./admin db validate
    Database integrity: FAILED
      error: row 4 missing from index
    Foreign key check: OK

**Exit Codes** | 0 = all checks passed | 1 = any check failed or database unreadable

---

### 4.4 admin db prune

**Synopsis**

    admin [global-flags] db prune [--older-than N] [--force]

**Description**

Deletes events older than N days. Without `--older-than`, uses `event_retention_days` from the
monitoring config (default: 7). Without `--force`, shows a confirmation prompt.

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--older-than N` | config value | Delete events older than N days |
| `--force` | false | Skip confirmation prompt |

**Examples**

    $ ./admin db prune
    WARNING: This will delete all events older than 2026-02-24T14:22:11Z (7 days).
    Type 'yes' to confirm: yes
    Deleted 1203 events older than 7 days.

    $ ./admin db prune --older-than 30 --force
    Deleted 8742 events older than 30 days.

**Exit Codes** | 0 = pruned (or aborted) | 1 = database error

---

### 4.5 admin db export

**Synopsis**

    admin [global-flags] db export [--format csv|json] [--output PATH]
                                   [--device ID] [--after RFC3339] [--before RFC3339]

**Description**

Exports Modbus event records as CSV or JSON. Each record is one Modbus transaction: source and
destination addresses, function code (the operation type, e.g., ReadHoldingRegisters), register
address range, and any values written. Without `--output`, data goes to stdout.

Export limit: 100,000 rows. If reached, a warning is printed to stderr and export stops.
Use `--before`/`--after` to narrow the time range.

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--format csv\|json` | `csv` | Output format |
| `--output PATH` | stdout | Write to file |
| `--device ID` | all | Filter by device placement ID |
| `--after RFC3339` | none | Include events at or after this timestamp |
| `--before RFC3339` | none | Include events before this timestamp |

**Examples**

CSV to file:

    $ ./admin db export --format csv --output events.csv
    Exported 4821 events to events.csv.

CSV columns: `id, timestamp, src_addr, dst_addr, unit_id, func_code, func_name, addr_start,
addr_count, is_write, success, exception, response_us, device_id, device_name, env_id, write_values`

JSON to file:

    $ ./admin db export --format json --output events.json
    Exported 4821 events to events.json.

JSON output is an indented array of objects with the same fields as the CSV columns.

Filtered export:

    $ ./admin db export --device wt-plc-01 --after 2026-03-03T08:00:00Z --before 2026-03-03T09:00:00Z --output wt-plc-01-morning.csv
    Exported 1800 events to wt-plc-01-morning.csv.

Limit warning:

    $ ./admin db export --output all.csv
    WARNING: Export limit reached (100000 rows). Use --before/--after to narrow the range.

**Exit Codes** | 0 = success (limit warning does not change exit code) | 1 = database error or bad flags

---

### 4.6 admin config view

**Synopsis**

    admin [global-flags] config view [PATH]

**Description**

Parses the monitoring config and prints a formatted summary with defaults applied. The optional
positional `PATH` argument overrides `--config` for this command only. Uses lenient parsing --
displays the config even if validation errors are present.

`poll_interval_seconds` controls how often PLCs are polled. `baseline_learning_cycles` is the
number of polling cycles required to establish a traffic baseline (default: 150 cycles = 5 minutes
at 2s poll interval). `gateway_request_delay_ms` adds inter-frame spacing for serial gateways.

**Example**

    $ ./admin config view

    Configuration: ./config/monitor.yaml

      log_level:                    info
      poll_interval_seconds:        2
      gateway_request_delay_ms:     10
      api_addr:                     :8091
      dashboard_addr:               :8090
      baseline_learning_cycles:     150
      ring_buffer_size:             300
      max_alerts:                   1000
      event_db_path:                data/events.db
      event_retention_days:         7

      Syslog:
        enabled:                    false

      Environments (1):
        greenfield-water-mfg (localhost): 6 endpoint(s)
          port 5020  PLC      unit_id=1    Water Treatment PLC 1 - Intake
          port 5021  PLC      unit_id=1    Water Treatment PLC 2 - Treatment
          port 5022  PLC      unit_id=1    Water Treatment PLC 3 - Distribution
          port 5030  Gateway  unit_id=0    Serial-to-Ethernet Gateway
          port 5040  PLC      unit_id=1    Power Substation PLC
          port 5050  PLC      unit_id=1    Wastewater PLC

**Exit Codes** | 0 = success | 1 = file not found or YAML parse error

---

### 4.7 admin config validate

**Synopsis**

    admin [global-flags] config validate [PATH]

**Description**

Validates the monitoring config against all required fields and value constraints. Run after
editing `monitor.yaml` before restarting the monitoring service.

**Examples**

    $ ./admin config validate
    Configuration is valid.

    $ ./admin config validate
    Validation failed: invalid config "./config/monitor.yaml": at least one environment must be defined

    $ ./admin config validate
    Validation failed: invalid config "./config/monitor.yaml": poll_interval_seconds must be >= 1, got 0

**Exit Codes** | 0 = valid | 1 = validation failure, file not found, or parse error

---

### 4.8 admin design validate

**Synopsis**

    admin [global-flags] design validate <path> [--verbose] [--cross-refs-only]

**Description**

Validates design layer YAML against JSON Schemas and checks cross-file references. Accepts a
single file or an environment directory. Schema type is inferred from the file's location within
the design directory.

Cross-reference checks (directory mode) validate: device and network files exist, placement
networks are declared in the environment, gateway references resolve to valid placement IDs,
register map variants exist in the device atom, and register addresses are within device bounds.

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--verbose` | false | Show schema descriptions alongside each error |
| `--cross-refs-only` | false | Skip schema validation; check only cross-references (directory mode only) |

**Examples**

Single file, passing:

    $ ./admin design validate design/devices/compactlogix-l33er.yaml
    design/devices/compactlogix-l33er.yaml: OK

Single file, failing:

    $ ./admin design validate design/devices/broken-device.yaml
        design/devices/broken-device.yaml:4: /device/vendor: value is required

With --verbose:

    $ ./admin design validate design/devices/broken-device.yaml --verbose
        design/devices/broken-device.yaml:4: /device/vendor: value is required
          expected: Manufacturer name (e.g., Allen-Bradley, Siemens, Modicon)

Environment directory, passing:

    $ ./admin design validate design/environments/greenfield-water-mfg/

    Validating environment: greenfield-water-mfg

      environment.yaml                 OK (schema)
      process.yaml                     OK (schema)
      Cross-references                 OK
      Device: compactlogix-l33er       OK (schema)
      Device: modicon-984              OK (schema)
      Device: moxa-nport-5150          OK (schema)
      Device: slc-500-05               OK (schema)

      Result: 7/7 passed, 0 errors

Environment directory with a cross-reference error:

    $ ./admin design validate design/environments/example-broken/

    Validating environment: example-broken

      environment.yaml                 OK (schema)
      Cross-references                 FAIL
        design/environments/example-broken/environment.yaml:31: placement "wt-plc-02": network "wt-level2" is not listed in environment networks
      Device: compactlogix-l33er       OK (schema)

      Result: 2/3 passed, 1 errors

Cross-references only:

    $ ./admin design validate design/environments/greenfield-water-mfg/ --cross-refs-only

    Validating environment: greenfield-water-mfg

      Cross-references                 OK

      Result: 1/1 passed, 0 errors

**Exit Codes** | 0 = all validations passed | 1 = any error, file not found, or schema load failure

**Notes**: Files outside the design directory produce: `cannot determine schema type for <path>:
file is not in design/devices/, design/networks/, or design/environments/`.

---

### 4.9 admin design list

**Synopsis**

    admin [global-flags] design list

**Description**

Lists all device atoms, network atoms, and environment definitions in three formatted tables.
Serial-type networks (RS-485, RS-232 buses) show `-` in the Subnet column.

**Example**

    $ ./admin design list

    Design Layer Elements (./design)

    Devices (7):
    ID                    Vendor              Model               Category
    --------------------  ------------------  ------------------  --------
    compactlogix-l33er    Allen-Bradley       CompactLogix L33ER  PLC
    modicon-984           Schneider Electric  Modicon 984         PLC
    moxa-nport-5150       Moxa                NPort 5150          Gateway
    slc-500-05            Allen-Bradley       SLC 500/05          PLC

    Networks (13):
    ID               Type          Subnet
    ---------------  ------------  ----------------
    cross-plant      ethernet      172.16.0.0/30
    mfg-flat         ethernet      192.168.1.0/24
    mfg-serial-bus   serial-rs485  -

    Environments (4):
    ID                        Devices  Networks  Has Process
    ------------------------  -------  --------  -----------
    greenfield-water-mfg      5        6         yes
    quickstart-example        2        2         no

**Exit Codes** | 0 = success | 1 = design directory unreadable

---

### 4.10 admin baseline status

**Synopsis**

    admin [global-flags] baseline status

**Description**

Displays per-device baseline learning state from the monitoring service. A baseline is the
learned normal-traffic profile for a device: which registers are read, at what cadence, with what
values. The anomaly detector compares live traffic against this baseline to generate alerts.

Requires the monitoring service to be running.

**Examples**

    $ ./admin baseline status

    Device ID       Status    Samples  Required  Registers
    --------------  --------  -------  --------  ---------
    mfg-gateway-01  learned   150      150       4
    wt-plc-01       learned   150      150       12
    wt-plc-02       learning  87       150       14
    wt-plc-03       learning  23       150       14

No data yet:

    $ ./admin baseline status
    No baseline data available. Monitoring may not have started polling yet.

**Exit Codes** | 0 = status retrieved (including "no data" message) | 1 = API unreachable

**Notes**: Required samples = `baseline_learning_cycles` from config (default: 150). At 2s poll
interval, 150 cycles takes 5 minutes per device.

---

### 4.11 admin baseline reset

**Synopsis**

    admin [global-flags] baseline reset [--device ID] [--force]

**Description**

Triggers baseline re-learning. During the re-learning period, anomaly detection alerts are
suppressed.

> **OT Warning**: Baseline reset triggers a learning period during which anomaly detection is
> suspended. Any traffic observed during this window -- including malicious activity -- will be
> accepted as normal behavior and incorporated into the new baseline. In real OT environments,
> this is operationally equivalent to temporarily disabling the IDS. The `--force` flag bypasses
> the confirmation prompt but not the consequences.

Single-device reset (`--device`) proceeds without confirmation. Full reset requires `--force`
or interactive confirmation.

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--device ID` | all | Reset baseline for one device only |
| `--force` | false | Skip confirmation for full reset |

**Examples**

Single device:

    $ ./admin baseline reset --device wt-plc-03
    Baseline reset for device "wt-plc-03": reset accepted, learning started

Full reset with prompt:

    $ ./admin baseline reset
    WARNING: Baseline reset triggers a learning period. Anomaly detection alerts
             are suppressed during this window. This is operationally equivalent
             to disabling the IDS for all monitored devices.
    Type 'yes' to confirm full baseline reset: yes
    Full baseline reset: reset accepted (6 devices)

Full reset with --force:

    $ ./admin baseline reset --force
    Full baseline reset: reset accepted (6 devices)

**Exit Codes** | 0 = reset accepted (or aborted) | 1 = API unreachable or request rejected

---

### 4.12 admin web

**Synopsis**

    admin [global-flags] web [--addr ADDR]

**Description**

Starts the admin web dashboard. The dashboard provides browser-based access to the same
information as the CLI commands, with real-time HTMX auto-refresh. The server blocks until
interrupted with Ctrl-C.

The admin dashboard (port 8095) is separate from the monitoring dashboard (port 8090). The
monitoring dashboard shows real-time Modbus traffic and alerts; the admin dashboard provides
platform management.

**Flags**

| Flag | Default | Env Variable | Description |
|------|---------|-------------|-------------|
| `--addr ADDR` | `:8095` | `OTS_ADMIN_ADDR` | HTTP listen address |

**Examples**

    $ ./admin web
    Admin dashboard listening on http://localhost:8095

    $ ./admin web --addr :9000
    Admin dashboard listening on http://localhost:9000

**Exit Codes** | 0 = clean exit | 1 = bind failure (port in use) or runtime error

---

## 5. Environment Variables

All `OTS_` variables follow CLI flag > environment variable > default precedence.

| Variable | Default | Used By | Description |
|----------|---------|---------|-------------|
| `OTS_DESIGN_DIR` | `./design` | All commands | Path to design layer directory |
| `OTS_MONITOR_CONFIG` | `./config/monitor.yaml` | db, config, baseline, web | Monitoring config file path |
| `OTS_API_ADDR` | `localhost:8091` | health, baseline, web | Monitoring API address |
| `OTS_ADMIN_ADDR` | `:8095` | web | Admin dashboard listen address |
| `OTS_HEALTH_PORTS` | `5020,...,5064` (14 ports) | health | Comma-separated Modbus TCP ports to probe |

    export OTS_DESIGN_DIR=/opt/ot-sim/design
    export OTS_MONITOR_CONFIG=/etc/ot-sim/monitor.yaml
    export OTS_API_ADDR=10.0.1.5:8091

---

## 6. Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All operations succeeded |
| 1 | Any operation failed |

**OT-aware failure definition**: The CLI treats a degraded condition as failure.

- `admin health`: One offline PLC = exit 1, even if all other services respond. A single offline
  PLC may mean loss of automated control for an entire process stage (e.g., water distribution).
- `admin db validate`: Any integrity violation = exit 1.
- `admin design validate`: Any schema or cross-reference error = exit 1.
- `admin baseline reset`: If the API is unreachable, exit 1 (the reset was never applied).

This convention supports scripting:

    ./admin health && ./admin db validate && echo "Platform ready"

---

## 7. Troubleshooting

### All ports offline

**Symptom**: Every row shows `offline  TCP connection refused` in `admin health`.

**Cause**: The plant simulator is not running, or ports do not match the default list.

**Resolution**: Start the plant: `cd plant && ./plant`. If using a non-default environment,
set the correct port list: `./admin --plant-ports 5020,5021,5022 health`.

---

### Database not found

**Symptom**: `admin db status: stat database file "data/events.db": no such file or directory`

**Cause**: The monitoring service has not created the database yet, or the path is wrong.

**Resolution**: Start the monitoring service and wait for one polling cycle. Check the configured
path: `./admin config view | grep event_db_path`. Override: `./admin --db /path/events.db db status`.

---

### Cannot read config file

**Symptom**: `Validation failed: reading config file "./config/monitor.yaml": no such file or directory`

**Cause**: Config file is not at the expected path.

**Resolution**: `./admin --config /path/to/monitor.yaml config validate` or set
`OTS_MONITOR_CONFIG=/path/to/monitor.yaml`.

---

### Cannot determine schema type

**Symptom**: `admin design validate: cannot determine schema type for <path>: file is not in design/devices/, design/networks/, or design/environments/`

**Cause**: The file being validated is outside the design directory tree, so the schema cannot be inferred.

**Resolution**: Ensure the file is under the correct design subdirectory. If running from a
different directory, use absolute paths and set `--design-dir` explicitly:
`./admin --design-dir /opt/ot-sim/design design validate /opt/ot-sim/design/devices/my-plc.yaml`.

---

### Monitoring API unreachable

**Symptom**: `admin baseline status: monitoring API unreachable at localhost:8091: <connection error>`

**Cause**: The monitoring service is not running or is on a different address.

**Resolution**: Start the monitoring service: `cd monitoring && ./monitor`. Check the configured
address: `./admin config view | grep api_addr`. Override: `./admin --api-addr localhost:9091 baseline status`.

---

### Admin web: address already in use

**Symptom**: `admin web: server error: listen tcp :8095: bind: address already in use`

**Cause**: Another process is already on port 8095.

**Resolution**: `lsof -ti :8095 | xargs kill` or use a different port:
`./admin web --addr :9095`.
