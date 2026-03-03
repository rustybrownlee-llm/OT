# Hint 3: Reading CEF Format

If the CEF output in your netcat terminal looks like a wall of text and you are not sure
how to parse it, this hint breaks the format down field by field.

---

## The CEF Structure

Every CEF message has exactly two parts: a header and an extensions block, separated by
the seventh pipe character.

```
<priority>CEF:0|vendor|product|version|signatureId|name|severity|extensions
```

Count the pipe characters: there are exactly 7 fields separated by 6 pipe characters in
the header. Everything after the seventh pipe is the extensions block.

---

## Breaking Down a Real Example

Take this event (line-wrapped here for readability; it arrives as one line):

```
<134>CEF:0|OTSimulator|Monitor|0.6|3|Read Holding Registers|1|src=127.0.0.1:52341 dst=127.0.0.1:5020 cs1=Read Holding Registers cs1Label=FunctionCode cn1=0 cn1Label=AddressStart cn2=5 cn2Label=AddressCount cs2=wt-plc-01 cs2Label=DeviceID cs3=greenfield-water-mfg cs3Label=Environment rt=1740873600000 outcome=success
```

**Priority tag** `<134>`:

This is the syslog priority. Calculate it: facility_code * 8 + syslog_severity_code.
For local0 (facility 16) and Informational (syslog severity 6): 16 * 8 + 6 = 134.

A write event will show `<130>` (16 * 8 + 2 = 130, Critical). This is the fastest way to
spot writes in the feed without reading the full line.

**`CEF:0`**: CEF format version. Always 0.

**`OTSimulator`**: Vendor.

**`Monitor`**: Product name.

**`0.6`**: Product version.

**`3`**: signatureId. This is the Modbus function code number.
- 1 = Read Coils
- 3 = Read Holding Registers
- 6 = Write Single Register
- 16 = Write Multiple Registers

**`Read Holding Registers`**: The human-readable name for that function code.

**`1`**: CEF severity. The mapping:
- 1 = read success (Low/Informational)
- 3 = diagnostic operation like FC 43 (Medium-Low)
- 5 = read failure, device returned exception (Medium)
- 7 = write operation, any FC 5/6/15/16 (High)

---

## The Extensions Block

After the seventh pipe, all fields are `key=value` pairs separated by spaces. The keys are
standardized CEF extension names:

| What you see | What it means |
|-------------|---------------|
| `src=127.0.0.1:52341` | Source IP and ephemeral port of the Modbus client |
| `dst=127.0.0.1:5020` | Device IP and port |
| `cs1=Read Holding Registers` | Function code name (string custom field 1) |
| `cs1Label=FunctionCode` | Label identifying what cs1 contains |
| `cn1=0` | Starting register address (numeric custom field 1) |
| `cn1Label=AddressStart` | Label for cn1 |
| `cn2=5` | Number of registers in the request |
| `cn2Label=AddressCount` | Label for cn2 |
| `cs2=wt-plc-01` | Device ID from the environment definition |
| `cs2Label=DeviceID` | Label for cs2 |
| `cs3=greenfield-water-mfg` | Environment ID |
| `cs3Label=Environment` | Label for cs3 |
| `rt=1740873600000` | Timestamp in milliseconds since Unix epoch |
| `outcome=success` | Whether device responded with data (success) or exception (failure) |

---

## Quick Reference: What to Look For in Phase A5

When you execute the write injection in Phase A5, scan the syslog feed for:

- Priority tag `<130>` -- this is the write event (severity Critical = syslog 2)
- `signatureId=6` -- FC 6 Write Single Register (or 16 for multiple registers)
- `severity=7` -- CEF High severity
- `outcome=success` -- the write was accepted by the device
- `cn1=2` -- register address 2 (if you targeted wt-plc-03 as suggested)
- `cs2=wt-plc-03` -- the device that received the write

If you do not see this event in the syslog feed, the monitor did not capture the write.
Check that the monitor is running and syslog is enabled in `monitor.yaml`.
