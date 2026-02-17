//go:build !cli

package main

import (
	"log/slog"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxLogEntries = 500

// LogEntry represents a single communication log entry.
type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`   // "info", "warn", "error"
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

// CommStats holds runtime statistics for the polling loop.
type CommStats struct {
	SamplesTotal  uint64  `json:"samplesTotal"`
	ErrorsTotal   uint64  `json:"errorsTotal"`
	CurrentHz     float64 `json:"currentHz"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
}

// CommLog is a ring-buffer based communication log that emits events to the frontend.
type CommLog struct {
	mu      sync.Mutex
	entries []LogEntry
}

func newCommLog() *CommLog {
	return &CommLog{
		entries: make([]LogEntry, 0, maxLogEntries),
	}
}

func (cl *CommLog) add(entry LogEntry) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.entries = append(cl.entries, entry)
	if len(cl.entries) > maxLogEntries {
		cl.entries = cl.entries[len(cl.entries)-maxLogEntries:]
	}
}

func (cl *CommLog) getAll() []LogEntry {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	out := make([]LogEntry, len(cl.entries))
	copy(out, cl.entries)
	return out
}

// log adds an entry to the comm log, emits it to the frontend, and logs via slog.
func (a *App) log(level, message, detail string) {
	entry := LogEntry{
		Time:    time.Now().Format("15:04:05.000"),
		Level:   level,
		Message: message,
		Detail:  detail,
	}

	if a.commLog == nil {
		a.commLog = newCommLog()
	}
	a.commLog.add(entry)

	// Emit to frontend
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "comm:log", entry)
	}

	// Also log via slog
	switch level {
	case "error":
		slog.Error(message, "detail", detail)
	case "warn":
		slog.Warn(message, "detail", detail)
	default:
		slog.Info(message, "detail", detail)
	}
}

// GetCommLog returns all recent log entries (Wails-bound).
func (a *App) GetCommLog() []LogEntry {
	if a.commLog == nil {
		return nil
	}
	return a.commLog.getAll()
}

// GetCommStats returns current communication statistics (Wails-bound).
func (a *App) GetCommStats() *CommStats {
	a.mu.Lock()
	defer a.mu.Unlock()

	stats := &CommStats{}
	if a.lg != nil {
		ls := a.lg.Stats()
		stats.SamplesTotal = ls.SampleCount
		stats.ErrorsTotal = ls.ErrorCount
		stats.CurrentHz = ls.CurrentHz
		stats.UptimeSeconds = ls.UptimeSeconds
	}
	return stats
}

// emitStats sends periodic stats to the frontend. Called from a goroutine.
func (a *App) emitStats() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		a.mu.Lock()
		if a.lg == nil || !a.lg.IsRunning() {
			a.mu.Unlock()
			return
		}
		ls := a.lg.Stats()
		a.mu.Unlock()

		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "comm:stats", CommStats{
				SamplesTotal:  ls.SampleCount,
				ErrorsTotal:   ls.ErrorCount,
				CurrentHz:     ls.CurrentHz,
				UptimeSeconds: ls.UptimeSeconds,
			})
		}
	}
}
