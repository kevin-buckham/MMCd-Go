# MMCD Datalogger

Cross-platform diagnostic and datalogging tool for pre-OBDII (1990-1994) Mitsubishi vehicles — Eclipse, Eagle Talon, Plymouth Laser, Galant (4G63).

Rebuilt from the classic [MMCd PalmOS datalogger](https://mmcdlogger.sourceforge.net/) as a modern Go application with both a **desktop GUI** (Wails v2 + Svelte) and a **headless CLI**.

## Features

- **Live datalogging** — Poll up to 22 ECU sensors in real-time
- **CSV export** — Timestamped logs with human-readable converted values
- **Real-time graphs** — Scrolling time-series charts of selected sensors
- **DTC read/erase** — Read active and stored diagnostic trouble codes
- **Actuator tests** — Fuel pump, purge, EGR, injector disable
- **Cross-platform** — Windows, macOS, Linux (single binary ~10-15MB)
- **Headless CLI** — For Raspberry Pi, SSH, or scripted logging

## Hardware Requirements

You need a **USB-to-TTL serial adapter** (FTDI FT232, CH340, CP2102, etc.) and a **1N4148 diode** (or Schottky equivalent). No MAX232 level shifter is needed — the FTDI connects directly to the ECU's ALDL connector at TTL levels.

### ALDL Connector Pinout

```
  ┌─────────────────────────────┐
  │  1  2  3  4  5  6  7  8    │  ALDL 12-pin connector
  │  9  10 11 12               │  (under dash, driver side)
  └─────────────────────────────┘
   │      │      │
   │      │      └── Pin 12: Ground
   │      └── Pin 10: Diagnostic enable (short to ground)
   └── Pin 1: Serial data (bidirectional)
```

### The "Universal" Cable Schematic

This design works with MMCd, TunerPro, and EvoScan. The data line (Pin 1) is **bidirectional** — the ECU transmits and receives on the same wire. A **diode on the TX line** prevents the adapter's transmit signal from fighting the ECU's responses.

```
FTDI/USB-TTL Adapter                           ALDL Connector
─────────────────                              ──────────────

RX  (Yellow/White) ─────────────────────────── Car Pin 1 (Data)

                        1N4148 Diode
TX  (Green) ────────────┤►├────────────────── Car Pin 1 (Data)
                   (cathode→FTDI)
                   (anode→car)

GND (Black) ────────────────────────────────── Car Pin 12 (Ground)

                    Car Pin 10 ──── Car Pin 12 (jumper to enable diag mode)
```

**Wire color reference** (standard FTDI pinout):
- **RX** (Yellow or White) → directly to Car Pin 1
- **TX** (Green) → through diode to Car Pin 1
- **GND** (Black) → Car Pin 12

### Diode Details

The diode on TX ensures the adapter only drives the line low and doesn't hold it high between bytes, which would block the ECU's open-collector responses.

- **1N4148** (silicon): Forward drop ~0.7V. For 5V TTL, Logic Low threshold is 0.8V, so 0.7V is technically safe. Works for 99% of setups.
- **BAT85 or 1N5817** (Schottky): Forward drop ~0.2–0.3V. Gives a cleaner, more solid Logic Low with extra margin. **Use this if you have one.**

**Diode orientation:**
- **Stripe (cathode)** → towards FTDI adapter
- **Non-stripe (anode)** → towards car

### Connection Summary

| FTDI Pin | Wire Color | Connects To | Notes |
|----------|------------|-------------|-------|
| RX | Yellow/White | Car Pin 1 | Direct connection |
| TX | Green | Car Pin 1 | **Through diode** (anode→car, cathode→FTDI) |
| GND | Black | Car Pin 12 | Ground |
| — | — | Car Pin 10→12 | Jumper/bridge to enable diagnostic mode |

**Serial settings:** 1953 baud, 8 data bits, 1 stop bit, no parity, no flow control.

See the [original documentation](https://mmcdlogger.sourceforge.net/) for additional reference.

## Installation

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Wails v2](https://wails.io/docs/gettingstarted/installation) (for desktop GUI)
- [Node.js 18+](https://nodejs.org/) (for frontend build)

### Build

```bash
# Install dependencies
make install

# Build desktop app
make build

# Or build CLI only (no Wails/frontend needed)
make cli
```

## Usage

### Desktop GUI

```bash
# Launch the desktop app
./mmcd
```

### CLI Commands

```bash
# List available sensors
mmcd sensors

# Start datalogging with live terminal display
mmcd log --port /dev/ttyUSB0 --sensors RPM,TPS,COOL,TIMA --output log.csv --display

# Log all sensors
mmcd log --port /dev/ttyUSB0 --sensors all --output log.csv

# Read diagnostic trouble codes
mmcd dtc --port /dev/ttyUSB0

# Read and erase DTCs
mmcd dtc --port /dev/ttyUSB0 --erase

# Run actuator test
mmcd test --port /dev/ttyUSB0 --command fuel-pump

# Review a saved log
mmcd review --file log.csv

# Import an old MMCd PalmOS PDB log file to CSV
mmcd import --file 2003-01-17_First_run.PDB

# Import to native binary format (for replay)
mmcd import --file 2003-02-01_YEMELYA.PDB --format mmcd

# Import with imperial units
mmcd import --file log.PDB --units imperial
```

### Common Options

| Flag | Description | Default |
|------|-------------|---------|
| `--port, -p` | Serial port | (required) |
| `--baud, -b` | Baud rate | 1953 |
| `--units, -u` | Unit system: metric, imperial, raw | metric |

## Supported Sensors

| Slug | Address | Description | Unit |
|------|---------|-------------|------|
| RPM | 0x21 | Engine speed | rpm |
| TPS | 0x17 | Throttle position | % |
| COOL | 0x07 | Coolant temperature | °C/°F |
| TIMA | 0x06 | Timing advance | ° |
| KNCK | 0x26 | Knock sum | count |
| INJP | 0x29 | Injector pulse width | ms |
| INJD | computed | Injector duty cycle | % |
| O2-R | 0x13 | O2 sensor (rear) | V |
| O2-F | 0x3E | O2 sensor (front) | V |
| BATT | 0x14 | Battery voltage | V |
| BARO | 0x15 | Barometric pressure | bar/psi |
| AIRT | 0x3A | Air intake temperature | °C/°F |
| MAFS | 0x1A | Mass air flow | Hz |
| FTRL | 0x0C | Fuel trim low | % |
| FTRM | 0x0D | Fuel trim middle | % |
| FTRH | 0x0E | Fuel trim high | % |
| FTO2 | 0x0F | O2 feedback trim | % |
| ACLE | 0x1D | Accel enrichment | % |
| ISC | 0x16 | ISC position | % |
| EGRT | 0x12 | EGR temperature | °C/°F |
| FLG0 | 0x00 | Flags (AC clutch) | flags |
| FLG2 | 0x02 | Flags (TDC/PS/Idle) | flags |

## Log Formats

### CSV (default)
Human-readable timestamped log with both converted values and raw bytes. Each sensor gets two columns: `SLUG` (formatted value) and `SLUG_raw` (0-255). Created by `mmcd log --output file.csv` or `mmcd import --format csv`.

### .mmcd (native binary)
Compact binary format for efficient storage and replay. 48 bytes per sample (8-byte nanosecond timestamp + 4-byte dataPresent bitmask + 32-byte raw data + 4 bytes padding). Created by `mmcd import --format mmcd`. Can be replayed in the desktop GUI or converted to CSV.

### PDB (PalmOS import)
The original MMCd PalmOS app stored logs as `.PDB` database files using the FileStream `DBLK` format. These contain 40-byte `GraphSample` structs (big-endian) with PalmOS epoch timestamps. Use `mmcd import --file log.PDB` to convert.

## Protocol

The MMCD protocol is simple request-reply over serial:

1. Send 1 byte (sensor address)
2. Receive 2 bytes (echo of address + data byte)
3. Apply conversion formula to get engineering units

**Serial settings:** 1953 baud, 8 data bits, 1 stop bit, no parity, no flow control.

## License

This program is free software; you can redistribute it and/or modify it under the terms of the **GNU General Public License v2 or later** as published by the Free Software Foundation. See [LICENSE](LICENSE) for the full text.

Inspired by and thanks to the original [MMCd PalmOS datalogger](https://mmcdlogger.sourceforge.net/) by Dmitry Yurtaev (GNU GPL v2).

**Developers:** Kevin Buckham & Claude (Anthropic)

© 2026 Kevin Buckham & Claude (Anthropic)
