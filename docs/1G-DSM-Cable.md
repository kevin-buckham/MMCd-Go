# 1G DSM USB Datalogging Cable

Build guide for a USB datalogging cable for 1G DSM vehicles (1990-1994 Mitsubishi Eclipse, Eagle Talon, Plymouth Laser).

## Parts

| Qty | Part | Notes |
|-----|------|-------|
| 1 | 5V FTDI USB-to-TTL cable/adapter | Must have FT232R or FT232RL chip. Needs TX, RX, GND pins. |
| 1 | 1N914 or 1N4148 signal diode | Small signal diode (NOT a rectifier diode) |
| 1 | 10K ohm resistor | 1/4W, any tolerance |
| 1 | ALDL 12-pin connector or probe wires | To connect to the vehicle diagnostic port |

## ALDL Connector Pinout (12-pin)

The ALDL connector is located under the dash, driver side.

```
  ___________________________
 /  F  E  D  C  B  A         \
|  12  11  10  9  8  7         |
|   6   5   4   3   2   1     |
 \___________________________ /
```

Pins used for datalogging:

| Pin | Function | Description |
|-----|----------|-------------|
| 1 | Data | Bidirectional serial data line (half-duplex) |
| 10 | Diagnostic Enable | Must be grounded to enable diagnostic mode |
| 12 | Ground | Chassis/signal ground |

## FTDI Cable Pins Used

Only three pins from the FTDI adapter are needed:

| Pin | Signal | Wire Color (TTL-232R) |
|-----|--------|-----------------------|
| TX | Transmit Data (output) | Orange |
| RX | Receive Data (input) | Yellow |
| GND | Ground | Black |

**VCC, RTS, CTS are NOT connected.**

## Wiring Diagram

```
FTDI Adapter                                    ALDL Connector
============                                    ==============

                    1N914 Diode
TX (Orange) ────────|>|────┬──────────────────── Pin 1 (Data)
              (anode)  (cathode)
                           │
RX (Yellow) ───────────────┘
                           │
                      [10K Resistor]
                           │
GND (Black) ───────────────┼──────────────────── Pin 12 (Ground)
                           │
                           └──────────────────── Pin 10 (Diag Enable)
```

### Component Details

**Diode (1N914/1N4148):**
- Anode connects to FTDI TX
- Cathode (stripe/band) connects to the junction with RX and ALDL Pin 1
- Purpose: allows the FTDI to drive the data line while preventing it from fighting the ECU's open-collector output when idle

**10K Resistor:**
- Connects between ALDL Pin 1 (Data) and GND
- Purpose: pull-down resistor to establish a known idle state on the data line

**Diagnostic Enable (Pin 10):**
- Must be connected to Pin 12 (Ground) to put the ECU in diagnostic mode
- Without this jumper, the ECU will not respond to data requests

## FTDI EEPROM Configuration

The FTDI chip's EEPROM must be flashed with signal inversion enabled for both TX and RX. The 1G DSM ECU uses inverted signaling compared to standard UART.

### Linux (ftdi_eeprom)

Create a config file (e.g., `mmcd-ftdi.conf`):

```ini
vendor_id=0x0403
product_id=0x6001

invert_txd=true
invert_rxd=true
```

Flash with:

```bash
sudo ftdi_eeprom --flash-eeprom mmcd-ftdi.conf
```

### Windows (FT_Prog)

1. Download FT_Prog from the FTDI website
2. Scan for devices
3. Navigate to **Hardware Specific** > **Invert RS232 Signals**
4. Check **Invert TXD** and **Invert RXD**
5. Program the device

## Protocol Settings

| Parameter | Value |
|-----------|-------|
| Baud Rate | 1953 bps |
| Data Bits | 8 |
| Stop Bits | 1 |
| Parity | None |
| Flow Control | None |

The ECU uses a half-duplex protocol on a single wire. The app sends a 1-byte sensor address and receives a 2-byte response (echo of the address + data byte).

## Troubleshooting

**No data from ECU:**
- Verify Pin 10 is jumpered to Pin 12 (Ground) to enable diagnostic mode
- Verify the diode is oriented correctly (stripe/cathode toward ALDL Pin 1)
- Verify FTDI EEPROM has `invert_txd` and `invert_rxd` enabled
- Try with the key ON, engine OFF first to confirm basic communication
- Check that the correct serial port is selected in the app (on macOS, use `/dev/cu.usbserial-*`, not `/dev/tty.usbserial-*`)

**Garbled or intermittent data:**
- Verify baud rate is set to 1953
- Check solder joints and connections
- Verify the 10K resistor is connected between the data line and ground
- Keep cable length reasonable (under 2 meters from adapter to ALDL connector)

**Adapter not detected:**
- Install FTDI VCP (Virtual COM Port) drivers for your OS
- On macOS, the adapter should appear as `/dev/cu.usbserial-XXXXXXXX`
- On Linux, the adapter should appear as `/dev/ttyUSB0` (user must be in `dialout` group)

## References

- [DSMtuners: How to set up MMCD & make a logging cable](https://www.dsmtuners.com/threads/how-to-set-up-mmcd-make-a-logging-cable.203316/)
- [DSMtuners: The Easy Way to Make a 1G USB Datalogging Cable](https://www.dsmtuners.com/threads/the-easy-way-to-make-a-1g-usb-datalogging-cable.424006/)
- [EvoEcu Wiki: 1G DSM Datalogging Cable](https://evoecu.logic.net/wiki/1G_DSM_Datalogging_Cable)
- [FTDI TTL-232R Datasheet](https://ftdichip.com/wp-content/uploads/2023/07/DS_TTL-232R_CABLES.pdf)
