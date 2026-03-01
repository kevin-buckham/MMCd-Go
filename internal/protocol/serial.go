package protocol

import (
	"fmt"
	"log/slog"
	goruntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

const (
	// DefaultBaudRate matches the original PalmOS MMCD code (1920 baud).
	DefaultBaudRate = 1920

	// DefaultDataBits for MMCD protocol.
	DefaultDataBits = 8
)

// SerialConn wraps a serial port connection to the ECU.
type SerialConn struct {
	mu       sync.Mutex
	port     serial.Port
	portName string
	baudRate int
	isOpen   bool
}

// NewSerialConn creates a new serial connection (not yet opened).
func NewSerialConn(portName string, baudRate int) *SerialConn {
	if baudRate <= 0 {
		baudRate = DefaultBaudRate
	}
	return &SerialConn{
		portName: portName,
		baudRate: baudRate,
	}
}

// Open opens the serial port with MMCD protocol settings (8N1, no flow control).
func (sc *SerialConn) Open() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.isOpen {
		return nil
	}

	mode := &serial.Mode{
		BaudRate: sc.baudRate,
		DataBits: DefaultDataBits,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}

	port, err := serial.Open(sc.portName, mode)
	if err != nil {
		return fmt.Errorf("failed to open serial port %s: %w", sc.portName, err)
	}

	// Set read timeout to 500ms (matching original code's half-second timeout)
	if err := port.SetReadTimeout(500 * time.Millisecond); err != nil {
		port.Close()
		return fmt.Errorf("failed to set read timeout: %w", err)
	}

	sc.port = port
	sc.isOpen = true
	slog.Info("serial port opened", "port", sc.portName, "baud", sc.baudRate)
	if sc.baudRate != DefaultBaudRate {
		slog.Warn("non-standard baud rate", "baud", sc.baudRate, "expected", DefaultBaudRate)
	}
	return nil
}

// Close closes the serial port.
func (sc *SerialConn) Close() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.isOpen {
		return nil
	}

	err := sc.port.Close()
	sc.isOpen = false
	sc.port = nil
	slog.Info("serial port closed", "port", sc.portName)
	return err
}

// IsOpen returns whether the port is currently open.
func (sc *SerialConn) IsOpen() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.isOpen
}

// Send writes bytes to the serial port.
func (sc *SerialConn) Send(data []byte) (int, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.isOpen {
		return 0, fmt.Errorf("serial port not open")
	}
	return sc.port.Write(data)
}

// Receive reads bytes from the serial port.
func (sc *SerialConn) Receive(buf []byte) (int, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.isOpen {
		return 0, fmt.Errorf("serial port not open")
	}
	return sc.port.Read(buf)
}

// PortName returns the configured port name.
func (sc *SerialConn) PortName() string {
	return sc.portName
}

// BaudRate returns the configured baud rate.
func (sc *SerialConn) BaudRate() int {
	return sc.baudRate
}

// Flush drains any stale bytes from the serial receive buffer.
func (sc *SerialConn) Flush() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.isOpen {
		return nil
	}
	return sc.port.ResetInputBuffer()
}

// ListPorts returns available serial ports on the system, filtered for relevance.
// On macOS: only /dev/cu.* ports (not /dev/tty.* which block on carrier detect),
// excluding Bluetooth, debug-console, and wlan-debug system ports.
// On Linux: excludes built-in /dev/ttyS* ports (motherboard UARTs).
// On Windows: no filtering (COM ports are already clean).
// USB-serial adapters are sorted to the top of the list.
func ListPorts() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, fmt.Errorf("failed to list serial ports: %w", err)
	}
	return filterPorts(ports), nil
}

// ListAllPorts returns all serial ports without platform filtering,
// but still sorted with USB-serial adapters first.
func ListAllPorts() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, fmt.Errorf("failed to list serial ports: %w", err)
	}
	sort.Slice(ports, func(i, j int) bool {
		iUSB := isUSBSerial(ports[i])
		jUSB := isUSBSerial(ports[j])
		if iUSB != jUSB {
			return iUSB
		}
		return ports[i] < ports[j]
	})
	return ports, nil
}

// macOS system ports that are never USB-serial adapters.
var macSystemPorts = []string{
	"Bluetooth-Incoming-Port",
	"debug-console",
	"wlan-debug",
}

// filterPorts removes irrelevant ports and sorts USB-serial adapters first.
func filterPorts(ports []string) []string {
	var filtered []string
	for _, p := range ports {
		if goruntime.GOOS == "darwin" {
			// Skip /dev/tty.* — use /dev/cu.* (callout) instead.
			// tty.* blocks on open() waiting for carrier detect.
			if strings.HasPrefix(p, "/dev/tty.") {
				continue
			}
			// Skip known macOS system ports
			skip := false
			for _, sys := range macSystemPorts {
				if strings.Contains(p, sys) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		} else if goruntime.GOOS == "linux" {
			// Skip built-in motherboard UARTs (ttyS0, ttyS1, etc.)
			if strings.HasPrefix(p, "/dev/ttyS") {
				continue
			}
		}
		filtered = append(filtered, p)
	}

	// Sort USB-serial adapters to the top
	sort.Slice(filtered, func(i, j int) bool {
		iUSB := isUSBSerial(filtered[i])
		jUSB := isUSBSerial(filtered[j])
		if iUSB != jUSB {
			return iUSB
		}
		return filtered[i] < filtered[j]
	})

	return filtered
}

// isUSBSerial returns true if the port looks like a USB-serial adapter.
func isUSBSerial(port string) bool {
	p := strings.ToLower(port)
	return strings.Contains(p, "usbserial") ||
		strings.Contains(p, "usbmodem") ||
		strings.Contains(p, "wchusbserial") ||
		strings.Contains(p, "slab_usbtouart") ||
		strings.Contains(p, "ttyusb") ||
		strings.Contains(p, "ttyacm")
}
