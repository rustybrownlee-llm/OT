# Milestone: Beta 0.7

**Target**: Build platform administration tools so that non-developer engineers and integration partners can operate, validate, and configure the OT simulator without touching Go source code, SQL queries, or raw YAML by trial-and-error. An engineer should be able to check platform health, manage the event database, validate and edit design layer YAML with schema guidance, and configure monitoring -- all from a web browser or documented CLI commands.

**Implements**: New ADR (Platform Administration) -- to be authored if architectural decisions arise during SOW drafting

**Status**: Planning

## Definition of Done

An engineer can:

1. Run `otsim health` from the terminal and see a clear summary of which services are running, which ports are active, and whether devices are reachable -- without knowing port numbers or API endpoints
2. Run `otsim db status` and see event store statistics (row count, DB size, oldest/newest event, retention window) without writing SQL
3. Run `otsim db export --format csv` and get a CSV file of transaction events for offline analysis in Excel or a SIEM import
4. Run `otsim config validate monitor.yaml` and see whether the monitoring configuration is valid, with clear error messages for every invalid field
5. Run `otsim design validate design/devices/compactlogix-l33er.yaml` and see whether a device atom conforms to the JSON Schema, with line-level error reporting
6. Run `otsim design validate design/environments/greenfield-water-mfg/` and validate an entire environment (environment.yaml, process.yaml, all referenced device and network atoms) in one command
7. Open `http://localhost:8095` and see an admin dashboard with service health, event DB statistics, and navigation to all admin functions
8. Open the YAML editor in the admin dashboard, browse the `design/` directory tree, select a device atom, and see it rendered with syntax highlighting in a code editor (CodeMirror 6)
9. Edit a device atom YAML in the browser, click "Validate", and see schema validation errors highlighted inline with descriptions of what's wrong and what's expected
10. Save YAML edits from the browser back to disk, with the editor preventing saves that fail schema validation
11. View and edit `monitor.yaml` from the admin dashboard with the same schema-validated editor experience
12. See JSON Schema files in `design/schemas/` that formally document the expected structure of every design layer YAML type (device atom, network atom, environment, process)
13. Read CLI reference documentation that explains every command, its flags, examples, and expected output -- written for engineers without Go experience
14. Hand the platform to a partner engineer (e.g., Blastwave) with the admin dashboard URL and CLI docs, and have them operate it independently

## What Beta 0.7 Is NOT

- No RBAC, authentication, or authorization (single-user training deployments; no security risk from full admin access)
- No plant management or control (plant is started separately; admin health-checks it but does not start/stop/configure it)
- No Modbus write capability from admin tools (observation and configuration only)
- No alert management from admin (alerts are managed in the monitoring dashboard at :8090)
- No scenario management or authoring tools (scenarios are markdown, authored in an editor)
- No multi-instance federation (single platform deployment)
- No real-time log streaming in admin dashboard (health checks are point-in-time; live logs stay in terminal)
- No database migration framework (append-only schema, 7-day retention makes migrations unnecessary)
- No Cisco CyberVision, Dragos, or BlastShield integration (commercial tools deferred to Beta 0.8+)
- No new simulation capabilities or protocol support (admin tools operate on the existing platform)

## Design Layer JSON Schemas

### Schema Strategy

JSON Schema (Draft 2020-12) for each design layer YAML type. Schemas live in `design/schemas/` alongside the YAML they validate. The admin CLI and web editor both consume these schemas.

Why JSON Schema:
- Industry standard with broad tooling support (CodeMirror, VS Code, ajv, Go validators)
- Documents the design layer contract independently of Go struct definitions
- Partner engineers already familiar with JSON Schema from API specifications
- Can generate documentation from schema definitions

### Schema Files

| File | Validates | Key Constraints |
|------|-----------|-----------------|
| `design/schemas/device-atom.schema.json` | `design/devices/*.yaml` | schema_version, device metadata, connectivity ports/protocols, register capacity with addressing mode, register map variants with typed registers, diagnostics |
| `design/schemas/network-atom.schema.json` | `design/networks/*.yaml` | schema_version, network metadata, type enum (ethernet/serial-rs485/serial-rs232), properties conditional on type (subnet for ethernet, no subnet for serial) |
| `design/schemas/environment.schema.json` | `design/environments/*/environment.yaml` | schema_version, environment metadata, network refs, placements with device/network/IP/port validation, optional boundaries |
| `design/schemas/process.schema.json` | `design/environments/*/process.yaml` | schema_version, process metadata with flow direction, stages with controllers/equipment/instruments, ISA-5.1 tag format, connections |

### Schema Scope

Schemas validate structure, types, required fields, enum values, and format constraints (IP addresses, port ranges, register address ranges). They do NOT validate cross-file references (e.g., placement.device references a device atom that exists). Cross-reference validation is a CLI command that layers on top of schema validation.

## Admin CLI Architecture

### Module Structure

New top-level `admin/` Go module, independent of plant/ and monitoring/:

```
admin/
    go.mod                          -- module github.com/rustybrownlee/ot-simulator/admin
    go.sum
    cmd/admin/
        main.go                     -- CLI entry point, command routing
    internal/
        cli/                        -- Command implementations
            health.go               -- otsim health
            db.go                   -- otsim db status|validate|prune|export
            config.go               -- otsim config view|validate
            design.go               -- otsim design validate|list
            baseline.go             -- otsim baseline status|reset
        schema/                     -- JSON Schema loading and validation
            validator.go            -- Validate YAML against JSON Schema
            loader.go               -- Load schemas from design/schemas/
        dbutil/                     -- SQLite event store operations (read-only)
            status.go               -- DB stats, schema check
            export.go               -- CSV/JSON export
        web/                        -- Admin web server
            server.go               -- HTTP server, routes, middleware
            handlers_health.go      -- Health dashboard data
            handlers_db.go          -- DB management panel data
            handlers_config.go      -- Config viewer/editor data
            handlers_editor.go      -- YAML editor data, file operations
            handlers_baseline.go    -- Baseline status data
        templates/                  -- HTML templates (go:embed)
        static/                     -- Static assets (go:embed)
```

### Command Structure

```
otsim health                                    -- Service health summary
otsim db status                                 -- Event store statistics
otsim db validate                               -- Schema integrity check
otsim db prune [--older-than <days>]            -- Manual retention enforcement
otsim db export [--format csv|json] [--output <path>] [--device <id>] [--after <time>] [--before <time>]
otsim config view [<path>]                      -- Display parsed config
otsim config validate [<path>]                  -- Validate config file
otsim design validate <path>                    -- Validate YAML against schema
otsim design list                               -- List all design layer elements
otsim baseline status                           -- Show per-device baseline state
otsim baseline reset [--device <id>]            -- Trigger baseline re-learning
otsim web [--addr :8095]                        -- Start admin web server
```

### Cross-Cutting Concerns

- **Design directory**: CLI discovers `design/` via `--design-dir` flag or `OTS_DESIGN_DIR` env var (default: `./design`)
- **Monitor config**: CLI finds monitor.yaml via `--config` flag or `OTS_MONITOR_CONFIG` env var
- **Event DB**: CLI finds events.db via config's `event_db_path` or `--db` flag
- **Monitor API**: CLI reaches the monitoring API via `--api-addr` flag or `OTS_API_ADDR` (default: `localhost:8091`)
- **Plant health**: CLI checks plant ports via `--plant-ports` flag or `OTS_HEALTH_PORTS` env var

## Admin Web Interface

### Technology Stack

| Technology | Purpose | Notes |
|-----------|---------|-------|
| Bootstrap 5 | Layout and components | CDN, consistent with monitoring dashboard |
| HTMX | Dynamic updates | CDN, consistent with monitoring dashboard |
| CodeMirror 6 | YAML code editor | CDN (`@codemirror/lang-yaml`, `@codemirror/lint`) -- new dependency |
| go:embed | Template/static bundling | Single binary deployment |

CodeMirror 6 is the only new web dependency. It is loaded via CDN (ESM modules from esm.sh or cdn.jsdelivr.net), maintaining the no-build-step pattern. No npm, no webpack, no node_modules.

### Pages

| Path | Title | Content |
|------|-------|---------|
| `/` | Dashboard | Service health cards (plant ports, monitor API, event DB), quick stats |
| `/db` | Event Database | Row count, DB size, oldest/newest event, retention status, prune button, export form |
| `/config` | Configuration | monitor.yaml in CodeMirror editor with validation, save capability |
| `/design` | Design Library | File tree browser for design/ directory |
| `/design/edit?file=<path>` | YAML Editor | CodeMirror with YAML mode, schema validation, save/validate buttons |
| `/baseline` | Baselines | Per-device baseline status table, reset button |

### YAML Editor Design

The editor page has three panels:
1. **File tree** (left sidebar): Directory browser for `design/` showing devices/, networks/, environments/
2. **Editor** (center): CodeMirror 6 with YAML syntax highlighting, line numbers, bracket matching
3. **Validation panel** (bottom): Schema validation results with line numbers, severity, and fix suggestions

Validation workflow:
1. User opens a file from the tree or navigates directly via URL
2. Server loads the file content and determines its schema type from the file path
3. CodeMirror renders the YAML with syntax highlighting
4. User edits and clicks "Validate" -- HTMX POST sends YAML to server
5. Server validates against JSON Schema, returns line-level error/warning list
6. Editor shows lint markers at error lines; validation panel shows full error details
7. "Save" button is enabled only when validation passes (or user forces save)

### Port Assignment

| Port | Service | Notes |
|------|---------|-------|
| 8095 | Admin dashboard | New; admin web interface |

## SOW Backlog

### Ordered by Dependency

| # | SOW | Title | Dependencies | Status | Notes |
|---|-----|-------|-------------|--------|-------|
| 1 | SOW-034.0 | Design Layer JSON Schemas | None | Not Started | JSON Schema for device atom, network atom, environment, process YAML types |
| 2 | SOW-035.0 | Admin CLI Module and Core Commands | None | Not Started | New admin/ Go module, health check, db status/validate/prune/export, config view/validate |
| 3 | SOW-036.0 | Design Validation CLI | SOW-034.0, SOW-035.0 | Not Started | `otsim design validate` using JSON Schema, cross-reference validation, `otsim design list` |
| 4 | SOW-037.0 | Admin Web Dashboard | SOW-035.0 | Not Started | Web server on :8095, health dashboard, DB panel, config viewer, baseline status |
| 5 | SOW-038.0 | YAML Editor with Schema Validation | SOW-034.0, SOW-037.0 | Not Started | CodeMirror 6 editor, file tree browser, schema validation UI, save workflow |
| 6 | SOW-039.0 | CLI Reference Documentation | SOW-035.0, SOW-036.0 | Not Started | Command reference, examples, expected output for every CLI command |

### Dependency Graph

```
SOW-034.0 (JSON Schemas) ─────────────────────┐
  │                                            │
  └──> SOW-036.0 (Design Validate CLI) ────────┤
                                               ├──> SOW-038.0 (YAML Editor)
SOW-035.0 (CLI Foundation) ──> SOW-037.0 (Web) ┘
  │                              │
  ├──> SOW-036.0                 │
  └──> SOW-039.0 (CLI Docs) <───┘
```

Parallelism: SOW-034.0 and SOW-035.0 can be built concurrently. SOW-036.0 requires both. SOW-037.0 requires SOW-035.0. SOW-038.0 requires SOW-034.0 and SOW-037.0. SOW-039.0 can start once SOW-036.0 is complete.

## Configuration Additions

```yaml
# No changes to monitor.yaml. Admin tools discover existing config via CLI flags
# and environment variables.
```

### Environment Variables (New)

| Variable | Default | Used By | Purpose |
|----------|---------|---------|---------|
| `OTS_DESIGN_DIR` | `./design` | CLI, Web | Path to design layer directory |
| `OTS_MONITOR_CONFIG` | `./config/monitor.yaml` | CLI, Web | Path to monitoring config file |
| `OTS_API_ADDR` | `localhost:8091` | CLI, Web | Monitoring API address for health/baseline |
| `OTS_ADMIN_ADDR` | `:8095` | Web | Admin web server bind address |

## Port Assignments

| Port | Service | Change |
|------|---------|--------|
| 5020-5039 | Modbus TCP | No change; admin health-checks these |
| 8090 | Monitoring dashboard | No change |
| 8091 | Alert API | No change; admin reads health/baseline from here |
| 8095 | Admin dashboard | **New**: admin web interface |

## Technical Debt to Address

| ID | Description | Resolution in Beta 0.7 |
|----|-------------|------------------------|
| (none from prior milestones) | Admin tools are net-new | N/A |

Note: The implicit "schema lives only in Go structs" problem is resolved by SOW-034.0 (JSON Schemas). This was not tracked as formal technical debt but was identified as an operational gap.

## Technical Debt Created

| ID | Description | Notes |
|----|-------------|-------|
| td-admin-070 | JSON Schemas must be kept in sync with Go struct changes in plant/ | Manual sync process. Future: generate schemas from Go structs or vice versa. |
| td-admin-071 | Admin web server has no authentication | Acceptable for single-user training. Add auth if multi-tenant deployment becomes a requirement. |
| td-admin-072 | CodeMirror 6 loaded via CDN; no offline support | Admin dashboard requires internet for first load. Future: bundle via go:embed if offline support needed. |
| td-admin-073 | Cross-reference validation (device refs, network refs) is separate from schema validation | JSON Schema validates structure; cross-reference logic is Go code. Future: extend schema with $ref resolution or custom keywords. |
| td-admin-074 | Config editor saves to disk but does not hot-reload the running monitor | Engineer must restart monitor after config changes. Future: add SIGHUP reload or watch-based config reload to monitor. |
| td-admin-075 | Admin CLI binary name is `admin`, not `otsim` | The `otsim` name implies a unified CLI. Beta 0.7 delivers `admin` as the binary; renaming/wrapping as `otsim` is a future polish step if multiple binaries need consolidation. |

## Roadmap Impact

Beta 0.7 inserts platform administration tools before the commercial tool integration phase. The subsequent milestones shift:

| Previous | New | Focus |
|----------|-----|-------|
| Beta 0.7 | Beta 0.8 | Passive packet capture + Cisco CyberVision (Scenario 04 Phase B) |
| Beta 0.8 | Beta 0.9 | Dragos Platform threat detection (Scenario 04 Phase C) |
| Beta 0.9 | Beta 0.10 | Blastwave BlastShield microsegmentation (Scenario 04 Phase D) |

This insertion is justified: partner engineers need admin tools before they can independently operate the platform for commercial tool integration testing.

## Architecture Reference

- ADR-005: Monitoring Integration Architecture (admin reads monitoring API, does not bypass it)
- ADR-009: Design Layer and Composable Environments (schemas formalize the design layer contract)
- CLAUDE.md: Port assignments, technology stack, separation of concerns
