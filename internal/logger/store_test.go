package logger

import (
	"os"
	"testing"
	"time"

	"github.com/kbuckham/mmcd/internal/sensor"
)

func TestBinaryLogRoundTrip(t *testing.T) {
	tmp, err := os.CreateTemp("", "test-*.mmcd")
	if err != nil {
		t.Fatal(err)
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)

	defs := sensor.DefaultDefinitions()
	indices := []int{4, 14, 17, 19} // COOL, TPS, RPM, INJP
	units := sensor.UnitMetric

	// Write samples
	writer, err := NewBinaryWriter(tmpName, indices, units)
	if err != nil {
		t.Fatalf("NewBinaryWriter failed: %v", err)
	}

	now := time.Now()
	samples := []sensor.Sample{
		{Time: now, DataPresent: 0x00060010, RawData: func() [32]byte {
			var d [32]byte
			d[4] = 0x80  // COOL raw
			d[17] = 0x40 // RPM raw = 64 → 2000rpm
			d[19] = 0x20 // INJP raw
			return d
		}()},
		{Time: now.Add(100 * time.Millisecond), DataPresent: 0x00064010, RawData: func() [32]byte {
			var d [32]byte
			d[4] = 0x82
			d[14] = 0x80 // TPS raw
			d[17] = 0x42
			d[19] = 0x22
			return d
		}()},
	}

	for _, s := range samples {
		s.ComputeDerivatives(defs)
		if err := writer.WriteSample(s); err != nil {
			t.Fatalf("WriteSample failed: %v", err)
		}
	}

	if writer.Count() != 2 {
		t.Errorf("Count() = %d, want 2", writer.Count())
	}
	writer.Close()

	// Read back
	log, err := ReadBinaryLog(tmpName)
	if err != nil {
		t.Fatalf("ReadBinaryLog failed: %v", err)
	}

	if log.Version != mmcdVersion {
		t.Errorf("Version = %d, want %d", log.Version, mmcdVersion)
	}

	if log.Units != units {
		t.Errorf("Units = %d, want %d", log.Units, units)
	}

	if len(log.Indices) != len(indices) {
		t.Errorf("len(Indices) = %d, want %d", len(log.Indices), len(indices))
	}

	if log.SampleCount != 2 {
		t.Errorf("SampleCount = %d, want 2", log.SampleCount)
	}

	if len(log.Samples) != 2 {
		t.Fatalf("len(Samples) = %d, want 2", len(log.Samples))
	}

	// Verify first sample data
	s := log.Samples[0]
	if s.RawData[17] != 0x40 {
		t.Errorf("Sample[0].RawData[17] = 0x%02X, want 0x40", s.RawData[17])
	}
	if s.RawData[4] != 0x80 {
		t.Errorf("Sample[0].RawData[4] = 0x%02X, want 0x80", s.RawData[4])
	}

	// Verify time is preserved (within 1ms due to nanosecond truncation)
	timeDiff := s.Time.Sub(now)
	if timeDiff < -time.Millisecond || timeDiff > time.Millisecond {
		t.Errorf("Sample[0] time drift = %v", timeDiff)
	}
}

func TestBinaryLogInvalidFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "test-bad-*.mmcd")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Write([]byte("not a valid mmcd file"))
	tmp.Close()
	defer os.Remove(tmp.Name())

	_, err = ReadBinaryLog(tmp.Name())
	if err == nil {
		t.Error("Expected error reading invalid binary log")
	}
}
