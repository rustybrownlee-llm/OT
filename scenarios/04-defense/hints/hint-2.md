# Hint 2: Understanding the Baseline

If you are stuck on Phase A2 and not sure what the baseline engine is doing, or if you are
waiting and unsure whether to keep waiting, this hint explains what is happening internally.

---

## What Gets Learned During the Learning Period

The baseline engine runs a 150-cycle learning period (approximately 5 minutes at 2-second
polling). During those 150 cycles, for each device it records:

**From register snapshots (existing engine)**:
- The statistical distribution of each register's values (mean, standard deviation)
- Whether each register moves at all (some registers are static configuration values;
  others drift continuously as the process runs)
- Response time statistics (mean, standard deviation)

**From transaction events (new event-driven engine)**:
- Which function codes were received (the ObservedFCs set)
- Which source addresses sent requests (the ObservedSrcs set)
- Whether any write function code (FC 5, 6, 15, 16) was observed (IsWriteTarget flag)
- The polling interval between consecutive events (for detecting poll gaps)

When the learning period ends, the engine freezes these learned values and begins evaluating
new events against them.

---

## How to Check Whether Learning Is Complete

Two ways:

**1. Assets page**: Navigate to `http://localhost:8090/assets`. Each device row has a Baseline
Status column. Look for "Established" instead of "Learning". This updates in real time.

**2. Baselines API**: Navigate to `http://localhost:8091/api/baselines`. This returns JSON
showing the baseline status for each device. Look for `"status": "established"`.

The transition happens device by device, not all at once. The three Ethernet PLCs (wt-plc-01,
wt-plc-02, wt-plc-03) will usually transition first because their polling started first.
The serial PLCs (mfg-plc-01, mfg-plc-02) may finish slightly later if the gateway unit ID
discovery took a few extra cycles to complete.

---

## Why Alerts Do Not Fire During the Learning Period

This is a deliberate design choice, not a bug. During learning, the engine is building its
model of "normal." Firing alerts before the model is complete would generate false positives:
the first write to a device during learning would fire `write_to_readonly` even if writes
are legitimately part of normal operations for that device (as they are for wt-plc-01 and
wt-plc-02).

The alert rules only activate after `"status": "established"`. This is the same approach
commercial tools use: Dragos calls the pre-alert period the "characterization phase."

If you perform the Phase A5 write injection before the baseline is established, no alerts
will fire. Confirm established status before injecting.

---

## What Happens After Learning

Once established, every new event is evaluated against the frozen baseline:

- New function code not seen during learning? Fire `fc_anomaly`.
- Write to a device that was read-only during learning? Fire `write_to_readonly`.
- New source address not seen during learning? Fire `new_source`.
- No event from a device for more than 3x the learned polling interval? Fire `poll_gap`.

The baselines do not update after establishment (in this version). A new FC seen after
establishment is permanently anomalous until the device is removed and re-learned. This is
a technical simplification for the lab (td-baseline-029); production tools use sliding
window baselines that adapt slowly over time.
