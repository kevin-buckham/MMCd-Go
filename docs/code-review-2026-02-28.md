# Code Review: MMCd-Go vs Original MMCD C (PalmOS)

**Date:** 2026-02-28
**Reviewer:** Claude Opus 4.6
**Scope:** Serial protocol reliability, goroutine usage, concurrency
**Compared against:** `~/mmcd/MMCd/` (original PalmOS C codebase)

## Summary

17 findings total. The Go rewrite is architecturally sound and captures the essential protocol behavior. Key differences in baud rate, polling strategy, and error recovery warrant attention.

## Critical

### 1. Baud Rate Mismatch

**Original C** (`mmcd.c:73`):
```c
#define ECU_BAUDRATE 1920
```

**Go** (`serial.go:13-14`):
```go
DefaultBaudRate = 1953
```

The comment acknowledges the discrepancy ("spec says 1953, original code uses 1920") but defaults to 1953. The original developers chose 1920 and tested it against real hardware. EvoScan also uses 1920. While the 1.7% difference is within UART tolerance, USB-serial adapter clock quantization may push it outside tolerance on some chipsets.

**Recommendation:** Change default to 1920 to match the original, or make it user-configurable.

## Important

### 2. Synchronous vs Pipelined Polling

**Original C** (`mmcd.c:657-661`): Pipelined — sends the next sensor request immediately after receiving the current response, before returning to the event loop:
```c
if(nextSensor()) {
    SrmSend(portId, &_pnlSensor[currentSensor].addr, 1, &err);
    receiveTimeout = TimGetTicks();
    waitingAnswer = true;
}
```

**Go** (`ecu.go:55-96`): Synchronous — send, block until full response, return, then send next. Adds ~5ms ECU idle time per sensor. With 22 sensors, that's ~110ms slower per full scan cycle.

**Recommendation:** Consider pipelining where the next request is sent immediately after receiving the current response.

### 3. Error Recovery Differences

| Aspect | Original C | Go |
|--------|-----------|-----|
| Error threshold | 5 consecutive errors | 20 consecutive errors |
| Recovery action | Close + reopen port | Disconnect entirely |
| Error clearing | `SrmClearErr` + `SrmReceiveFlush` | `ResetInputBuffer` only |
| Intermediate recovery | Reopens port before giving up | None — straight to disconnect |

**Recommendation:** Consider reducing watchdog threshold and adding an intermediate close/reopen recovery step before full disconnect.

### 4. CSV Writer Data Race

**File:** `app.go:311-343`

The `OnSample` callback registered in `StartLogging` accesses `a.csvWriter` from the poll goroutine without holding `a.mu`. Meanwhile, `StopLogging` sets `a.csvWriter = nil` under `a.mu`. If `StopLogging` runs between the nil check and `WriteSample`, a panic or write to a closed writer could occur.

**Recommendation:** Protect `a.csvWriter` access in the callback with `a.mu`, or capture the writer in a local variable at registration time.

### 5. `emitStats` Goroutine Leak

**File:** `app.go:294`, `commlog.go:115-137`

Each call to `StartMonitoring` launches a new `emitStats` goroutine with no cancellation mechanism. If monitoring is stopped and restarted, duplicate goroutines accumulate until they self-exit on their next 2-second tick.

**Recommendation:** Use a `context.Context` or done channel to signal the goroutine to stop immediately.

### 6. `App.log()` Accesses `a.ctx` Without Mutex

**File:** `commlog.go:61-88`

The `log()` method accesses `a.ctx` without holding `a.mu`. It's called from both mutex-holding contexts (Connect, Disconnect) and non-holding contexts (OnError, OnDisconnect callbacks). In practice, `a.ctx` is set once in `startup()` and never changed, so this is safe but would trigger Go's race detector.

**Recommendation:** Document immutability of `a.ctx`, or pass it as a parameter to callbacks.

## Suggestions

### 7. No Port Close/Reopen on Intermediate Errors

The C code physically closes and reopens the serial port after 5 errors, which resets UART hardware state and USB-serial adapter. The Go code has no intermediate recovery — it goes straight to full disconnect after 20 errors.

### 8. Missing State Reset on Port Open

The C code explicitly resets scan state (sensor index, pending answer flag, data-present flags) when opening the port. The Go code relies on architectural separation and the `Probe()` flush, which is adequate but less explicit.

### 9. Command Echo Timeout

The C code uses 100ms for the command echo timeout (`ticksPerSecond / 10`). The Go code uses 500ms. Could be tightened to 200ms to fail faster on bad connections.

### 10. No Inter-Request Delay

The C code naturally has inter-request gaps due to the PalmOS event loop. The Go code sends requests back-to-back. The ECU should handle this since the serial line speed is the bottleneck, but a small delay could improve reliability.

### 11. Missing VSPD Sensor (Index 23)

The C code defines VSPD (Vehicle Speed) at index 23, address 0xEF. The Go code marks indices 23-31 as unused. Note: 0xEF is in the command range (>=0xC0) which Go's `QuerySensor` guard would block.

### 12. Address 0x12 Slug Difference

The C code dynamically maps address 0x12 to MAP/WBO2/EGRT/0-5V based on user configuration. The Go code hardcodes it as EGRT.

## Confirmed Correct

- **DTC byte ordering** — matches original exactly
- **`ECU.busMu` serialization** — correctly prevents interleaved send+receive pairs
- **Logger callback pattern** — copy-then-invoke outside lock, correct and deadlock-free
- **Disconnect callback goroutine** — correctly avoids deadlock from callback context
- **Logger mutex granularity** — correctly releases lock during blocking serial I/O
- **Sensor conversions** — faithfully match original C conversion functions
- **INJD formula** — `IPW * RPM / 117` matches C `computeDerivatives`
- **Command whitelist** — safety improvement over original (C has no guard)
- **Probe on connect** — good addition the original lacks
