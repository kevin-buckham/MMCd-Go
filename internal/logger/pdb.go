package logger

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kbuckham/mmcd/internal/sensor"
)

// PalmOS epoch: January 1, 1904 00:00:00 UTC
// Offset from Unix epoch (January 1, 1970) in seconds.
const palmOSEpochOffset = 2082844800

// pdbHeader is the 78-byte PalmOS PDB file header.
type pdbHeader struct {
	Name             [32]byte
	Attributes       uint16
	Version          uint16
	CreationDate     uint32
	ModificationDate uint32
	LastBackupDate   uint32
	ModificationNum  uint32
	AppInfoOffset    uint32
	SortInfoOffset   uint32
	Type             [4]byte
	Creator          [4]byte
	UniqueIDSeed     uint32
	NextRecordListID uint32
	NumRecords       uint16
}

// pdbRecordEntry is an 8-byte record index entry.
type pdbRecordEntry struct {
	DataOffset uint32
	Attributes uint8
	UniqueID   [3]byte
}

// graphSampleRaw is the raw 40-byte GraphSample struct from the PalmOS app.
// All fields are big-endian (Motorola 68K).
type graphSampleRaw struct {
	Time        uint32
	DataPresent uint32
	Data        [32]byte
}

const graphSampleSize = 40

// PDBLog represents a parsed PalmOS MMCD log file.
type PDBLog struct {
	Name    string
	Samples []sensor.Sample
}

// ParsePDB reads a PalmOS PDB file from the old MMCd logger and returns
// the log name and all sensor samples contained in it.
//
// PDB format:
//   - 78-byte header (name, type="strm", creator="MMCd")
//   - N × 8-byte record index entries
//   - Record data: each record starts with 8-byte DBLK header,
//     followed by packed 40-byte GraphSample structs (big-endian)
func ParsePDB(filename string) (*PDBLog, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDB file: %w", err)
	}
	defer f.Close()

	// Read header
	var hdr pdbHeader
	if err := binary.Read(f, binary.BigEndian, &hdr); err != nil {
		return nil, fmt.Errorf("failed to read PDB header: %w", err)
	}

	// Validate type and creator
	typeStr := string(hdr.Type[:])
	creatorStr := string(hdr.Creator[:])
	if typeStr != "strm" || creatorStr != "MMCd" {
		return nil, fmt.Errorf("not an MMCd log file: type=%q creator=%q", typeStr, creatorStr)
	}

	// Extract null-terminated name
	nameBytes := hdr.Name[:]
	nameEnd := 0
	for i, b := range nameBytes {
		if b == 0 {
			nameEnd = i
			break
		}
	}
	if nameEnd == 0 {
		nameEnd = len(nameBytes)
	}

	log := &PDBLog{
		Name: string(nameBytes[:nameEnd]),
	}

	// Read record index entries
	records := make([]pdbRecordEntry, hdr.NumRecords)
	for i := 0; i < int(hdr.NumRecords); i++ {
		if err := binary.Read(f, binary.BigEndian, &records[i]); err != nil {
			return nil, fmt.Errorf("failed to read record index entry %d: %w", i, err)
		}
	}

	if len(records) == 0 {
		return log, nil
	}

	// Read each record and extract GraphSamples
	fileInfo, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := fileInfo.Size()

	for i, rec := range records {
		// Calculate record data size
		var recordEnd int64
		if i+1 < len(records) {
			recordEnd = int64(records[i+1].DataOffset)
		} else {
			recordEnd = fileSize
		}

		recordStart := int64(rec.DataOffset)
		if recordStart >= fileSize || recordStart >= recordEnd {
			continue
		}

		// Seek to record data
		if _, err := f.Seek(recordStart, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek to record %d: %w", i, err)
		}

		// Read DBLK header (8 bytes: 4 magic + 4 size)
		var magic [4]byte
		var blockSize uint32
		if err := binary.Read(f, binary.BigEndian, &magic); err != nil {
			continue // skip malformed records
		}
		if err := binary.Read(f, binary.BigEndian, &blockSize); err != nil {
			continue
		}

		if string(magic[:]) != "DBLK" {
			continue // not a data block
		}

		// Read GraphSamples from the remaining record data
		dataLen := recordEnd - recordStart - 8 // subtract DBLK header
		numSamples := int(dataLen) / graphSampleSize

		for j := 0; j < numSamples; j++ {
			var raw graphSampleRaw
			if err := binary.Read(f, binary.BigEndian, &raw); err != nil {
				break // end of record or read error
			}

			// Skip empty samples (time == 0 or no data present)
			if raw.Time == 0 || raw.DataPresent == 0 {
				continue
			}

			// Skip garbage samples: all bits set in dataPresent is impossible
			// (sensor slots 23-31 don't exist in the original app)
			if raw.DataPresent == 0xFFFFFFFF {
				continue
			}

			// Convert PalmOS time to Go time and validate range.
			// These vehicles and PalmOS devices were used ~2000-2010.
			unixSec := int64(raw.Time) - palmOSEpochOffset
			sampleTime := time.Unix(unixSec, 0)
			year := sampleTime.Year()
			if year < 1995 || year > 2030 {
				continue // garbage timestamp from uninitialized PDB memory
			}

			sample := sensor.Sample{
				Time:        sampleTime,
				DataPresent: raw.DataPresent,
				RawData:     raw.Data,
			}

			log.Samples = append(log.Samples, sample)
		}
	}

	return log, nil
}

// PDBToCSV converts a parsed PDB log to CSV format.
func PDBToCSV(pdbLog *PDBLog, outputFile string, defs []sensor.Definition, units sensor.UnitSystem) error {
	if len(pdbLog.Samples) == 0 {
		return fmt.Errorf("no samples in PDB log")
	}

	// Determine which sensor indices have any data across all samples
	var presentMask uint32
	for _, s := range pdbLog.Samples {
		presentMask |= s.DataPresent
	}

	var indices []int
	for i := 0; i < sensor.MaxSensors; i++ {
		if presentMask&(1<<uint(i)) != 0 {
			indices = append(indices, i)
		}
	}

	// Also add INJD if RPM and INJP are present
	hasRPM := presentMask&(1<<17) != 0
	hasINJP := presentMask&(1<<19) != 0
	if hasRPM && hasINJP {
		indices = append(indices, 20) // INJD
	}

	writer, err := NewCSVWriter(outputFile, defs, indices, units)
	if err != nil {
		return err
	}
	defer writer.Close()

	for i := range pdbLog.Samples {
		// Compute derivatives (INJD) for each sample
		pdbLog.Samples[i].ComputeDerivatives(defs)
		if err := writer.WriteSample(pdbLog.Samples[i]); err != nil {
			return fmt.Errorf("failed to write sample %d: %w", i, err)
		}
	}

	return nil
}
