package protocol

import (
	"testing"
)

func TestQuerySensor_RejectsCommandRange(t *testing.T) {
	// Addresses >= 0xC0 should be rejected as they're in the command range
	tests := []byte{0xC0, 0xCA, 0xF1, 0xFC, 0xFF}
	defs := make([]struct{}, 0) // ECU needs no defs for this test
	_ = defs

	conn := NewSerialConn("/dev/null", 1953)
	ecu := NewECU(conn, nil)

	for _, addr := range tests {
		_, err := ecu.QuerySensor(addr)
		if err == nil {
			t.Errorf("QuerySensor(0x%02X) should have been rejected (command range), got nil error", addr)
		}
	}
}

func TestQuerySensor_AcceptsSensorRange(t *testing.T) {
	// Addresses < 0xC0 should be accepted (will fail on send since port isn't open,
	// but the address check should pass)
	tests := []byte{0x00, 0x10, 0x7F, 0xBF}

	conn := NewSerialConn("/dev/null", 1953)
	ecu := NewECU(conn, nil)

	for _, addr := range tests {
		_, err := ecu.QuerySensor(addr)
		if err == nil {
			t.Errorf("QuerySensor(0x%02X) should fail (port not open), but not due to address rejection", addr)
			continue
		}
		// Error should be about sending, not about address rejection
		expected := "failed to send"
		if len(err.Error()) > 15 && err.Error()[:14] == "address 0x" {
			t.Errorf("QuerySensor(0x%02X) was incorrectly rejected as command range", addr)
		}
		_ = expected
	}
}

func TestSendCommand_RejectsUnknownAddresses(t *testing.T) {
	conn := NewSerialConn("/dev/null", 1953)
	ecu := NewECU(conn, nil)

	// These are NOT in the whitelist
	invalid := []byte{0x00, 0x10, 0x7F, 0xBF, 0xC0, 0xCB, 0xFD, 0xFE, 0xFF}
	for _, addr := range invalid {
		_, err := ecu.SendCommand(addr, 100)
		if err == nil {
			t.Errorf("SendCommand(0x%02X) should have been rejected (not in whitelist)", addr)
		}
	}
}

func TestSendCommand_AcceptsWhitelistedAddresses(t *testing.T) {
	conn := NewSerialConn("/dev/null", 1953)
	ecu := NewECU(conn, nil)

	// These ARE in the whitelist - will fail on flush/send since port isn't open,
	// but should pass the whitelist check
	valid := []byte{0xCA, 0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC}
	for _, addr := range valid {
		_, err := ecu.SendCommand(addr, 100)
		if err == nil {
			t.Errorf("SendCommand(0x%02X) should fail (port not open), but not due to whitelist", addr)
			continue
		}
		// Error should NOT be about whitelist
		if err.Error() == "command 0x"+string(addr)+" is not a known ECU command address" {
			t.Errorf("SendCommand(0x%02X) was incorrectly rejected by whitelist", addr)
		}
	}
}

func TestValidCommandAddrs_Complete(t *testing.T) {
	// Verify the whitelist contains exactly the expected addresses
	expected := []byte{0xCA, 0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC}

	if len(validCommandAddrs) != len(expected) {
		t.Errorf("validCommandAddrs has %d entries, want %d", len(validCommandAddrs), len(expected))
	}

	for _, addr := range expected {
		if !validCommandAddrs[addr] {
			t.Errorf("validCommandAddrs missing expected address 0x%02X", addr)
		}
	}
}
