package protocol

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kbuckham/mmcd/internal/sensor"
)

// validCommandAddrs is the whitelist of known ECU command addresses.
var validCommandAddrs = map[byte]bool{
	0xCA: true,                                     // DTC erase
	0xF1: true, 0xF2: true, 0xF3: true, 0xF4: true, // actuator tests
	0xF5: true, 0xF6: true, 0xF7: true, 0xF8: true,
	0xF9: true, 0xFA: true, 0xFB: true, 0xFC: true,
}

// ECU handles communication with the Mitsubishi OBDI ECU.
type ECU struct {
	conn  *SerialConn
	defs  []sensor.Definition
	busMu sync.Mutex // held for entire send+receive cycles to prevent interleaving
}

// NewECU creates a new ECU communicator.
func NewECU(conn *SerialConn, defs []sensor.Definition) *ECU {
	return &ECU{
		conn: conn,
		defs: defs,
	}
}

// Probe sends a single sensor query to verify the ECU is responding.
// It flushes the receive buffer first to clear any stale data, then queries
// the RPM sensor (0x21). Returns nil on success or an error describing
// exactly what went wrong (timeout, echo mismatch, etc.).
func (e *ECU) Probe() error {
	e.conn.Flush()

	const probeAddr byte = 0x21 // RPM — always available with key ON
	data, err := e.QuerySensor(probeAddr)
	if err != nil {
		return fmt.Errorf("ECU probe failed (addr 0x%02X): %w", probeAddr, err)
	}
	slog.Info("ECU probe OK", "addr", fmt.Sprintf("0x%02X", probeAddr), "data", fmt.Sprintf("0x%02X", data))
	return nil
}

// QuerySensor sends a sensor address to the ECU and reads the response.
// The ECU echoes the address byte back, followed by the data byte.
// Returns the raw data byte.
// Addresses >= 0xC0 are rejected as they are in the command range.
func (e *ECU) QuerySensor(addr byte) (byte, error) {
	if addr >= 0xC0 {
		return 0, fmt.Errorf("address 0x%02X is in command range (>=0xC0), refusing to poll", addr)
	}

	e.busMu.Lock()
	defer e.busMu.Unlock()

	// Send the address byte
	_, err := e.conn.Send([]byte{addr})
	if err != nil {
		return 0, fmt.Errorf("failed to send sensor address 0x%02X: %w", addr, err)
	}

	// Read 2 bytes: echo + data
	buf := make([]byte, 2)
	totalRead := 0
	deadline := time.Now().Add(500 * time.Millisecond)

	for totalRead < 2 && time.Now().Before(deadline) {
		n, err := e.conn.Receive(buf[totalRead:])
		if err != nil {
			e.conn.Flush()
			return 0, fmt.Errorf("failed to read response for 0x%02X: %w", addr, err)
		}
		totalRead += n
	}

	if totalRead < 2 {
		e.conn.Flush()
		return 0, fmt.Errorf("timeout reading response for 0x%02X: got %d bytes", addr, totalRead)
	}

	// Verify echo — discard sample on mismatch
	if buf[0] != addr {
		slog.Warn("ECU echo mismatch", "expected", fmt.Sprintf("0x%02X", addr), "got", fmt.Sprintf("0x%02X", buf[0]))
		e.conn.Flush()
		return 0, fmt.Errorf("echo mismatch for 0x%02X: got 0x%02X", addr, buf[0])
	}

	return buf[1], nil
}

// PollSensors queries all sensors at the given indices and returns a complete sample.
func (e *ECU) PollSensors(indices []int) (sensor.Sample, error) {
	var sample sensor.Sample
	sample.Time = time.Now()

	for _, idx := range indices {
		if idx < 0 || idx >= len(e.defs) {
			continue
		}
		def := e.defs[idx]
		if !def.Exists || def.Computed || def.Addr == 0xFF {
			continue
		}

		data, err := e.QuerySensor(def.Addr)
		if err != nil {
			slog.Debug("sensor query failed", "slug", def.Slug, "addr", fmt.Sprintf("0x%02X", def.Addr), "error", err)
			continue
		}

		sample.SetData(idx, data)
	}

	// Compute derived values (e.g., injector duty cycle)
	sample.ComputeDerivatives(e.defs)

	return sample, nil
}

// SendCommand sends a command byte to the ECU and waits for the response.
// Used for actuator tests (0xF1-0xFC) and DTC erase (0xCA).
// timeout is the maximum time to wait for the ECU to complete the action.
// Only whitelisted command addresses are accepted.
func (e *ECU) SendCommand(cmd byte, timeout time.Duration) (byte, error) {
	if !validCommandAddrs[cmd] {
		return 0, fmt.Errorf("command 0x%02X is not a known ECU command address", cmd)
	}

	e.busMu.Lock()
	defer e.busMu.Unlock()

	// Flush stale bytes before sending command
	e.conn.Flush()

	// Send command
	_, err := e.conn.Send([]byte{cmd})
	if err != nil {
		return 0, fmt.Errorf("failed to send command 0x%02X: %w", cmd, err)
	}

	// Read echo with short timeout
	buf := make([]byte, 1)
	echoDeadline := time.Now().Add(500 * time.Millisecond)
	echoRead := 0
	for echoRead < 1 && time.Now().Before(echoDeadline) {
		n, err := e.conn.Receive(buf)
		if err != nil {
			e.conn.Flush()
			return 0, fmt.Errorf("failed to read echo for command 0x%02X: %w", cmd, err)
		}
		echoRead += n
	}
	if echoRead < 1 {
		e.conn.Flush()
		return 0, fmt.Errorf("timeout reading echo for command 0x%02X", cmd)
	}
	if buf[0] != cmd {
		e.conn.Flush()
		return 0, fmt.Errorf("command echo mismatch: sent 0x%02X, got 0x%02X", cmd, buf[0])
	}

	// Read result with blocking loop (ECU responds when done, up to timeout)
	result := make([]byte, 1)
	resultDeadline := time.Now().Add(timeout)
	resultRead := 0
	for resultRead < 1 && time.Now().Before(resultDeadline) {
		n, err := e.conn.Receive(result)
		if err != nil {
			e.conn.Flush()
			return 0, fmt.Errorf("failed to read result for command 0x%02X: %w", cmd, err)
		}
		resultRead += n
	}
	if resultRead < 1 {
		e.conn.Flush()
		return 0, fmt.Errorf("timeout waiting for result of command 0x%02X", cmd)
	}

	return result[0], nil
}

// Conn returns the underlying serial connection.
func (e *ECU) Conn() *SerialConn {
	return e.conn
}
