package protocol

import (
	"fmt"
	"log/slog"
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

// ListPorts returns available serial ports on the system.
func ListPorts() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, fmt.Errorf("failed to list serial ports: %w", err)
	}
	return ports, nil
}
