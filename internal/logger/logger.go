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

// ErrorCallback is called when a poll cycle encounters an error.
type ErrorCallback func(err error)

// DisconnectCallback is called when the watchdog detects persistent failures.
type DisconnectCallback func()

// LoggerStats holds runtime statistics for the polling loop.
type LoggerStats struct {
	SampleCount   uint64  `json:"sampleCount"`
	ErrorCount    uint64  `json:"errorCount"`
	CurrentHz     float64 `json:"currentHz"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
}

// Logger manages the ECU polling loop and data collection.
type Logger struct {
	poller    SamplePoller
	defs      []sensor.Definition
	indices   []int // sensor indices to poll
	units     sensor.UnitSystem
	callbacks []SampleCallback
	errCbs    []ErrorCallback
	disconnCb DisconnectCallback
	pollRate  time.Duration // interval between polls

	mu              sync.Mutex
	running         bool
	cancel          context.CancelFunc
	lastSample      sensor.Sample
	sampleCount     uint64
	errorCount      uint64
	consecutiveErrs uint32
	startTime       time.Time
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

// OnError registers a callback that fires on each poll error.
func (l *Logger) OnError(cb ErrorCallback) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errCbs = append(l.errCbs, cb)
}

// OnDisconnect registers a callback for when the watchdog detects persistent failures.
func (l *Logger) OnDisconnect(cb DisconnectCallback) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.disconnCb = cb
}

// Stats returns current polling statistics.
func (l *Logger) Stats() LoggerStats {
	l.mu.Lock()
	defer l.mu.Unlock()
	var hz float64
	if !l.startTime.IsZero() {
		elapsed := time.Since(l.startTime).Seconds()
		if elapsed > 0 {
			hz = float64(l.sampleCount) / elapsed
		}
	}
	return LoggerStats{
		SampleCount:   l.sampleCount,
		ErrorCount:    l.errorCount,
		CurrentHz:     hz,
		UptimeSeconds: time.Since(l.startTime).Seconds(),
	}
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

const watchdogThreshold = 20 // consecutive errors before declaring disconnect

// pollLoop continuously queries the ECU for sensor data.
func (l *Logger) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(l.pollRate)
	defer ticker.Stop()

	l.mu.Lock()
	l.startTime = time.Now()
	l.sampleCount = 0
	l.errorCount = 0
	l.consecutiveErrs = 0
	l.mu.Unlock()

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
				l.mu.Lock()
				l.errorCount++
				l.consecutiveErrs++
				consecErrs := l.consecutiveErrs
				errCbs := make([]ErrorCallback, len(l.errCbs))
				copy(errCbs, l.errCbs)
				disconnCb := l.disconnCb
				l.mu.Unlock()

				slog.Debug("poll error", "error", err, "consecutive", consecErrs)

				for _, cb := range errCbs {
					cb(err)
				}

				// Watchdog: if too many consecutive errors, assume disconnect
				if consecErrs >= watchdogThreshold {
					slog.Warn("watchdog: too many consecutive errors, assuming ECU disconnect", "count", consecErrs)
					if disconnCb != nil {
						disconnCb()
					}
					return
				}
				continue
			}

			l.mu.Lock()
			l.sampleCount++
			l.consecutiveErrs = 0
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
