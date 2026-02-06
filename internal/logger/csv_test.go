package logger

import (
	"encoding/csv"
	"os"
	"testing"
	"time"

	"github.com/kbuckham/mmcd/internal/sensor"
)

func TestCSVWriter(t *testing.T) {
	tmp, err := os.CreateTemp("", "test-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)

	defs := sensor.DefaultDefinitions()
	indices := []int{14, 17} // TPS, RPM
	units := sensor.UnitMetric

	writer, err := NewCSVWriter(tmpName, defs, indices, units)
	if err != nil {
		t.Fatalf("NewCSVWriter failed: %v", err)
	}

	// Write two samples
	now := time.Now()

	s1 := sensor.Sample{Time: now}
	s1.SetData(14, 128) // TPS ~ 50%
	s1.SetData(17, 64)  // RPM = 31.25*64 = 2000

	s2 := sensor.Sample{Time: now.Add(500 * time.Millisecond)}
	s2.SetData(14, 200) // TPS ~ 78%
	s2.SetData(17, 128) // RPM = 4000

	if err := writer.WriteSample(s1); err != nil {
		t.Fatalf("WriteSample(s1) failed: %v", err)
	}
	if err := writer.WriteSample(s2); err != nil {
		t.Fatalf("WriteSample(s2) failed: %v", err)
	}

	if writer.Count() != 2 {
		t.Errorf("Count() = %d, want 2", writer.Count())
	}

	writer.Close()

	// Read back and verify
	f, err := os.Open(tmpName)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("CSV read failed: %v", err)
	}

	// Header + 2 data rows = 3 records
	if len(records) != 3 {
		t.Fatalf("CSV has %d rows, want 3 (header + 2 data)", len(records))
	}

	// Check header
	header := records[0]
	if header[0] != "Timestamp" || header[1] != "Elapsed_ms" {
		t.Errorf("Header[0:2] = %v, want [Timestamp, Elapsed_ms]", header[0:2])
	}

	// TPS and RPM columns should be present (slug + raw for each)
	if header[2] != "TPS" {
		t.Errorf("Header[2] = %s, want TPS", header[2])
	}
	if header[3] != "TPS_raw" {
		t.Errorf("Header[3] = %s, want TPS_raw", header[3])
	}
	if header[4] != "RPM" {
		t.Errorf("Header[4] = %s, want RPM", header[4])
	}

	// Check data row 1 elapsed time is 0
	if records[1][1] != "0" {
		t.Errorf("Row1 elapsed = %s, want '0'", records[1][1])
	}

	// Check data row 2 elapsed time is ~500ms
	if records[2][1] != "500" {
		t.Errorf("Row2 elapsed = %s, want '500'", records[2][1])
	}

	// Check raw values
	if records[1][3] != "128" {
		t.Errorf("Row1 TPS_raw = %s, want '128'", records[1][3])
	}
	if records[1][5] != "64" {
		t.Errorf("Row1 RPM_raw = %s, want '64'", records[1][5])
	}
}
