package sensor

import (
	"testing"
)

func TestDefaultDefinitions(t *testing.T) {
	defs := DefaultDefinitions()

	if len(defs) != MaxSensors {
		t.Errorf("DefaultDefinitions() returned %d sensors, want %d", len(defs), MaxSensors)
	}

	// Check RPM sensor is at index 17
	if defs[17].Slug != "RPM" {
		t.Errorf("defs[17].Slug = %s, want RPM", defs[17].Slug)
	}
	if defs[17].Addr != 0x21 {
		t.Errorf("defs[17].Addr = 0x%02X, want 0x21", defs[17].Addr)
	}

	// Check TPS at index 14
	if defs[14].Slug != "TPS" {
		t.Errorf("defs[14].Slug = %s, want TPS", defs[14].Slug)
	}
	if defs[14].Addr != 0x17 {
		t.Errorf("defs[14].Addr = 0x%02X, want 0x17", defs[14].Addr)
	}

	// INJD should be computed
	if !defs[20].Computed {
		t.Errorf("defs[20] (INJD) should be computed")
	}
	if defs[20].Slug != "INJD" {
		t.Errorf("defs[20].Slug = %s, want INJD", defs[20].Slug)
	}
}

func TestAllPollableIndices(t *testing.T) {
	defs := DefaultDefinitions()
	indices := AllPollableIndices(defs)

	// Should not include index 0 (unused), 20 (INJD computed), or 23-31 (unused)
	for _, idx := range indices {
		if !defs[idx].Exists {
			t.Errorf("AllPollableIndices includes non-existent sensor at index %d", idx)
		}
		if defs[idx].Computed {
			t.Errorf("AllPollableIndices includes computed sensor %s at index %d", defs[idx].Slug, idx)
		}
		if defs[idx].Addr == 0xFF {
			t.Errorf("AllPollableIndices includes sensor with addr 0xFF at index %d", idx)
		}
	}

	// Should have ~21 pollable sensors (indices 1-19, 21-22)
	if len(indices) < 15 || len(indices) > 25 {
		t.Errorf("AllPollableIndices returned %d sensors, expected 15-25", len(indices))
	}
}

func TestFindBySlug(t *testing.T) {
	defs := DefaultDefinitions()

	idx, def := FindBySlug(defs, "RPM")
	if idx != 17 || def == nil {
		t.Errorf("FindBySlug(RPM) = (%d, %v), want (17, non-nil)", idx, def)
	}

	idx, def = FindBySlug(defs, "NONEXISTENT")
	if idx != -1 || def != nil {
		t.Errorf("FindBySlug(NONEXISTENT) = (%d, %v), want (-1, nil)", idx, def)
	}
}

func TestSlugsToIndices(t *testing.T) {
	defs := DefaultDefinitions()

	indices, notFound := SlugsToIndices(defs, []string{"RPM", "TPS", "FAKE"})
	if len(indices) != 2 {
		t.Errorf("SlugsToIndices returned %d indices, want 2", len(indices))
	}
	if len(notFound) != 1 || notFound[0] != "FAKE" {
		t.Errorf("SlugsToIndices notFound = %v, want [FAKE]", notFound)
	}
}

func TestSampleComputeDerivatives(t *testing.T) {
	defs := DefaultDefinitions()

	sample := Sample{}
	// RPM at index 17, raw 128 → 4000 rpm
	sample.SetData(17, 128)
	// INJP at index 19, raw 50 → 12.8ms
	sample.SetData(19, 50)

	sample.ComputeDerivatives(defs)

	// INJD at index 20 should now be set
	if !sample.HasData(20) {
		t.Error("ComputeDerivatives did not set INJD")
	}

	// INJD = (50 * 128) / 117 = 54
	expected := byte(50 * 128 / 117)
	if sample.RawData[20] != expected {
		t.Errorf("INJD raw = %d, want %d", sample.RawData[20], expected)
	}
}

func TestSampleComputeDerivativesCapped(t *testing.T) {
	defs := DefaultDefinitions()

	sample := Sample{}
	sample.SetData(17, 255) // RPM max
	sample.SetData(19, 255) // INJP max

	sample.ComputeDerivatives(defs)

	// Should be capped at 255
	if sample.RawData[20] != 255 {
		t.Errorf("INJD raw = %d, want 255 (capped)", sample.RawData[20])
	}
}
