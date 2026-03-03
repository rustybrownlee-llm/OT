# Hint 4: Process View Navigation and ISA-5.1 Interpretation

This hint covers Phase G: Process View Context.

---

## Reading ISA-5.1 Instrument Tags

Every instrument label on the process view follows the ISA-5.1 tag convention. The tag encodes
two pieces of information: what is being measured (first letter) and what kind of device it is
(suffix letters).

**First letter -- measured variable**:

| Letter | Variable | Example |
|--------|---------|---------|
| F | Flow | FT-101 (flow transmitter), FIC-202 (flow indicating controller) |
| P | Pressure | PT-201 (pressure transmitter), PDT-201 (differential pressure transmitter) |
| T | Temperature | TT-101 (temperature transmitter) |
| A | Analysis | AT-201 (analyzer transmitter: turbidity, pH, DO, chlorine) |
| L | Level | LT-301 (level transmitter) |
| R | Radiation | RT-203 (radiation transmitter: UV intensity at 254nm) |
| S | Speed | SC-101 (speed controller: writable), ST-102 (speed transmitter: read-only) |
| Z | Position | ZT-101 (position transmitter: analog %), ZS-101 (position switch: discrete bool) |
| K | Time | KIC-101 (time indicating controller: cycle time setpoint, writable) |

**Suffix letters -- function**:

| Suffix | Meaning | Writable? |
|--------|---------|----------|
| T | Transmitter (measurement) | No -- read-only sensor |
| IC | Indicating Controller (setpoint) | Yes -- PLC setpoint commanding an actuator |
| C | Controller | Yes -- writable |
| S | Switch (discrete) | No -- read-only status |
| Q | Quantity (totalizer) | Sometimes -- check device atom |
| HS | Hand Switch | Yes (per project convention) -- SCADA-initiated command |
| run | Run command | Yes (project extension) -- binary start/stop coil |

The practical rule: if the tag suffix contains "T" alone or "S" alone, it is read-only. If it
contains "IC", "C", or "HS", it is likely writable. Always confirm against the device atom.

---

## ZT vs ZS: A Critical Distinction in the Pipeline Environment

The pipeline process view shows both ZT and ZS instruments on the same valves. They are different
things:

- **ZT-101** (Inlet Block Valve Position): analog feedback, 0-100%, HR[5] on ps-rtu-02. Reading
  50% means the valve is halfway between open and closed -- it may be transitioning, or it may
  be stuck.

- **ZS-101** (ESD Active Status): discrete coil, boolean, Coil[4] on ps-rtu-02. Reading 1 means
  the Emergency Shutdown sequence is active and the ESD valve has tripped closed. This coil is
  read-only and cannot be reset via Modbus -- it requires a local operator reset.

Do not confuse the analog valve position reading with the discrete ESD status. They are on
different registers and represent different physical facts.

---

## Pipeline Domain: The Three Things You Need to Know

If the gas pipeline environment is unfamiliar, focus on three concepts:

**1. What the compressor does**: It boosts pipeline pressure so gas can flow from source to
destination. Without compression, pressure drops along the pipeline and gas stops moving.
ps-plc-01 controls the compressor; run-C-101 (Coil[0]) is the run command.

**2. What custody transfer metering measures**: At the contract boundary between two pipeline
operators, the AGA-3 orifice meter calculates gas volume for billing. Three inputs: differential
pressure (PDT-201, HR[5] on ps-rtu-01), static pressure (PT-201, HR[3]), and temperature
(TT-201, HR[4]). Flow rate is proportional to sqrt(DP), so PDT-201 is the dominant input.
All three are on ps-rtu-01, which is station-LAN-only (10.20.1.20). You cannot reach ps-rtu-01
directly from the WAN -- only through a pivot via ps-plc-01.

**3. What the chromatograph determines**: The NGC measures gas quality -- BTU heating value
(AT-306, HR[6] on ps-fc-01) and specific gravity (AT-307, HR[7]). These determine the energy
value of the gas delivered. AT-306 and AT-307 feed into the AGA-3 calculation. ps-fc-01 is a
serial device behind ps-gw-01 (Moxa gateway, 10.20.1.30:5043). Access chain from WAN:
WAN -> ps-plc-01 -> station LAN -> ps-gw-01 -> ps-fc-01.

---

## FQ Totalizer Zero Is Normal at Rollover

FQ-201 (Meter Run 1 Volume Today) and FQ-250 (Station Total Volume Today) reset to zero at the
contract day rollover. The rollover time is set by contract -- often 9:00 AM per NAESB standards,
not midnight. If you observe a zero reading at or shortly after 9:00 AM, this is expected
behavior, not a meter failure or attack. The register accumulates again from zero until the
next rollover.

A zero reading at other times warrants investigation: it could indicate that all meter runs have
been disabled via HS-201 through HS-204 (the enable coils on ps-rtu-01), or that the RTU has
lost power or communications.

---

## Process View Refresh Rate Is Not SCADA Speed

The process view refreshes at approximately 2 seconds. Real SCADA HMI systems poll PLCs at
500ms to 1 second intervals. The slower refresh is intentional: the dashboard is a secondary
monitoring overlay, not the primary control system. The values are accurate but slightly behind
real time.

When you compare a process view reading to a simultaneous mbpoll read, the values may differ
by up to one polling cycle. Both readings come from the same Modbus TCP registers; the
discrepancy is the polling delay, not a data error.
