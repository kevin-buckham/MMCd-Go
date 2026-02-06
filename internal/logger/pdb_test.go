package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func findPDBFiles(t *testing.T) (string, string) {
	t.Helper()
	// Look for PDB files relative to project root
	projectRoot := filepath.Join("..", "..", "..")
	firstRun := filepath.Join(projectRoot, "docs", "2003-01-17_First_run.PDB")
	yemelya := filepath.Join(projectRoot, "docs", "2003-02-01_YEMELYA.PDB")

	if _, err := os.Stat(firstRun); err != nil {
		t.Skipf("PDB test file not found: %s (run tests from mmcd-go/)", firstRun)
	}

	return firstRun, yemelya
}

func TestParsePDB_FirstRun(t *testing.T) {
	firstRun, _ := findPDBFiles(t)

	log, err := ParsePDB(firstRun)
	if err != nil {
		t.Fatalf("ParsePDB(%s) failed: %v", firstRun, err)
	}

	if log.Name == "" {
		t.Error("PDB log name is empty")
	}
	t.Logf("Log name: %s", log.Name)

	if len(log.Samples) == 0 {
		t.Fatal("No samples parsed from First_run PDB")
	}
	t.Logf("Samples: %d", len(log.Samples))

	// First sample should have a reasonable time
	first := log.Samples[0]
	year := first.Time.Year()
	if year < 2000 || year > 2010 {
		t.Errorf("First sample year = %d, expected 2000-2010", year)
	}
	t.Logf("First sample time: %s", first.Time.Format("2006-01-02 15:04:05"))

	// Should have some data present
	if first.DataPresent == 0 {
		t.Error("First sample has no data present")
	}
	t.Logf("First sample dataPresent: 0x%08X", first.DataPresent)
}

func TestParsePDB_YEMELYA(t *testing.T) {
	_, yemelya := findPDBFiles(t)

	if _, err := os.Stat(yemelya); err != nil {
		t.Skipf("YEMELYA PDB not found: %s", yemelya)
	}

	log, err := ParsePDB(yemelya)
	if err != nil {
		t.Fatalf("ParsePDB(%s) failed: %v", yemelya, err)
	}

	t.Logf("Log name: %s", log.Name)
	t.Logf("Samples: %d", len(log.Samples))

	if len(log.Samples) == 0 {
		t.Fatal("No samples parsed from YEMELYA PDB")
	}

	// YEMELYA should have more sensors than First_run
	var presentMask uint32
	for _, s := range log.Samples {
		presentMask |= s.DataPresent
	}

	sensorCount := 0
	for i := 0; i < 32; i++ {
		if presentMask&(1<<uint(i)) != 0 {
			sensorCount++
		}
	}
	t.Logf("Unique sensors across all samples: %d (mask: 0x%08X)", sensorCount, presentMask)

	// YEMELYA data should have many sensors (we saw 16 in the hex dump)
	if sensorCount < 5 {
		t.Errorf("Expected at least 5 sensors in YEMELYA, got %d", sensorCount)
	}

	// Time range check
	first := log.Samples[0].Time
	last := log.Samples[len(log.Samples)-1].Time
	duration := last.Sub(first)
	t.Logf("Time range: %s to %s (%.1fs)",
		first.Format("2006-01-02 15:04:05"),
		last.Format("2006-01-02 15:04:05"),
		duration.Seconds())

	if duration.Seconds() < 0 {
		t.Error("Time range is negative")
	}
}

func TestParsePDB_InvalidFile(t *testing.T) {
	// Create a temp file that isn't a PDB
	tmp, err := os.CreateTemp("", "test-*.pdb")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.Write([]byte("this is not a PDB file"))
	tmp.Close()

	_, err = ParsePDB(tmp.Name())
	if err == nil {
		t.Error("Expected error parsing invalid PDB file")
	}
}

func TestParsePDB_NonExistentFile(t *testing.T) {
	_, err := ParsePDB("/nonexistent/file.pdb")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}
