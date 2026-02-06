package logger

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kbuckham/mmcd/internal/sensor"
)

// Native MMCD binary log format (.mmcd)
//
// Header (16 bytes):
//   [4] Magic: "MMCD"
//   [1] Version: 1
//   [1] UnitSystem: 0=metric, 1=english, 2=raw
//   [2] SensorCount: number of sensor indices stored
//   [4] SampleCount: total number of samples (updated on close)
//   [4] Reserved
//
// Sensor Index Table (SensorCount bytes):
//   Each byte is the sensor definition index (0-31) that is being logged
//
// Samples (40 bytes each, little-endian):
//   [8] UnixNano: int64 nanoseconds since Unix epoch
//   [4] DataPresent: uint32 bitmask
//   [28] Data: first 28 bytes of the 32-byte raw data array
//        (we store all 32 bytes, padded)
//
// Actually, let's keep it simple and match the original GraphSample exactly:
// Sample (48 bytes):
//   [8] UnixNano: int64
//   [4] DataPresent: uint32
//   [4] Padding
//   [32] RawData

const (
	mmcdMagic      = "MMCD"
	mmcdVersion    = 1
	mmcdHeaderSize = 16
	mmcdSampleSize = 48
)

// BinaryWriter writes sensor samples to our native .mmcd binary format.
type BinaryWriter struct {
	file        *os.File
	sampleCount uint32
}

// NewBinaryWriter creates a new .mmcd binary log file.
func NewBinaryWriter(filename string, indices []int, units sensor.UnitSystem) (*BinaryWriter, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create binary log %s: %w", filename, err)
	}

	// Write header
	header := make([]byte, mmcdHeaderSize)
	copy(header[0:4], mmcdMagic)
	header[4] = mmcdVersion
	header[5] = byte(units)
	binary.LittleEndian.PutUint16(header[6:8], uint16(len(indices)))
	binary.LittleEndian.PutUint32(header[8:12], 0) // sample count placeholder
	// header[12:16] reserved

	if _, err := f.Write(header); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to write header: %w", err)
	}

	// Write sensor index table
	indexTable := make([]byte, len(indices))
	for i, idx := range indices {
		indexTable[i] = byte(idx)
	}
	if _, err := f.Write(indexTable); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to write index table: %w", err)
	}

	return &BinaryWriter{file: f}, nil
}

// WriteSample appends a sample to the binary log.
func (bw *BinaryWriter) WriteSample(sample sensor.Sample) error {
	buf := make([]byte, mmcdSampleSize)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(sample.Time.UnixNano()))
	binary.LittleEndian.PutUint32(buf[8:12], sample.DataPresent)
	// buf[12:16] padding
	copy(buf[16:48], sample.RawData[:])

	if _, err := bw.file.Write(buf); err != nil {
		return fmt.Errorf("failed to write sample: %w", err)
	}
	bw.sampleCount++
	return nil
}

// Close finalizes the binary log, updating the sample count in the header.
func (bw *BinaryWriter) Close() error {
	// Update sample count in header
	if _, err := bw.file.Seek(8, io.SeekStart); err == nil {
		countBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(countBuf, bw.sampleCount)
		bw.file.Write(countBuf)
	}
	return bw.file.Close()
}

// Count returns the number of samples written.
func (bw *BinaryWriter) Count() uint32 {
	return bw.sampleCount
}

// BinaryLog represents a parsed .mmcd binary log.
type BinaryLog struct {
	Version     byte
	Units       sensor.UnitSystem
	Indices     []int
	SampleCount uint32
	Samples     []sensor.Sample
}

// ReadBinaryLog reads a .mmcd binary log file.
func ReadBinaryLog(filename string) (*BinaryLog, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open binary log: %w", err)
	}
	defer f.Close()

	// Read header
	header := make([]byte, mmcdHeaderSize)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	if string(header[0:4]) != mmcdMagic {
		return nil, fmt.Errorf("not an MMCD binary log (bad magic)")
	}

	log := &BinaryLog{
		Version:     header[4],
		Units:       sensor.UnitSystem(header[5]),
		SampleCount: binary.LittleEndian.Uint32(header[8:12]),
	}

	sensorCount := binary.LittleEndian.Uint16(header[6:8])

	// Read sensor index table
	indexTable := make([]byte, sensorCount)
	if _, err := io.ReadFull(f, indexTable); err != nil {
		return nil, fmt.Errorf("failed to read index table: %w", err)
	}
	log.Indices = make([]int, sensorCount)
	for i, b := range indexTable {
		log.Indices[i] = int(b)
	}

	// Read samples
	log.Samples = make([]sensor.Sample, 0, log.SampleCount)
	buf := make([]byte, mmcdSampleSize)
	for {
		_, err := io.ReadFull(f, buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read sample: %w", err)
		}

		unixNano := int64(binary.LittleEndian.Uint64(buf[0:8]))
		sample := sensor.Sample{
			Time:        time.Unix(0, unixNano),
			DataPresent: binary.LittleEndian.Uint32(buf[8:12]),
		}
		copy(sample.RawData[:], buf[16:48])
		log.Samples = append(log.Samples, sample)
	}

	return log, nil
}
