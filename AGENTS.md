# AGENTS.md — MMCd Datalogger

Architectural guardrails for AI tools working on this codebase.

## 1. Stack & Runtime

- **Runtime:** Go 1.22+ / Wails v2 desktop app + headless CLI
- **Frontend:** Svelte 4 + Vite 5 + uplot (charting)
- **Bus Type:** Serial UART (1953 baud, 8N1, half-duplex ALDL) via `go.bug.st/serial`
- **Protocol:** MMCD — 1-byte request, 2-byte response (echo + data) on single wire
- **Platforms:** Linux, macOS, Windows (cross-compiled CLI; native desktop per-platform)

## 2. Implementation Patterns

### Package Layering

```
protocol/  →  logger/  →  app.go / cli/
   ↑              ↑
 sensor/ (standalone, no deps on other internal packages)
```

No circular dependencies. `protocol/` is the lowest layer.

### Wiring: Manual Construction

Correct:
```go
conn := protocol.NewSerialConn(port, baud)
ecu := protocol.NewECU(conn, defs)
lg := logger.NewWithRate(ecu, defs, indices, units, rate)
```

Wrong:
```go
// No DI frameworks, no service locators, no init() magic
container.Register(...)
```

### Serial Bus: Atomic Send+Receive

Correct:
```go
e.busMu.Lock()
defer e.busMu.Unlock()
e.conn.Send([]byte{addr})
// read echo + data while holding lock
n, _ := e.conn.Receive(buf)
```

Wrong:
```go
e.conn.Send([]byte{addr})
// UNLOCKED — another goroutine could send here
e.conn.Receive(buf) // corrupted response
```

### Error Handling: Wrap with %w, Flush on Error

Correct:
```go
if totalRead < 2 {
    e.conn.Flush() // clear stale bytes
    return 0, fmt.Errorf("timeout reading response for 0x%02X: got %d bytes", addr, totalRead)
}
```

Wrong:
```go
return 0, errors.New("read failed") // no context, no flush
```

### Callbacks: Snapshot Before Invoke

Correct:
```go
l.mu.Lock()
callbacks := make([]SampleCallback, len(l.callbacks))
copy(callbacks, l.callbacks)
l.mu.Unlock()
for _, cb := range callbacks { cb(sample) } // outside lock
```

Wrong:
```go
l.mu.Lock()
for _, cb := range l.callbacks { cb(sample) } // holding lock — deadlock risk
l.mu.Unlock()
```

## 3. Serial Protocol Safety (CRITICAL)

| Rule | Enforcement |
|------|-------------|
| No concurrent bus access | `ECU.busMu` held for entire send+receive cycle |
| No polling command addresses | `QuerySensor` rejects addr >= 0xC0 |
| Commands are whitelisted | `validCommandAddrs` map; `SendCommand` rejects unknown |
| Echo verification required | Every response checked; mismatch = flush + error |
| Flush on error | `conn.Flush()` after timeout or mismatch to clear stale bytes |
| Pause monitoring for commands | `Stop()` logger before DTC/actuator ops, `Start()` after |
| Timeout all reads | Deadline-based loop, never block indefinitely |
| `SetReadTimeout` uses `time.Duration` | Pass `500 * time.Millisecond`, NOT `500` (that's 500 nanoseconds) |

### Address Space

| Range | Type | Handling |
|-------|------|----------|
| 0x00–0xBF | Sensor queries | `QuerySensor()` — echo + 1 data byte |
| 0xC0–0xEF | Reserved | Rejected by both Query and Command |
| 0xCA | DTC erase | `SendCommand()` with 2s timeout |
| 0xF1–0xFC | Actuator tests | `SendCommand()` with 10s timeout |

## 4. Concurrency Model

### Mutex Map

| Mutex | Protects | Held For |
|-------|----------|----------|
| `SerialConn.mu` | Serial port read/write | Single `Send()` or `Receive()` call |
| `ECU.busMu` | Bus-level atomicity | Entire send+receive cycle |
| `Logger.mu` | Poll loop state, callbacks | Per-tick decision; callbacks called outside |
| `App.mu` | All app state | Connection/monitoring/settings changes |
| `CommLog.mu` | Frontend log ring buffer | Single append/copy |
| `CSVWriter.mu` | CSV file writes | Per-row write + flush |

### Goroutine Lifecycle

- **Poll loop:** `go l.pollLoop(ctx)` — exits on `ctx.Done()` or watchdog (20 consecutive errors)
- **Stats emission:** `go a.emitStats()` — ticker loop, exits when logger stops
- **Disconnect handler:** `go func() { a.mu.Lock(); cleanup; a.mu.Unlock() }()` — avoids deadlock from callback context
- All goroutines must have a defined exit path. No leaked goroutines.

## 5. Wails Frontend-Backend Boundary

### Event Contract

| Event | Payload | Trigger |
|-------|---------|---------|
| `connection:status` | `{connected, port, baud, demo?, reason?}` | Connect/Disconnect/Watchdog |
| `sensor:sample` | `{time, values, floats, rawData, dataPresent}` | Each poll cycle |
| `comm:stats` | `{SamplesTotal, ErrorsTotal, CurrentHz, UptimeSeconds}` | Every 2 seconds |
| `comm:log` | `{Time, Level, Message, Detail}` | Any loggable event |
| `logging:status` | `{logging, filename?, count?}` | Start/Stop CSV logging |

### Rules

- All Wails-bound methods acquire `App.mu` before accessing state.
- Callbacks from logger goroutine must not hold `App.mu` — decouple via goroutine if needed.
- Wails auto-marshals Go structs to JSON. Use `json:` struct tags.
- Frontend receives events via `runtime.EventsOn()` in Svelte.

## 6. Application Reliability

- **Error handling:** Wrap all errors with `fmt.Errorf("context: %w", err)`. No silent swallowing. Propagate or handle.
- **Resource cleanup:** `defer conn.Close()`, `defer writer.Close()`, `defer lg.Stop()` on all paths including error paths. Nil-check before Close.
- **Timeouts:** All serial reads use deadline-based loops. No indefinite waits.
- **Watchdog:** 20 consecutive poll errors triggers auto-disconnect with user notification.
- **Graceful degradation:** Probe failure on connect is a warning, not a hard error — user can still attempt monitoring.
- **State cleanup on disconnect:** Stop logger, close CSV writer, close serial port, nil all references, notify frontend.

## 7. Code Style & Conventions

### Naming

| Scope | Convention | Example |
|-------|-----------|---------|
| Exported functions | PascalCase | `QuerySensor()`, `PollSensors()` |
| Unexported functions | camelCase | `pollLoop()`, `decodeDTCs()` |
| Receiver names | Single letter | `(sc *SerialConn)`, `(e *ECU)`, `(lg *Logger)` |
| Constants | PascalCase | `DefaultBaudRate`, `MaxSensors` |
| Sensor slugs | Uppercase 4-char | `RPM`, `TPS`, `COOL`, `INJP` |
| Conversion functions | `f` prefix | `fDEC()`, `fERPM()`, `fBATT()` |

### Imports

Three groups separated by blank lines:
```go
import (
    "fmt"           // 1. Standard library (alphabetical)
    "time"

    "go.bug.st/serial"  // 2. External third-party

    "github.com/kbuckham/mmcd/internal/protocol"  // 3. Internal packages
)
```

### Error Handling

- Return `error`, never `panic` (except unrecoverable startup failures).
- Wrap with `fmt.Errorf("context: %w", err)` — always include `%w`.
- No sentinel errors. No custom error types.

### Logging

- Backend: `slog.Info/Warn/Debug/Error` with structured key-value pairs.
- Frontend-facing: `a.log(level, message, detail)` emits to both slog and Wails event.
- Debug level for per-sample poll errors. Info for lifecycle events. Warn for probe failures. Error for connection loss.

### Testing

- Test naming: `Test<Type>_<Behavior>` (e.g., `TestLogger_WatchdogDisconnect`)
- Mock pollers implement `SamplePoller` interface.
- File-based tests use `os.CreateTemp` with `defer os.Remove`.
- No subtests (`t.Run`), no test helpers beyond mock structs.

## 8. Build & Test Commands

```bash
# Prerequisites (first time)
make setup

# Build desktop app
make build

# Build CLI only (no Wails/frontend)
make cli

# Dev mode (hot reload)
make dev

# Run tests
make test
# or: go test ./internal/...

# Cross-compile CLI for all platforms
make release-cli

# Build desktop for current platform
make release-desktop

# Full release (CLI + desktop)
make release

# Format
gofmt -w .

# Vet
go vet ./...
```

## 9. Commit & PR Conventions

- **Format:** Imperative, descriptive (e.g., "Fix serial read timeout bug causing no ECU data on macOS/Linux")
- **Prefixes by type:** "Add" (new feature), "Fix" (bug fix), "Update" (docs/existing), "Bump version to" (release prep)
- **Co-author:** `Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>` when AI-assisted
- **Releases:** Tag with `v{major}.{minor}.{patch}`, push tag to trigger CI release workflow
- **Branch model:** Direct to `main` (small team, early stage)

## 10. Project Mapping

### Critical Files

| File | Purpose |
|------|---------|
| `internal/protocol/ecu.go` | ECU protocol — query/command logic, bus safety, address validation |
| `internal/protocol/serial.go` | Serial port — open/close/send/receive, baud rate, timeout |
| `internal/sensor/definitions.go` | Sensor table — 22 sensors with addresses, slugs, conversion functions |
| `internal/logger/logger.go` | Poll loop — watchdog, callbacks, stats, goroutine lifecycle |
| `app.go` | Wails bindings — state machine, frontend events, all user-facing methods |

### Testing

- **No hardware-in-the-loop:** Tests use mock pollers and simulators.
- **Simulator:** `protocol.Simulator` generates realistic fake data for UI development.
- **Demo mode:** `ConnectDemo()` uses simulator — no serial port needed.
- **Run:** `go test ./internal/...` or `make test`

## 11. Review Checklist

### Serial Protocol Safety

- [ ] Does this change send bytes to the serial port? Is `ECU.busMu` held for the entire send+receive cycle?
- [ ] Does this add a new ECU address? Is it in the correct range (sensor < 0xC0, command whitelisted)?
- [ ] Does this change flush the receive buffer on error paths?
- [ ] Are all serial reads deadline-based? No indefinite blocking?
- [ ] If adding a new command, is monitoring paused first (`lg.Stop()` / `lg.Start()`)?

### Concurrency

- [ ] Are shared resources protected by the correct mutex?
- [ ] Are callbacks invoked outside of held locks?
- [ ] Does any new goroutine have a defined exit path (context cancellation, channel close, or condition)?
- [ ] Could this change cause a deadlock between `App.mu` and `ECU.busMu`?

### Resource Cleanup

- [ ] Are file descriptors, serial ports, and writers closed on all paths (including error paths)?
- [ ] Is `defer` used for cleanup? Is nil-check done before `Close()`?
- [ ] On disconnect, is all state properly nil'd and frontend notified?

### Frontend Contract

- [ ] If changing Wails-bound method signatures, is the Svelte frontend updated?
- [ ] If adding/changing events, does the payload match what the frontend expects?
- [ ] Are struct fields tagged with `json:` for proper marshaling?

### Application Reliability

- [ ] Are errors wrapped with `fmt.Errorf("context: %w", err)`?
- [ ] Is the watchdog threshold (20 consecutive errors) still appropriate for this change?
- [ ] Does this change affect the connection probe or its error reporting?

## Docs Lifecycle

```
docs/
├── plans/          # Temporary — active development plans
├── features/       # One file per shipped feature (converted from plans)
└── 1G-DSM-Cable.md # Hardware build guide
```

### Rules

- **On feature completion:** Convert `docs/plans/[name].md` to `docs/features/[name].md`. Delete the plan.
- **On every PR:** Check that docs touched by the change are still accurate. Remove stale references.
- **Plans are disposable.** They exist to guide work in progress. Never let a completed plan linger.
