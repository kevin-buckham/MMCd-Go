package logger

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kbuckham/mmcd/internal/sensor"
)

// mockPoller is a test double for SamplePoller.
type mockPoller struct {
	mu        sync.Mutex
	failCount int // number of times to return error before succeeding
	calls     int
}

func (m *mockPoller) PollSensors(indices []int) (sensor.Sample, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.failCount > 0 {
		m.failCount--
		return sensor.Sample{}, fmt.Errorf("mock poll error")
	}
	s := sensor.Sample{Time: time.Now()}
	return s, nil
}

// alwaysFailPoller always returns errors.
type alwaysFailPoller struct {
	calls atomic.Int64
}

func (p *alwaysFailPoller) PollSensors(indices []int) (sensor.Sample, error) {
	p.calls.Add(1)
	return sensor.Sample{}, fmt.Errorf("persistent failure")
}

func TestLogger_StatsCountSamples(t *testing.T) {
	poller := &mockPoller{}
	defs := sensor.DefaultDefinitions()
	indices := []int{14} // TPS
	lg := NewWithRate(poller, defs, indices, sensor.UnitMetric, 5*time.Millisecond)

	sampleCount := atomic.Int64{}
	lg.OnSample(func(s sensor.Sample) {
		sampleCount.Add(1)
	})

	lg.Start()
	time.Sleep(100 * time.Millisecond)
	lg.Stop()

	stats := lg.Stats()
	if stats.SampleCount == 0 {
		t.Error("Stats.SampleCount should be > 0 after polling")
	}
	if stats.ErrorCount != 0 {
		t.Errorf("Stats.ErrorCount = %d, want 0", stats.ErrorCount)
	}
	if stats.CurrentHz <= 0 {
		t.Error("Stats.CurrentHz should be > 0 after polling")
	}
	if sampleCount.Load() == 0 {
		t.Error("OnSample callback should have been called")
	}
}

func TestLogger_ErrorCallback(t *testing.T) {
	poller := &mockPoller{failCount: 5}
	defs := sensor.DefaultDefinitions()
	indices := []int{14}
	lg := NewWithRate(poller, defs, indices, sensor.UnitMetric, 5*time.Millisecond)

	errorCount := atomic.Int64{}
	lg.OnError(func(err error) {
		errorCount.Add(1)
	})

	lg.Start()
	time.Sleep(200 * time.Millisecond)
	lg.Stop()

	stats := lg.Stats()
	if stats.ErrorCount == 0 {
		t.Error("Stats.ErrorCount should be > 0 with failing poller")
	}
	if errorCount.Load() == 0 {
		t.Error("OnError callback should have been called")
	}
	// After errors stop, samples should succeed
	if stats.SampleCount == 0 {
		t.Error("Stats.SampleCount should be > 0 after errors resolve")
	}
}

func TestLogger_WatchdogDisconnect(t *testing.T) {
	poller := &alwaysFailPoller{}
	defs := sensor.DefaultDefinitions()
	indices := []int{14}
	lg := NewWithRate(poller, defs, indices, sensor.UnitMetric, 1*time.Millisecond)

	disconnected := atomic.Bool{}
	lg.OnDisconnect(func() {
		disconnected.Store(true)
	})

	lg.Start()
	// Wait for watchdog to trigger (20 consecutive errors at 1ms each = ~20ms)
	time.Sleep(200 * time.Millisecond)

	if !disconnected.Load() {
		t.Error("OnDisconnect should have been called after persistent failures")
	}

	stats := lg.Stats()
	if stats.ErrorCount < uint64(watchdogThreshold) {
		t.Errorf("Stats.ErrorCount = %d, should be >= %d (watchdog threshold)", stats.ErrorCount, watchdogThreshold)
	}
}

func TestLogger_IsRunning(t *testing.T) {
	poller := &mockPoller{}
	defs := sensor.DefaultDefinitions()
	lg := NewWithRate(poller, defs, []int{14}, sensor.UnitMetric, 10*time.Millisecond)

	if lg.IsRunning() {
		t.Error("IsRunning() should be false before Start()")
	}

	lg.Start()
	if !lg.IsRunning() {
		t.Error("IsRunning() should be true after Start()")
	}

	lg.Stop()
	if lg.IsRunning() {
		t.Error("IsRunning() should be false after Stop()")
	}
}

func TestLogger_StartStopIdempotent(t *testing.T) {
	poller := &mockPoller{}
	defs := sensor.DefaultDefinitions()
	lg := NewWithRate(poller, defs, []int{14}, sensor.UnitMetric, 10*time.Millisecond)

	// Multiple starts should be safe
	lg.Start()
	lg.Start()
	if !lg.IsRunning() {
		t.Error("should still be running after double Start()")
	}

	// Multiple stops should be safe
	lg.Stop()
	lg.Stop()
	if lg.IsRunning() {
		t.Error("should not be running after double Stop()")
	}
}

func TestLogger_ConsecutiveErrorsResetOnSuccess(t *testing.T) {
	// Fail 5 times, then succeed — should NOT trigger watchdog
	poller := &mockPoller{failCount: 5}
	defs := sensor.DefaultDefinitions()
	indices := []int{14}
	lg := NewWithRate(poller, defs, indices, sensor.UnitMetric, 2*time.Millisecond)

	disconnected := atomic.Bool{}
	lg.OnDisconnect(func() {
		disconnected.Store(true)
	})

	lg.Start()
	time.Sleep(200 * time.Millisecond)
	lg.Stop()

	if disconnected.Load() {
		t.Error("OnDisconnect should NOT have been called — errors resolved before threshold")
	}
}
