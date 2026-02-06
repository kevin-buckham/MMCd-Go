//go:build !cli

package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/kbuckham/mmcd/internal/logger"
	"github.com/kbuckham/mmcd/internal/protocol"
	"github.com/kbuckham/mmcd/internal/sensor"
	"github.com/kbuckham/mmcd/internal/version"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds the application state and is bound to the Wails frontend.
type App struct {
	ctx context.Context

	mu            sync.Mutex
	defs          []sensor.Definition
	conn          *protocol.SerialConn
	ecu           *protocol.ECU
	sim           *protocol.Simulator
	lg            *logger.Logger
	csvWriter     *logger.CSVWriter
	units         sensor.UnitSystem
	activeIndices []int
	connected     bool
	demoMode      bool
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{
		defs:  sensor.DefaultDefinitions(),
		units: sensor.UnitMetric,
	}
}

// startup is called when the app starts. The context is saved for runtime calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	slog.Info("MMCD app started")
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	a.Disconnect()
	slog.Info("MMCD app shutdown")
}

// --- Methods exposed to the Svelte frontend via Wails bindings ---

// ListSerialPorts returns available serial ports on the system.
func (a *App) ListSerialPorts() ([]string, error) {
	return protocol.ListPorts()
}

// Connect opens a serial connection to the ECU.
func (a *App) Connect(port string, baud int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.connected {
		return fmt.Errorf("already connected")
	}

	if baud <= 0 {
		baud = protocol.DefaultBaudRate
	}

	a.conn = protocol.NewSerialConn(port, baud)
	if err := a.conn.Open(); err != nil {
		return err
	}

	a.ecu = protocol.NewECU(a.conn, a.defs)
	a.connected = true

	runtime.EventsEmit(a.ctx, "connection:status", map[string]interface{}{
		"connected": true,
		"port":      port,
		"baud":      baud,
	})

	slog.Info("connected to ECU", "port", port, "baud", baud)
	return nil
}

// ConnectDemo starts a simulated ECU connection for UI development.
func (a *App) ConnectDemo() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.connected {
		return fmt.Errorf("already connected")
	}

	a.sim = protocol.NewSimulator(a.defs)
	a.connected = true
	a.demoMode = true

	runtime.EventsEmit(a.ctx, "connection:status", map[string]interface{}{
		"connected": true,
		"port":      "DEMO",
		"baud":      0,
		"demo":      true,
	})

	slog.Info("connected in DEMO mode (simulated ECU)")
	return nil
}

// IsDemoMode returns whether the app is in demo/simulator mode.
func (a *App) IsDemoMode() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.demoMode
}

// Disconnect closes the serial connection.
func (a *App) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.lg != nil && a.lg.IsRunning() {
		a.lg.Stop()
	}

	if a.csvWriter != nil {
		a.csvWriter.Close()
		a.csvWriter = nil
	}

	if a.conn != nil {
		a.conn.Close()
	}

	a.connected = false
	a.demoMode = false
	a.ecu = nil
	a.sim = nil
	a.conn = nil
	a.lg = nil

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "connection:status", map[string]interface{}{
			"connected": false,
		})
	}

	return nil
}

// IsConnected returns the connection status.
func (a *App) IsConnected() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.connected
}

// GetSensorDefinitions returns all sensor definitions for the UI.
func (a *App) GetSensorDefinitions() []sensor.Definition {
	return a.defs
}

// SetActiveSensors sets which sensors to poll by their slugs.
func (a *App) SetActiveSensors(slugs []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	upperSlugs := make([]string, len(slugs))
	for i, s := range slugs {
		upperSlugs[i] = strings.ToUpper(s)
	}

	indices, notFound := sensor.SlugsToIndices(a.defs, upperSlugs)
	if len(notFound) > 0 {
		slog.Warn("unknown sensors", "slugs", notFound)
	}

	// Add INJD if RPM and INJP are both selected
	hasRPM, hasINJP := false, false
	for _, idx := range indices {
		if a.defs[idx].Slug == "RPM" {
			hasRPM = true
		}
		if a.defs[idx].Slug == "INJP" {
			hasINJP = true
		}
	}
	if hasRPM && hasINJP {
		if injdIdx, _ := sensor.FindBySlug(a.defs, "INJD"); injdIdx >= 0 {
			indices = append(indices, injdIdx)
		}
	}

	a.activeIndices = indices

	if a.lg != nil {
		a.lg.SetIndices(indices)
	}

	return nil
}

// StartMonitoring begins polling the ECU and emitting samples to the frontend.
func (a *App) StartMonitoring() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.connected {
		return fmt.Errorf("not connected")
	}

	if a.lg != nil && a.lg.IsRunning() {
		return nil // already running
	}

	indices := a.activeIndices
	if len(indices) == 0 {
		indices = sensor.AllPollableIndices(a.defs)
		a.activeIndices = indices
	}

	var poller logger.SamplePoller
	var pollRate time.Duration
	if a.demoMode {
		poller = a.sim
		pollRate = 50 * time.Millisecond // 20Hz for smooth UI updates
	} else {
		poller = a.ecu
		pollRate = 1 * time.Millisecond // as fast as possible for real ECU
	}
	a.lg = logger.NewWithRate(poller, a.defs, indices, a.units, pollRate)

	a.lg.OnSample(func(sample sensor.Sample) {
		// Emit sample to frontend
		values := sample.ConvertedValues(a.defs, a.units)
		floats := sample.ConvertedFloats(a.defs, a.units)
		runtime.EventsEmit(a.ctx, "sensor:sample", map[string]interface{}{
			"time":        sample.Time.Format(time.RFC3339Nano),
			"values":      values,
			"floats":      floats,
			"rawData":     sample.RawData,
			"dataPresent": sample.DataPresent,
		})
	})

	return a.lg.Start()
}

// StopMonitoring stops the polling loop.
func (a *App) StopMonitoring() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.lg != nil {
		a.lg.Stop()
	}
}

// StartLogging begins writing samples to a CSV file.
func (a *App) StartLogging(filename string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.csvWriter != nil {
		return fmt.Errorf("already logging")
	}

	indices := a.activeIndices
	if len(indices) == 0 {
		indices = sensor.AllPollableIndices(a.defs)
	}

	var err error
	a.csvWriter, err = logger.NewCSVWriter(filename, a.defs, indices, a.units)
	if err != nil {
		return err
	}

	if a.lg != nil {
		a.lg.OnSample(func(sample sensor.Sample) {
			if a.csvWriter != nil {
				a.csvWriter.WriteSample(sample)
			}
		})
	}

	runtime.EventsEmit(a.ctx, "logging:status", map[string]interface{}{
		"logging":  true,
		"filename": filename,
	})

	return nil
}

// StopLogging stops CSV logging.
func (a *App) StopLogging() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.csvWriter == nil {
		return nil
	}

	count := a.csvWriter.Count()
	err := a.csvWriter.Close()
	a.csvWriter = nil

	runtime.EventsEmit(a.ctx, "logging:status", map[string]interface{}{
		"logging": false,
		"count":   count,
	})

	return err
}

// ReadDTCs reads diagnostic trouble codes from the ECU.
func (a *App) ReadDTCs() (*protocol.DTCResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.connected {
		return nil, fmt.Errorf("not connected")
	}

	return a.ecu.ReadDTCs()
}

// EraseDTCs clears stored fault codes.
func (a *App) EraseDTCs() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.connected {
		return fmt.Errorf("not connected")
	}

	return a.ecu.EraseDTCs()
}

// RunActuatorTest sends an actuator test command.
func (a *App) RunActuatorTest(command string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.connected {
		return "", fmt.Errorf("not connected")
	}

	commands := map[string]byte{
		"fuel-pump": 0xF6,
		"purge":     0xF5,
		"pressure":  0xF4,
		"egr":       0xF3,
		"mvic":      0xF2,
		"boost":     0xF1,
		"inj1":      0xFC,
		"inj2":      0xFB,
		"inj3":      0xFA,
		"inj4":      0xF9,
		"inj5":      0xF8,
		"inj6":      0xF7,
	}

	addr, ok := commands[strings.ToLower(command)]
	if !ok {
		return "", fmt.Errorf("unknown command: %s", command)
	}

	result, err := a.ecu.SendCommand(addr, 7*time.Second)
	if err != nil {
		return "", err
	}

	if result == 0x00 {
		return "OK", nil
	} else if result == 0xFF {
		return "Engine running (solenoid commands require engine OFF)", nil
	}
	return fmt.Sprintf("Response: 0x%02X", result), nil
}

// AboutInfo holds application metadata for the frontend.
type AboutInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Developers  string `json:"developers"`
	Copyright   string `json:"copyright"`
	License     string `json:"license"`
	Attribution string `json:"attribution"`
	URL         string `json:"url"`
}

// GetAboutInfo returns application version and attribution info.
func (a *App) GetAboutInfo() *AboutInfo {
	return &AboutInfo{
		Name:        version.Name,
		Version:     version.Version,
		Description: version.Description,
		Developers:  version.Developers,
		Copyright:   version.Copyright,
		License:     version.License,
		Attribution: version.Attribution,
		URL:         version.URL,
	}
}

// SetUnits changes the unit system.
func (a *App) SetUnits(units string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.units = sensor.ParseUnitSystem(units)
}

// LogData is the structure returned to the frontend for graph display.
type LogData struct {
	Slugs     []string             `json:"slugs"`
	Data      map[string][]float64 `json:"data"`
	ElapsedMs []float64            `json:"elapsedMs"` // elapsed milliseconds from start per sample
	Count     int                  `json:"count"`
	Name      string               `json:"name"`
}

// LoadLogFile opens a file dialog to pick a log file (CSV, .mmcd, or .PDB),
// reads it, and returns the converted float data for the graph.
func (a *App) LoadLogFile() (*LogData, error) {
	selection, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open Log File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Log Files (*.csv, *.mmcd, *.pdb)", Pattern: "*.csv;*.mmcd;*.pdb;*.PDB"},
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "MMCD Binary (*.mmcd)", Pattern: "*.mmcd"},
			{DisplayName: "PalmOS PDB (*.pdb)", Pattern: "*.pdb;*.PDB"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if selection == "" {
		return nil, fmt.Errorf("cancelled")
	}

	slog.Info("loading log file", "path", selection)

	lower := strings.ToLower(selection)
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return a.loadCSVLog(selection)
	case strings.HasSuffix(lower, ".mmcd"):
		return a.loadBinaryLog(selection)
	case strings.HasSuffix(lower, ".pdb"):
		return a.loadPDBLog(selection)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", selection)
	}
}

func (a *App) loadCSVLog(path string) (*LogData, error) {
	csvLog, err := logger.ReadCSVLog(path)
	if err != nil {
		return nil, err
	}
	// Generate elapsed times: assume ~50ms per sample if no real timestamps
	elapsed := make([]float64, csvLog.Count)
	for i := range elapsed {
		elapsed[i] = float64(i) * 50.0
	}
	// Use Elapsed_ms from CSV if available
	if csvLog.ElapsedMs != nil && len(csvLog.ElapsedMs) == csvLog.Count {
		elapsed = csvLog.ElapsedMs
	}
	return &LogData{
		Slugs:     csvLog.Slugs,
		Data:      csvLog.Data,
		ElapsedMs: elapsed,
		Count:     csvLog.Count,
		Name:      path,
	}, nil
}

func (a *App) loadBinaryLog(path string) (*LogData, error) {
	binLog, err := logger.ReadBinaryLog(path)
	if err != nil {
		return nil, err
	}

	// Convert samples to float data keyed by slug
	data := make(map[string][]float64)
	var slugs []string

	for _, idx := range binLog.Indices {
		if idx >= 0 && idx < len(a.defs) && a.defs[idx].Exists {
			slug := a.defs[idx].Slug
			slugs = append(slugs, slug)
			data[slug] = make([]float64, 0, len(binLog.Samples))
		}
	}

	elapsed := make([]float64, 0, len(binLog.Samples))
	var startTime time.Time
	for i, sample := range binLog.Samples {
		if i == 0 {
			startTime = sample.Time
		}
		elapsed = append(elapsed, float64(sample.Time.Sub(startTime).Milliseconds()))
		sample.ComputeDerivatives(a.defs)
		for _, idx := range binLog.Indices {
			if idx >= 0 && idx < len(a.defs) && a.defs[idx].Exists {
				slug := a.defs[idx].Slug
				if sample.HasData(idx) {
					data[slug] = append(data[slug], a.defs[idx].Convert(sample.RawData[idx], binLog.Units))
				} else {
					data[slug] = append(data[slug], 0)
				}
			}
		}
	}

	return &LogData{
		Slugs:     slugs,
		Data:      data,
		ElapsedMs: elapsed,
		Count:     len(binLog.Samples),
		Name:      path,
	}, nil
}

func (a *App) loadPDBLog(path string) (*LogData, error) {
	pdbLog, err := logger.ParsePDB(path)
	if err != nil {
		return nil, err
	}

	// Determine which sensors are present across all samples
	var presentMask uint32
	for _, s := range pdbLog.Samples {
		presentMask |= s.DataPresent
	}

	data := make(map[string][]float64)
	var slugs []string
	var indices []int

	for i, def := range a.defs {
		if def.Exists && presentMask&(1<<uint(i)) != 0 {
			slugs = append(slugs, def.Slug)
			indices = append(indices, i)
			data[def.Slug] = make([]float64, 0, len(pdbLog.Samples))
		}
	}

	elapsed := make([]float64, 0, len(pdbLog.Samples))
	var startTime time.Time
	for i, sample := range pdbLog.Samples {
		if i == 0 {
			startTime = sample.Time
		}
		elapsed = append(elapsed, float64(sample.Time.Sub(startTime).Milliseconds()))
		sample.ComputeDerivatives(a.defs)
		for _, idx := range indices {
			slug := a.defs[idx].Slug
			if sample.HasData(idx) {
				data[slug] = append(data[slug], a.defs[idx].Convert(sample.RawData[idx], sensor.UnitMetric))
			} else {
				data[slug] = append(data[slug], 0)
			}
		}
	}

	return &LogData{
		Slugs:     slugs,
		Data:      data,
		ElapsedMs: elapsed,
		Count:     len(pdbLog.Samples),
		Name:      pdbLog.Name,
	}, nil
}
