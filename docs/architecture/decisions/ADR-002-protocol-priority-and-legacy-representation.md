# ADR-002: Protocol Priority and Legacy Representation

**Status**: Proposed
**Date**: 2026-02-28
**Decision Makers**: Rusty Brownlee

---

## Context

Real OT environments use a mix of industrial protocols, many of which predate TCP/IP. The target environments this simulator models -- 30+ year old manufacturing floors connected to modern water treatment facilities -- span the full spectrum from RS-485 serial to Ethernet-based protocols.

### Protocol Landscape in Target Environments

| Protocol | Transport | Era | Where Found | Auth | Encryption |
|----------|-----------|-----|-------------|------|------------|
| Modbus RTU | RS-485 serial (9600 baud) | 1979 | Legacy manufacturing, old PLCs | None | None |
| Modbus TCP | TCP/IP port 502 | 1999 | Modern PLCs, serial-to-Ethernet bridges | None | None |
| EtherNet/IP | TCP/UDP port 44818 | 2001 | Allen-Bradley environments | None | None |
| DNP3 | TCP port 20000 or serial | 1990s | Utilities, remote sites | Optional (SA) | None standard |
| OPC DA | COM/DCOM (Windows) | 1996 | HMI-to-server, historian integration | Windows auth | None |
| OPC UA | TCP port 4840 | 2008 | Modern installations | Certificate | TLS optional |
| PROFINET | Ethernet Layer 2 | 2004 | Siemens environments | None | None |
| BACnet/IP | UDP port 47808 | 1995 | Building automation, HVAC | None | None |

### Go Library Ecosystem

| Protocol | Library | Stars | License | Client | Server | Maturity |
|----------|---------|-------|---------|--------|--------|----------|
| Modbus TCP/RTU | simonvetter/modbus | 408 | MIT | Yes | Yes | Good |
| Modbus TCP/RTU | goburrow/modbus | 1,024 | BSD-3 | Yes | No | Mature |
| OPC UA | gopcua/opcua | 800+ | MIT | Yes | Yes | Good |
| BACnet/IP | alexbeltran/gobacnet | 51 | MIT | Yes | Limited | Early |
| EtherNet/IP | loki-os/go-ethernet-ip | ~50 | MIT | Yes | No | Early |
| DNP3 | No mature Go library | -- | -- | -- | -- | -- |

---

## Decision

### D1: Modbus TCP as Primary Protocol (Phase 1)

**Decision**: Modbus TCP is the first and primary protocol implemented.

**Rationale**: Modbus TCP is the most widely deployed industrial protocol, the simplest to implement, and the most relevant to the target learning environment. Every OT security tool supports it. Every water treatment plant and manufacturing floor has it. Starting here gives immediate educational value.

**Go library**: `simonvetter/modbus` (MIT, client + server in one package). Must be validated in a POC before production use.

**Fallback**: `goburrow/modbus` (BSD-3, client only) for the monitoring side if `simonvetter/modbus` proves insufficient for server-side simulation.

### D2: Modbus RTU Simulation for Legacy Devices (Phase 2)

**Decision**: Simulated Modbus RTU over virtual serial ports represents legacy serial-only devices.

**Rationale**: The manufacturing floor's oldest PLCs (Modicon 984, SLC-500) communicate via RS-485 serial. IT engineers must understand that these devices cannot simply be "put on the network." The simulation models the serial-to-Ethernet conversion step that real environments require (Moxa NPort, Digi, ProSoft gateways).

**Implementation approach**:
- Virtual PLC runs Modbus RTU logic internally
- A simulated serial-to-Ethernet gateway converts RTU frames to Modbus TCP
- The gateway introduces realistic conversion artifacts: additional latency (5-20ms), one-at-a-time request serialization, and the expanded attack surface of the converter itself

### D3: Protocol Priority Order for Future Phases

**Decision**: Additional protocols are added in this order based on training value and library maturity.

| Priority | Protocol | Phase | Rationale |
|----------|----------|-------|-----------|
| 1 | Modbus TCP | Phase 1 | Universal, simple, immediate value |
| 2 | Modbus RTU (simulated serial) | Phase 2 | Legacy reality, serial-to-Ethernet teaching |
| 3 | OPC UA | Phase 3+ | Modern standard, TLS capable, contrast with Modbus |
| 4 | EtherNet/IP | Future | Allen-Bradley environments, CIP protocol |
| 5 | BACnet/IP | Future | Building automation, HVAC integration |
| 6 | DNP3 | Future | Utility/power extension, requires custom implementation |

### D4: No Proprietary Protocol Simulation

**Decision**: Proprietary protocols (Allen-Bradley DF1, Siemens MPI/PPI, Mitsubishi MELSEC) are not simulated.

**Rationale**: These protocols require specialized hardware or deep reverse engineering. The educational value of simulating proprietary protocols does not justify the implementation cost. Vendor-specific behavior is instead represented through device profiles (ADR-004) that model different register maps, response times, and diagnostic capabilities over standard Modbus.

---

## Consequences

### Positive
- Modbus TCP gives immediate educational value with mature Go library support
- Simulated serial-to-Ethernet conversion teaches a critical real-world concept
- Priority order aligned with library maturity reduces implementation risk
- OPC UA in Phase 3 creates a natural "before and after" security comparison (no auth vs. TLS)

### Negative
- No EtherNet/IP means Allen-Bradley-heavy environments are partially modeled
- No proprietary protocols means some vendor-specific behaviors are approximated
- DNP3 has no mature Go library, requiring custom implementation or C binding

### Risks
- **Risk**: `simonvetter/modbus` server implementation may have gaps for edge cases
- **Mitigation**: POC validation (POC-001) before production use; `goburrow/modbus` as client-side fallback
- **Risk**: Simulated serial may not expose the timing characteristics that make real serial challenging
- **Mitigation**: Add configurable latency and serialization constraints to the gateway simulation

---

## References
- simonvetter/modbus: https://github.com/simonvetter/modbus
- goburrow/modbus: https://github.com/goburrow/modbus
- gopcua/opcua: https://github.com/gopcua/opcua
- Modbus Application Protocol Specification V1.1b3
- Modbus Messaging on TCP/IP Implementation Guide V1.0b
