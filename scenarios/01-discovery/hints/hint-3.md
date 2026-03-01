# Hint 3: Unit ID Probing Methodology

Modbus unit IDs range from 1 to 247. You cannot scan a serial bus the way you scan an IP network.
Instead, probe unit IDs one at a time and interpret the responses.

**Three possible responses to a unit ID probe:**

1. **Valid register data**: A device exists at this unit ID and is responding normally.
2. **Modbus exception 02** (Illegal Data Address): A device exists but the address you requested
   is out of range. Legacy serial devices often use one-based addressing -- try starting at
   address 1, not address 0.
3. **Modbus exception 0x0B** (Gateway Target Device Failed to Respond): The gateway tried to
   reach a serial device at this unit ID but got no response. No device exists here.

**Stopping rule**: Probe sequentially starting at unit ID 1. Stop when you receive exception 0x0B
twice consecutively at consecutive unit IDs.

**The mbpoll command to probe unit ID 1 on port 5030:**

```
mbpoll -t 4 -r 1 -c 10 -1 -a 1 localhost -p 5030
```

The `-a 1` flag sets the Modbus unit ID. The `-r 1` flag starts at address 1 (not 0) because
legacy serial devices frequently use one-based addressing.

Try unit IDs 1, 2, 3, and 4. Record the responses. Two devices are present. The third and
fourth unit IDs will return exception 0x0B.

**After you find the devices:** Note the response times carefully. Both serial devices respond
through the same gateway, but they do not respond at the same speed. Different response times
through the same gateway are a signal that the devices are different models with different
processing speeds. This is a fingerprinting clue.
