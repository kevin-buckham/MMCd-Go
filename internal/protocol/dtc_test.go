package protocol

import (
	"testing"
)

func TestDecodeDTCs_NoCodes(t *testing.T) {
	codes := decodeDTCs(0x0000)
	if len(codes) != 0 {
		t.Errorf("decodeDTCs(0x0000) returned %d codes, want 0", len(codes))
	}
}

func TestDecodeDTCs_AllCodes(t *testing.T) {
	codes := decodeDTCs(0xFFFF)
	if len(codes) != 16 {
		t.Errorf("decodeDTCs(0xFFFF) returned %d codes, want 16", len(codes))
	}
}

func TestDecodeDTCs_SingleBit(t *testing.T) {
	// Bit 0 = Code 11 (Oxygen sensor)
	codes := decodeDTCs(0x0001)
	if len(codes) != 1 {
		t.Fatalf("decodeDTCs(0x0001) returned %d codes, want 1", len(codes))
	}
	if codes[0].Code != "11" {
		t.Errorf("codes[0].Code = %s, want '11'", codes[0].Code)
	}
	if codes[0].Description != "Oxygen sensor" {
		t.Errorf("codes[0].Description = %s, want 'Oxygen sensor'", codes[0].Description)
	}
}

func TestDecodeDTCs_MultipleBits(t *testing.T) {
	// Bits 5 + 10 = Code 21 (coolant temp) + Code 31 (knock)
	bitmap := uint16(1<<5 | 1<<10)
	codes := decodeDTCs(bitmap)
	if len(codes) != 2 {
		t.Fatalf("decodeDTCs(0x%04X) returned %d codes, want 2", bitmap, len(codes))
	}

	foundCoolant := false
	foundKnock := false
	for _, c := range codes {
		if c.Code == "21" && c.Description == "Engine coolant temperature sensor" {
			foundCoolant = true
		}
		if c.Code == "31" && c.Description == "Knock sensor" {
			foundKnock = true
		}
	}
	if !foundCoolant {
		t.Error("Missing code 21 (coolant temp)")
	}
	if !foundKnock {
		t.Error("Missing code 31 (knock)")
	}
}

func TestGetDTCTable(t *testing.T) {
	table := GetDTCTable()
	if len(table) != 16 {
		t.Errorf("DTC table has %d entries, want 16", len(table))
	}

	// Verify first and last entries
	if table[0].Code != "11" {
		t.Errorf("table[0].Code = %s, want '11'", table[0].Code)
	}
	if table[15].Code != "36" {
		t.Errorf("table[15].Code = %s, want '36'", table[15].Code)
	}
}
