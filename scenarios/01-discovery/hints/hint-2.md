# Hint 2: Serial Gateways and Modbus Unit IDs

The device at port 5030 is a Moxa NPort 5150. Its register contents describe a serial port: baud
rate, data format, transmit count, receive count. It is not a PLC. It is a serial-to-Ethernet
gateway -- a converter that bridges legacy serial devices onto a TCP network.

Here is how it works:

```
Your workstation
      |
   TCP/IP
      |
  Moxa NPort 5150  (IP: 192.168.1.20, port 5030)
      |
   RS-485 serial bus
      |
   [Device A]  [Device B]  [Device C]
```

When you connect to the gateway IP and send a Modbus TCP request, the gateway strips the TCP
header and forwards the Modbus payload onto the serial bus. The serial devices receive the request
and respond. The gateway wraps the response back into TCP and returns it to you.

The gateway is transparent: it does not interpret the Modbus content. It passes it through.

So how does a serial device know the message is for it specifically? Modbus includes a **unit ID**
field in every message. When multiple devices share a serial bus, each device has a unique unit
ID (1 through 247). A device only responds to messages addressed to its own unit ID.

To reach a serial device behind the gateway, you specify:
1. The gateway IP address and port (to get to the gateway over TCP)
2. The unit ID of the specific device you want (to get past the gateway to the right serial device)

The gateway at port 5030 bridges to at least one serial device. What unit IDs should you try?
