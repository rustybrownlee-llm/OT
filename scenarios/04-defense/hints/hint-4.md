# Hint 4: Why No Alerts Yet?

If you have performed the Phase A5 write injection and no alerts appear on
`http://localhost:8090/alerts`, one of three things is happening:

---

## Reason 1: The Baseline Is Not Established Yet

Alerts only fire after the baseline transitions to "established." If the baseline is still
in the "learning" phase when you send the write, the event is recorded but no alert fires.

**How to check**: Navigate to `http://localhost:8090/assets`. Look at the Baseline Status
column for the device you targeted. If it shows "Learning", wait for it to change to
"Established" and then repeat the write injection.

**Confirmation check**: Navigate to `http://localhost:8091/api/baselines` and find the
entry for your target device. Look for `"status": "established"` in the JSON. If the value
is `"learning"`, the engine is still accumulating samples.

The learning period requires 150 polling cycles at 2-second intervals. This is approximately
5 minutes from the moment the monitor first polled that device. If you started the monitor
recently, wait and check again.

---

## Reason 2: The Write Hit the Wrong Device or Port

If you targeted the wrong port or the wrong register, the write may have succeeded but
targeted a device that is baselined as a read-write device (one that legitimately receives
write commands during normal operations).

For example, if you accidentally wrote to `localhost -p 5020` (wt-plc-01, the intake PLC)
instead of `localhost -p 5022` (wt-plc-03, the distribution PLC), the write may not have
triggered `write_to_readonly` because wt-plc-01 is categorized as a read-write device
(it receives write commands for the pump speed setpoint SC-101 during normal operations).

**How to check**: Look at the `/events` page with the Write filter enabled. You should see
one write event from your injection. The Device column will show which device actually
received the write. If it is not wt-plc-03 (or your intended target), repeat the injection
against the correct port.

**Correct command for wt-plc-03** (distribution PLC, read-only):

```
mbpoll -t 4 -r 2 -c 1 -1 localhost -p 5022 -- 9999
```

---

## Reason 3: The Monitor Did Not Observe the Write

The monitoring module only observes Modbus transactions that it initiates. If you sent your
write command while the monitor's connection to the device was temporarily closed between
poll cycles, the monitor will not have captured it.

However, this scenario is unlikely. The monitor maintains persistent TCP connections to
devices and captures all transactions it sends. If your mbpoll command completed
successfully (returned "Written 1 registers"), the monitor captured it.

**More likely**: the monitor captured it but the alert has not yet propagated to the alerts
page. Alert evaluation happens asynchronously. Wait 5-10 seconds and refresh
`http://localhost:8090/alerts`.

---

## What a Successful Detection Looks Like

If alerts are firing correctly, the `/alerts` page will show exactly two new alerts with
timestamps matching your write injection time:

1. `write_to_readonly` -- severity Critical -- device wt-plc-03 (or your target)
2. `fc_anomaly` -- severity High -- device wt-plc-03 (or your target)

The alerts page may also show pre-existing `value_out_of_range` alerts from sensor noise.
These are from before your injection. Look for the two new alerts with timestamps matching
your injection time.

If you still see no alerts after confirming the baseline is established, the write hit the
correct device, and you have waited 10 seconds: check the monitor logs for any error
messages from the event hook or baseline engine.
