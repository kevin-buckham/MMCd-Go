package logger

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/kbuckham/mmcd/internal/sensor"
)

// SamplePoller is the interface for anything that can poll sensor data.
// Both the real ECU and the Simulator implement this.
type SamplePoller interface {
	PollSensors(indices []int) (sensor.Sample, error)
}

// SampleCallback is called each time a complete sensor sample is collected.
type SampleCallback func(sample sensor.Sample)

// Logger manages the ECU polling loop and data collection.
type Logger struct {
	poller    SamplePoller
	defs      []sensor.Definition
	indices   []int // sensor indices to poll
	units     sensor.UnitSystem
	callbacks []SampleCallback
	pollRate  time.Duration // interval between polls

	mu         sync.Mutex
	running    bool
	cancel     context.CancelFunc
	lastSample sensor.Sample
}

// New creates a new Logger with a real ECU or simulator as the poller.
func New(poller SamplePoller, defs []sensor.Definition, indices []int, units sensor.UnitSystem) *Logger {
	return &Logger{
		poller:   poller,
		defs:     defs,
		indices:  indices,
		units:    units,
		pollRate: 1 * time.Millisecond, // as fast as possible for real ECU
	}
}

// NewWithRate creates a Logger with a specific poll rate (useful for simulator).
func NewWithRate(poller SamplePoller, defs []sensor.Definition, indices []int, units sensor.UnitSystem, rate time.Duration) *Logger {
	l := New(poller, defs, indices, units)
	l.pollRate = rate
	return l
}

// OnSample registers a callback that fires each time a sample is collected.
func (l *Logger) OnSample(cb SampleCallback) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.callbacks = append(l.callbacks, cb)
}

// Start begins the polling loop in a goroutine.
func (l *Logger) Start() error {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel
	l.running = true
	l.mu.Unlock()

	go l.pollLoop(ctx)
	slog.Info("logger started", "sensors", len(l.indices))
	return nil
}

// Stop halts the polling loop.
func (l *Logger) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return
	}

	l.cancel()
	l.running = false
	slog.Info("logger stopped")
}

// IsRunning returns whether the logger is actively polling.
func (l *Logger) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

// LastSample returns the most recently collected sample.
func (l *Logger) LastSample() sensor.Sample {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastSample
}

// SetIndices updates which sensors are polled (can be called while running).
func (l *Logger) SetIndices(indices []int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.indices = indices
}

// pollLoop continuously queries the ECU for sensor data.
func (l *Logger) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(l.pollRate)
	defer ticker.Stop()

	cycleCount := uint64(0)
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.mu.Lock()
			indices := make([]int, len(l.indices))
			copy(indices, l.indices)
			l.mu.Unlock()

			if len(indices) == 0 {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			sample, err := l.poller.PollSensors(indices)
			if err != nil {
				slog.Debug("poll error", "error", err)
				continue
			}

			cycleCount++
			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 && cycleCount%10 == 0 {
				slog.Debug("sample rate", "hz", float64(cycleCount)/elapsed)
			}

			l.mu.Lock()
			l.lastSample = sample
			callbacks := make([]SampleCallback, len(l.callbacks))
			copy(callbacks, l.callbacks)
			l.mu.Unlock()

			for _, cb := range callbacks {
				cb(sample)
			}
		}
	}
}
