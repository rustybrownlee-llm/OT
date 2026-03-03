# Hint 1: What to Look For in the Event Table

If you are looking at the `/events` page and the data feels overwhelming, narrow your focus.
The event table contains a lot of rows, but each row has the same structure. Start with three
columns and ignore the rest.

---

## The Three Columns That Matter Most

**FC (Function Code)**: This number tells you what the request was asking the device to do.
In normal monitoring operations, you should see only two values:

- `1` -- Read Coils: asking a device for its discrete output states (pump running? valve open?)
- `3` -- Read Holding Registers: asking a device for its process values (flow rate, pressure, pH)

If you see `5`, `6`, `15`, or `16`, those are write operations. Something is sending commands
to devices, not just reading from them.

**Write badge**: A colored badge appears in the Write column when the function code is a write
operation (FC 5, 6, 15, or 16). During normal monitoring (Phases A1 through A4), you should
see zero write badges. The monitoring module only reads.

**Device**: Which device received the request. Watch this column for one minute and you will
see all 6 devices appear in rotation: wt-plc-01, wt-plc-02, wt-plc-03, mfg-gateway-01,
mfg-plc-01, mfg-plc-02. If a device stops appearing in the rotation, it has stopped
responding.

---

## Why FC 3 Dominates

Most OT process values (flow rates, pressures, temperatures, levels, setpoints) are stored in
holding registers (the 4x address space). FC 3 (Read Holding Registers) is the standard way
to read them. You will see far more FC 3 events than FC 1 events because most devices have
more holding registers than coils.

---

## What the "Write" Badge Means for Phase A5

When you reach Phase A5, you will send a single write command from `mbpoll`. After you run
that command, return to the `/events` page. The Write badge will appear in the table within
2 seconds. That is the event that will trigger the alerts. If you do not see the Write badge
appear after your command, the command did not reach the device.

---

## The Event Table Does Not Show Register Values

The event table shows what was requested, not what value was returned. It shows: "someone
asked device wt-plc-01 for 5 holding registers starting at address 0." It does not show what
those registers contained.

To see register values, go to `http://localhost:8090/assets/wt-plc-01`. The device detail
page shows the current register values from the most recent poll cycle.
