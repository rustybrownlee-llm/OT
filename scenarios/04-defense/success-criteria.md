# Scenario 04 Phase A: Success Criteria

This checklist defines completion for Scenario 04 Phase A. Evaluate each item against your
observations, completed baseline template, and notes from the exercise. All items must be
satisfied to mark Phase A complete.

---

## Phase A1: Deploy and Configure Monitoring (8 items)

- [ ] **SC-A1-01**: Monitoring dashboard opened at `http://localhost:8090`. Overview page
  shows exactly 6 devices for the greenfield-water-mfg environment: wt-plc-01, wt-plc-02,
  wt-plc-03, mfg-gateway-01, mfg-plc-01, mfg-plc-02. All 6 devices show Online status.

- [ ] **SC-A1-02**: Transaction event log opened at `http://localhost:8090/events`. Events
  are arriving in real time. The following columns are present and contain data: Time, Device,
  FC, FC Name, Address, Count, Success, Write.

- [ ] **SC-A1-03**: Event table observed for 60 seconds. FC 3 (Read Holding Registers) is
  the dominant function code. FC 1 (Read Coils) appears for devices with coils. No Write
  badges are visible during normal monitoring operation.

- [ ] **SC-A1-04**: All 6 device IDs appear in the event table within one polling cycle
  (2 seconds): wt-plc-01, wt-plc-02, wt-plc-03, mfg-gateway-01, mfg-plc-01, mfg-plc-02.

- [ ] **SC-A1-05**: Communication matrix opened at `http://localhost:8090/comms`. A star
  topology is visible: all traffic originates from a single source IP address (the monitoring
  process) connecting to all 6 device endpoints. No peer-to-peer device communications
  are visible.

- [ ] **SC-A1-06**: Source IP address of the monitoring process recorded from the `/comms`
  view. This address matches the `src` field in CEF events observed in Phase A4.

- [ ] **SC-A1-07**: Polling frequency confirmed by event counting. Approximately 15 events
  per device per 30 seconds (1 per 2-second poll cycle). Serial devices (mfg-plc-01 and
  mfg-plc-02) generate events with visibly longer response times than the Ethernet PLCs.

- [ ] **SC-A1-08**: Can explain in one sentence the difference between active polling (this
  phase) and passive capture (Phase B). Correct answer identifies that active polling injects
  Modbus read requests from the monitor, while passive capture observes copies of existing
  traffic without injecting any packets.

---

## Phase A2: Build a Traffic Baseline (8 items)

- [ ] **SC-A2-01**: Baseline learning period waited out (approximately 5 minutes from monitor
  startup). At least one device shows "Established" baseline status on
  `http://localhost:8090/assets` before proceeding to Phase A3.

- [ ] **SC-A2-02**: Baseline status API checked at `http://localhost:8091/api/baselines`.
  Baseline status transitions from "learning" to "established" observed and documented with
  timestamp.

- [ ] **SC-A2-03**: Per-device function code profile recorded for all 6 devices. Baseline
  observation template in `reference/baseline-observation-template.md` completed with FC
  distribution columns for each device.

- [ ] **SC-A2-04**: Write function codes (FC 5, FC 6, FC 15, FC 16) identified or confirmed
  absent in each device's FC profile. The `IsWriteTarget` flag accurately reflects whether
  each device received any write FC during the learning period.

- [ ] **SC-A2-05**: Response time range recorded for all 6 devices. Water treatment Ethernet
  PLCs show approximately 5ms. Moxa gateway shows approximately 15ms. SLC-500 (mfg-plc-01)
  shows approximately 65ms with ±20ms jitter. Modicon 984 (mfg-plc-02) shows approximately
  95ms with ±50ms jitter.

- [ ] **SC-A2-06**: Can explain why mfg-plc-02 (Modicon 984) has higher response time jitter
  than mfg-plc-01 (SLC-500). Correct answer: the Modicon 984 is a 1988-era processor with
  variable interrupt latency. The jitter is a characteristic of the device, not a network
  fault or attack indicator.

- [ ] **SC-A2-07**: Device detail page at `http://localhost:8090/assets/{device-id}` opened
  for at least 3 devices. FC Distribution section observed. Response time statistics observed.
  Baseline status confirms "established" after learning period.

- [ ] **SC-A2-08**: Can explain in one sentence why the 5-minute learning period is
  compressed for training. Correct answer: commercial tools (Dragos, CyberVision) use
  30-90 days to capture maintenance windows, shift changes, and seasonal variation. The
  5-minute period is sufficient for the lab environment but would not capture the full
  range of legitimate behavior in a production environment.

---

## Phase A3: Analyze Communication Patterns (8 items)

- [ ] **SC-A3-01**: All 6 devices categorized as read-only or read-write based on observed
  FC profiles. Categorization matches the expected table in Phase A3 of the scenario README.
  Any discrepancy between expected and observed is documented.

- [ ] **SC-A3-02**: Write filter applied on `/events` page. Zero write events confirmed
  during Phases A1-A3 (before the Phase A5 injection). This zero-write observation is the
  read-only baseline for the targeted device.

- [ ] **SC-A3-03**: FC histograms compared across at least 4 devices. Water treatment PLCs
  show FC 3 and FC 1. Moxa gateway shows FC 3 only (FC 1 not present). Serial PLCs show
  same FC pattern as Ethernet PLCs with longer response times.

- [ ] **SC-A3-04**: FC 1 absence on mfg-gateway-01 explained. Correct answer: the Moxa NPort
  5150 gateway status registers are all holding registers (4x address space). FC 1 (Read
  Coils) on the gateway returns Modbus exception 02 (Illegal Data Address). The gateway has
  no coils because it is not a PLC controlling physical actuators.

- [ ] **SC-A3-05**: Gateway aggregation teaching point understood. Can state that traffic to
  mfg-gateway-01 represents aggregated access to all serial devices behind it (mfg-plc-01 and
  mfg-plc-02). A Modbus request to mfg-plc-01 traverses: monitor --> gateway (TCP) --> RS-485
  bus --> SLC-500. The gateway is the single network-access point for both serial PLCs.

- [ ] **SC-A3-06**: Normal communication pattern documented. Source: single monitoring IP.
  Destination: 6 device endpoints. FCs: FC 1 and FC 3 only. Direction: monitor initiates
  all requests; devices only respond. Frequency: 1 poll per 2 seconds per device. Writes: zero.

- [ ] **SC-A3-07**: Can state what deviation from the normal pattern would indicate a security
  event. At minimum three of the following: new source IP, new function code, write to a
  read-only device, unexpected polling gap, FC 43 device identification request, peer-to-peer
  PLC communication.

- [ ] **SC-A3-08**: Can explain why a device with no Modbus coils is unlikely to be
  controlling physical actuators. Correct answer: coils represent discrete outputs (relay
  contacts, digital output cards) that drive physical actuators (pump starters, valve
  solenoids). A device with no coils has no discrete output capability in its Modbus map.

---

## Phase A4: Configure SIEM Forwarding (8 items)

- [ ] **SC-A4-01**: Syslog section added to `monitoring/config/monitor.yaml` with the correct
  field values: `enabled: true`, `target: "localhost:1514"`, `protocol: "udp"`,
  `facility: "local0"`, `format: "cef"`. No URL format used in the target field.

- [ ] **SC-A4-02**: Netcat listener started and receiving CEF events. Events are arriving
  continuously within one poll cycle after monitor restart.

- [ ] **SC-A4-03**: CEF header parsed successfully for at least one event. All seven header
  fields identified: priority, CEF version, vendor, product, version, signatureId, name,
  severity.

- [ ] **SC-A4-04**: CEF extensions parsed for at least one event. All required extension
  fields identified: `src`, `dst`, `cs1`/`cs1Label`, `cn1`/`cn1Label`, `cn2`/`cn2Label`,
  `cs2`/`cs2Label`, `cs3`/`cs3Label`, `rt`, `outcome`.

- [ ] **SC-A4-05**: CEF severity 1 confirmed as the dominant severity in the live feed
  (read success events). Severity 7 events not present in the feed during Phases A1-A4
  (no writes have been sent yet).

- [ ] **SC-A4-06**: Priority calculation demonstrated for at least one event. For facility
  local0 (code 16) and a read success (syslog severity 6): 16*8+6 = 134. The `<134>` tag
  is present on read success events.

- [ ] **SC-A4-07**: Can explain why FC 43 (Device Identification) events generate CEF
  severity 3 rather than severity 1 (same as other read operations). Correct answer
  references the TRITON/TRISIS attack: FC 43 was used as a reconnaissance vector to
  enumerate Schneider Electric Triconex safety PLC identity and firmware version. Severity
  3 ensures FC 43 events appear in threat-hunting dashboards rather than being silenced as
  informational noise.

- [ ] **SC-A4-08**: Can explain the security risk of plaintext syslog transport. Correct
  answer: an attacker capturing DMZ traffic can extract device topology, polling schedule,
  which devices receive write commands, and response time signatures that reveal device
  types. Production environments should use TLS-encrypted syslog (RFC 5425). The lab uses
  plaintext to avoid certificate management complexity.

---

## Phase A5: Detect Unauthorized Activity (8 items)

- [ ] **SC-A5-01**: Target device baseline confirmed as "Established" before executing the
  write injection. Write injection performed only after confirmation.

- [ ] **SC-A5-02**: `mbpoll` write command executed successfully. Expected output: "Written
  1 registers at address 2 on server localhost:5022" (or equivalent for the chosen target
  device).

- [ ] **SC-A5-03**: `write_to_readonly` alert observed at severity Critical within one polling
  cycle (2 seconds) of the write injection. Alert includes the targeted device ID. Alert rule
  ID is exactly `write_to_readonly` (underscores, not hyphens).

- [ ] **SC-A5-04**: `fc_anomaly` alert observed at severity High within one polling cycle of
  the write injection. Alert includes the targeted device ID and the unexpected function code
  (FC 6 Write Single Register). Alert rule ID is exactly `fc_anomaly`.

- [ ] **SC-A5-05**: Alert details reviewed from API. `alert_id` field follows deterministic
  format: `write_to_readonly:wt-plc-03:-1` (or the chosen device ID). `first_seen` and
  `last_seen` timestamps match the time of the write injection.

- [ ] **SC-A5-06**: CEF severity 7 event observed in the syslog feed for the write operation.
  The `cs1` field shows "Write Single Register". The `outcome` field shows "success" (or
  "failure" if the device returned an exception). The `cn1` field shows address 2 (or the
  chosen address).

- [ ] **SC-A5-07**: `new_source` alert confirmed absent after the write injection. Can explain
  why: the write was performed from the same workstation as the monitoring process (localhost
  source IP). The monitor already observed this source IP during the learning period. A
  `new_source` alert would only fire if the write came from a different IP address not seen
  during learning.

- [ ] **SC-A5-08**: Can explain why two alerts fired from a single write operation. Correct
  answer: the `write_to_readonly` rule fires because the device was baselined as read-only
  (IsWriteTarget=false); any write FC triggers it. The `fc_anomaly` rule fires because FC 6
  was not in the observed function code set from the learning period. The two rules detect
  from different angles: one checks the device's write-target status, the other checks the
  specific function code. Together they provide defense-in-depth.

---

## Conceptual Understanding (4 items)

- [ ] **SC-CON-01**: Can explain in two or three sentences why behavioral baselines work
  better than signature-based detection in OT environments. Correct answer includes: OT
  environments are highly deterministic (same devices, same function codes, same intervals
  every day); attackers increasingly use the legitimate Modbus protocol itself as the attack
  vector (no malware to detect); behavioral deviations from the learned normal are anomalous
  because the environment changes slowly and predictably.

- [ ] **SC-CON-02**: Can describe one capability that passive capture (Phase B) provides
  that active polling (Phase A) cannot. Correct answer: visibility into traffic from other
  sources. Active polling makes the monitor the only Modbus client; passive capture reveals
  all device conversations including SCADA server polls, engineering workstation connections,
  and unauthorized sources. The `new_source` alert becomes meaningful only with passive
  capture.

- [ ] **SC-CON-03**: Can name two things that commercial tools (Dragos Platform, Cisco
  CyberVision) automate from the manual process performed in this exercise. Any two of:
  automated asset inventory from traffic observation, automated baseline learning without
  operator interaction, threat intelligence matching against known ICS adversary TTPs,
  built-in alert rules for known OT attack patterns, pre-built SIEM connector integrations.

- [ ] **SC-CON-04**: Can state why it is important to understand the manual monitoring layer
  before deploying commercial tools. Correct answer: automated tools fail in predictable ways;
  evaluating false positives at 2:00 AM requires understanding what the tool is doing and
  why; an engineer who has never built a behavioral baseline cannot determine whether an
  automated baseline alert is a genuine threat or a misconfigured learning period. The
  manual layer provides the mental model for troubleshooting the automated layer.

---

## Completion Threshold

Phase A is complete when all 44 items above are satisfied. Partial completion is valid for
study purposes:

- SC-A1 through SC-A3 can be completed without syslog configuration (Phases A1-A3 only require
  the plant and monitor running)
- SC-A4 requires the syslog configuration change and a local listener tool (nc or equivalent)
- SC-A5 requires the baseline to be established and `mbpoll` available for the write injection
- SC-CON items are self-assessment and can be answered without running tools

Compare your baseline observation template against `solutions/solution.md` to verify that your
recorded FC profiles and device categorizations match the expected observations.
