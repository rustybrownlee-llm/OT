# Baseline Observation Template

**Facility**: Greenfield Water and Manufacturing
**Environment**: greenfield-water-mfg
**Date**: _______________
**Observed by**: _______________
**Monitor startup time**: _______________
**Baseline established time (first device)**: _______________

Fill in this template as you work through Phases A2 and A3. Compare your completed
observations against `solutions/solution.md` to verify accuracy.

---

## Section 1: Per-Device Function Code Profile

Record the function codes observed for each device from `http://localhost:8090/assets/{device-id}`.
The FC Distribution section shows which function codes were received during the observation window.

| Device | FC 1 (Read Coils) | FC 3 (Read HR) | FC 5 (Write Coil) | FC 6 (Write Reg) | FC 15 (Write Coils) | FC 16 (Write Regs) | Other FCs | IsWriteTarget |
|--------|-------------------|----------------|-------------------|------------------|--------------------|--------------------|-----------|---------------|
| wt-plc-01 | | | | | | | | |
| wt-plc-02 | | | | | | | | |
| wt-plc-03 | | | | | | | | |
| mfg-gateway-01 | | | | | | | | |
| mfg-plc-01 | | | | | | | | |
| mfg-plc-02 | | | | | | | | |

**Key**: Yes = observed in FC distribution. No = not observed. Exc = exception response.

**IsWriteTarget**: True if any write FC (5, 6, 15, or 16) was observed. False if only read
FCs were observed. This value determines whether a `write_to_readonly` alert will fire.

---

## Section 2: Response Time Profile

Record the observed response time range from each device's detail page.

| Device | Mean Response Time | Min Response Time | Max Response Time | Jitter | Notes |
|--------|-------------------|-------------------|-------------------|--------|-------|
| wt-plc-01 | | | | | |
| wt-plc-02 | | | | | |
| wt-plc-03 | | | | | |
| mfg-gateway-01 | | | | | |
| mfg-plc-01 | | | | | |
| mfg-plc-02 | | | | | |

---

## Section 3: Device Categorization

Based on the FC profiles in Section 1, categorize each device:

| Device | Category | Basis for Categorization |
|--------|----------|--------------------------|
| wt-plc-01 | Read-only / Read-write | |
| wt-plc-02 | Read-only / Read-write | |
| wt-plc-03 | Read-only / Read-write | |
| mfg-gateway-01 | Read-only / Read-write | |
| mfg-plc-01 | Read-only / Read-write | |
| mfg-plc-02 | Read-only / Read-write | |

Circle or underline the correct category for each device. Record the basis in the third
column (e.g., "FC 6 observed for setpoint HR[1]" or "FC 1 and FC 3 only, no write FCs").

---

## Section 4: Communication Pattern Summary

Complete this summary based on your observations from Phase A3.

**Source IP address(es) observed in the communication matrix**:
```
_____________________________________________
```

**Function codes observed across all devices during normal monitoring**:
```
_____________________________________________
```

**Any Write badges observed during Phases A1-A3** (expected: zero):
```
_____________________________________________
```

**Devices that do NOT appear in the event table FC 1 column** (expected: mfg-gateway-01):
```
_____________________________________________
```

---

## Section 5: Baseline Status Log

Record the transition times for each device from "learning" to "established":

| Device | Learning Started | Established At | Duration |
|--------|-----------------|----------------|---------|
| wt-plc-01 | | | |
| wt-plc-02 | | | |
| wt-plc-03 | | | |
| mfg-gateway-01 | | | |
| mfg-plc-01 | | | |
| mfg-plc-02 | | | |

---

## Section 6: CEF Syslog Observations (Phase A4)

Record observations from the CEF syslog feed.

**Dominant CEF severity in the feed** (expected: 1):
```
_____________________________________________
```

**Priority tag for read success events** (expected: `<134>`):
```
_____________________________________________
```

**Priority tag for write events** (expected: `<130>`, observed in Phase A5):
```
_____________________________________________
```

**Source address field (`src=`) in the CEF events**:
```
_____________________________________________
```

Does the `src` field match the source IP observed in the communication matrix (Section 4)?

```
Yes / No
```

---

## Section 7: Phase A5 Alert Summary

Complete this section after performing the Phase A5 write injection.

**Target device**: _______________
**Write command used**:
```
_____________________________________________
```

**Write injection time**: _______________

**Alert 1**:
- Rule ID: _______________
- Severity: _______________
- Device: _______________
- First seen: _______________

**Alert 2**:
- Rule ID: _______________
- Severity: _______________
- Device: _______________
- First seen: _______________

**Was `new_source` alert observed?** Yes / No

**Explanation for `new_source` absence**:
```
_____________________________________________
_____________________________________________
```

---

## Section 8: Notes

Record any observations that do not fit the tables above, including unexpected behavior,
discrepancies between expected and observed values, or questions for follow-up.

```
[Notes]
```
