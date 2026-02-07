package logger

import (
	"encoding/csv"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kbuckham/mmcd/internal/sensor"
)

// CSVWriter writes sensor samples to a CSV file.
type CSVWriter struct {
	mu        sync.Mutex
	file      *os.File
	writer    *csv.Writer
	defs      []sensor.Definition
	indices   []int
	units     sensor.UnitSystem
	count     int
	startTime time.Time
}

// NewCSVWriter creates a new CSV writer. It writes the header row immediately.
func NewCSVWriter(filename string, defs []sensor.Definition, indices []int, units sensor.UnitSystem) (*CSVWriter, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSV file %s: %w", filename, err)
	}

	w := csv.NewWriter(f)

	// Write header row
	header := []string{"Timestamp", "Elapsed_ms"}
	for _, idx := range indices {
		if idx >= 0 && idx < len(defs) && defs[idx].Exists {
			header = append(header, defs[idx].Slug)
			header = append(header, defs[idx].Slug+"_raw")
		}
	}
	if err := w.Write(header); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}
	w.Flush()

	return &CSVWriter{
		file:    f,
		writer:  w,
		defs:    defs,
		indices: indices,
		units:   units,
	}, nil
}

// WriteSample writes a single sensor sample as a CSV row.
func (cw *CSVWriter) WriteSample(sample sensor.Sample) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.count == 0 {
		cw.startTime = sample.Time
	}

	elapsed := sample.Time.Sub(cw.startTime).Milliseconds()

	row := []string{
		sample.Time.Format("2006-01-02T15:04:05.000"),
		fmt.Sprintf("%d", elapsed),
	}

	for _, idx := range cw.indices {
		if idx >= 0 && idx < len(cw.defs) && cw.defs[idx].Exists {
			if sample.HasData(idx) {
				row = append(row, cw.defs[idx].Format(sample.RawData[idx], cw.units))
				row = append(row, fmt.Sprintf("%d", sample.RawData[idx]))
			} else {
				row = append(row, "")
				row = append(row, "")
			}
		}
	}

	if err := cw.writer.Write(row); err != nil {
		return fmt.Errorf("failed to write CSV row: %w", err)
	}

	cw.count++

	// Flush every write for crash safety
	cw.writer.Flush()
	if err := cw.writer.Error(); err != nil {
		return fmt.Errorf("CSV flush error: %w", err)
	}

	return nil
}

// Close flushes and closes the CSV file.
func (cw *CSVWriter) Close() error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	cw.writer.Flush()
	if err := cw.writer.Error(); err != nil {
		cw.file.Close()
		return fmt.Errorf("CSV flush error: %w", err)
	}
	return cw.file.Close()
}

// Count returns the number of samples written.
func (cw *CSVWriter) Count() int {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	return cw.count
}
