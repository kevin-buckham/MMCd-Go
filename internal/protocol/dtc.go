package protocol

import (
	"fmt"
	"time"
)

// DTC address constants from the ECU.
const (
	AddrActiveDTCLow  byte = 0x38
	AddrActiveDTCHigh byte = 0x39
	AddrStoredDTCLow  byte = 0x3B
	AddrStoredDTCHigh byte = 0x3C
	AddrEraseDTC      byte = 0xCA
)

// DTCCode represents a single diagnostic trouble code.
type DTCCode struct {
	Bit         int    `json:"bit"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

// DTCResult holds the results of a DTC read operation.
type DTCResult struct {
	ActiveRaw  uint16    `json:"activeRaw"`
	StoredRaw  uint16    `json:"storedRaw"`
	Active     []DTCCode `json:"active"`
	Stored     []DTCCode `json:"stored"`
}

// dtcTable maps bit positions to fault codes and descriptions.
// From the MMCD protocol documentation.
var dtcTable = [16]DTCCode{
	{Bit: 0, Code: "11", Description: "Oxygen sensor"},
	{Bit: 1, Code: "12", Description: "Intake air flow sensor"},
	{Bit: 2, Code: "13", Description: "Intake air temperature sensor"},
	{Bit: 3, Code: "14", Description: "Throttle position sensor"},
	{Bit: 4, Code: "15", Description: "ISC motor position sensor"},
	{Bit: 5, Code: "21", Description: "Engine coolant temperature sensor"},
	{Bit: 6, Code: "22", Description: "Engine speed sensor"},
	{Bit: 7, Code: "23", Description: "TDC sensor"},
	{Bit: 8, Code: "24", Description: "Vehicle speed sensor"},
	{Bit: 9, Code: "25", Description: "Barometric pressure sensor"},
	{Bit: 10, Code: "31", Description: "Knock sensor"},
	{Bit: 11, Code: "41", Description: "Injector circuit"},
	{Bit: 12, Code: "42", Description: "Fuel pump relay"},
	{Bit: 13, Code: "43", Description: "EGR"},
	{Bit: 14, Code: "44", Description: "Ignition coil"},
	{Bit: 15, Code: "36", Description: "Ignition circuit"},
}

// decodeDTCs converts a 16-bit fault bitmap into a list of DTCCode entries.
func decodeDTCs(bitmap uint16) []DTCCode {
	var codes []DTCCode
	for i := 0; i < 16; i++ {
		if bitmap&(1<<uint(i)) != 0 {
			codes = append(codes, dtcTable[i])
		}
	}
	return codes
}

// ReadDTCs reads both active and stored diagnostic trouble codes from the ECU.
func (e *ECU) ReadDTCs() (*DTCResult, error) {
	if !e.conn.IsOpen() {
		if err := e.conn.Open(); err != nil {
			return nil, err
		}
		defer e.conn.Close()
	}

	result := &DTCResult{}

	// Read active DTCs (low byte)
	activeLow, err := e.QuerySensor(AddrActiveDTCLow)
	if err != nil {
		return nil, fmt.Errorf("failed to read active DTC low byte: %w", err)
	}

	// Read active DTCs (high byte)
	activeHigh, err := e.QuerySensor(AddrActiveDTCHigh)
	if err != nil {
		return nil, fmt.Errorf("failed to read active DTC high byte: %w", err)
	}

	result.ActiveRaw = uint16(activeLow) | (uint16(activeHigh) << 8)
	result.Active = decodeDTCs(result.ActiveRaw)

	// Read stored DTCs (low byte)
	storedLow, err := e.QuerySensor(AddrStoredDTCLow)
	if err != nil {
		return nil, fmt.Errorf("failed to read stored DTC low byte: %w", err)
	}

	// Read stored DTCs (high byte)
	storedHigh, err := e.QuerySensor(AddrStoredDTCHigh)
	if err != nil {
		return nil, fmt.Errorf("failed to read stored DTC high byte: %w", err)
	}

	result.StoredRaw = uint16(storedLow) | (uint16(storedHigh) << 8)
	result.Stored = decodeDTCs(result.StoredRaw)

	return result, nil
}

// EraseDTCs sends the erase command to clear all stored fault codes.
func (e *ECU) EraseDTCs() error {
	if !e.conn.IsOpen() {
		if err := e.conn.Open(); err != nil {
			return err
		}
		defer e.conn.Close()
	}

	result, err := e.SendCommand(AddrEraseDTC, 1*time.Second)
	if err != nil {
		return fmt.Errorf("failed to erase DTCs: %w", err)
	}

	if result != 0x00 {
		return fmt.Errorf("unexpected erase DTC response: 0x%02X", result)
	}

	return nil
}

// GetDTCTable returns the full DTC lookup table for display purposes.
func GetDTCTable() [16]DTCCode {
	return dtcTable
}
