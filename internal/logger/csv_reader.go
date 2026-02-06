package logger

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CSVLog represents a parsed CSV log file for graph display.
type CSVLog struct {
	Slugs     []string             // sensor slugs found in the header
	Data      map[string][]float64 // slug -> array of converted float values
	ElapsedMs []float64            // elapsed milliseconds per row (from Elapsed_ms column)
	Count     int                  // number of data rows
}

// ReadCSVLog reads a CSV log file produced by mmcd and returns the converted
// (non-raw) columns as float arrays keyed by slug.
func ReadCSVLog(filename string) (*CSVLog, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV has no data rows")
	}

	header := records[0]

	// Identify converted value columns (not Timestamp, Elapsed_ms, or *_raw)
	type colInfo struct {
		slug string
		col  int
	}
	var cols []colInfo
	elapsedCol := -1
	for i, h := range header {
		if h == "Elapsed_ms" {
			elapsedCol = i
			continue
		}
		if h == "Timestamp" || h == "" {
			continue
		}
		if strings.HasSuffix(h, "_raw") {
			continue
		}
		cols = append(cols, colInfo{slug: h, col: i})
	}

	if len(cols) == 0 {
		return nil, fmt.Errorf("no sensor columns found in CSV header")
	}

	slugs := make([]string, len(cols))
	data := make(map[string][]float64, len(cols))
	for i, c := range cols {
		slugs[i] = c.slug
		data[c.slug] = make([]float64, 0, len(records)-1)
	}

	var elapsedMs []float64
	if elapsedCol >= 0 {
		elapsedMs = make([]float64, 0, len(records)-1)
	}

	rowCount := 0
	for _, row := range records[1:] {
		// Parse elapsed time
		if elapsedCol >= 0 && elapsedCol < len(row) {
			ms, _ := strconv.ParseFloat(row[elapsedCol], 64)
			elapsedMs = append(elapsedMs, ms)
		}

		for _, c := range cols {
			if c.col >= len(row) || row[c.col] == "" {
				data[c.slug] = append(data[c.slug], 0)
				continue
			}
			val, err := strconv.ParseFloat(row[c.col], 64)
			if err != nil {
				data[c.slug] = append(data[c.slug], 0)
				continue
			}
			data[c.slug] = append(data[c.slug], val)
		}
		rowCount++
	}

	return &CSVLog{
		Slugs:     slugs,
		Data:      data,
		ElapsedMs: elapsedMs,
		Count:     rowCount,
	}, nil
}
