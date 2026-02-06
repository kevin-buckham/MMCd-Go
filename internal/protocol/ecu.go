package protocol

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/kbuckham/mmcd/internal/sensor"
)

// ECU handles communication with the Mitsubishi OBDI ECU.
type ECU struct {
	conn *SerialConn
	defs []sensor.Definition
}

// NewECU creates a new ECU communicator.
func NewECU(conn *SerialConn, defs []sensor.Definition) *ECU {
	return &ECU{
		conn: conn,
		defs: defs,
	}
}

// QuerySensor sends a sensor address to the ECU and reads the response.
// The ECU echoes the address byte back, followed by the data byte.
// Returns the raw data byte.
func (e *ECU) QuerySensor(addr byte) (byte, error) {
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
			return 0, fmt.Errorf("failed to read response for 0x%02X: %w", addr, err)
		}
		totalRead += n
	}

	if totalRead < 2 {
		return 0, fmt.Errorf("timeout reading response for 0x%02X: got %d bytes", addr, totalRead)
	}

	// Verify echo
	if buf[0] != addr {
		slog.Warn("ECU echo mismatch", "expected", fmt.Sprintf("0x%02X", addr), "got", fmt.Sprintf("0x%02X", buf[0]))
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
// timeout is how long to wait for the ECU to complete the action.
func (e *ECU) SendCommand(cmd byte, timeout time.Duration) (byte, error) {
	// Send command
	_, err := e.conn.Send([]byte{cmd})
	if err != nil {
		return 0, fmt.Errorf("failed to send command 0x%02X: %w", cmd, err)
	}

	// Read echo
	buf := make([]byte, 1)
	n, err := e.conn.Receive(buf)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("failed to read echo for command 0x%02X: %w", cmd, err)
	}
	if buf[0] != cmd {
		return 0, fmt.Errorf("command echo mismatch: sent 0x%02X, got 0x%02X", cmd, buf[0])
	}

	// Wait for result (ECU takes up to ~6 seconds for actuator tests)
	time.Sleep(timeout)

	result := make([]byte, 1)
	n, err = e.conn.Receive(result)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("failed to read result for command 0x%02X: %w", cmd, err)
	}

	return result[0], nil
}

// Conn returns the underlying serial connection.
func (e *ECU) Conn() *SerialConn {
	return e.conn
}
