# CLAUDE.md — MMCd Datalogger

All project rules, constraints, safety, code style, and build commands are in [`AGENTS.md`](AGENTS.md).
This file contains Claude Code-specific workflow instructions only.

## Quick Reference

```bash
make build          # Build desktop app
make cli            # Build CLI only (no Wails)
make dev            # Dev mode with hot reload
make test           # Run tests (or: go test ./internal/...)
go vet ./...        # Vet
gofmt -w .          # Format
```

## Workflow Rules

- Read `AGENTS.md` before making changes. It defines serial protocol safety rules, concurrency patterns, and code style.
- Run `go build ./...` after every code change to verify compilation.
- Run `go test ./...` before committing.
- Never commit without running tests. If tests fail, fix before committing.

## Commit Protocol

- Use imperative, descriptive messages: "Fix ...", "Add ...", "Update ..."
- End commit messages with: `Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>`
- Use HEREDOC format for multi-line commit messages.
- Do not push to remote unless explicitly asked.
- For releases: bump version in `internal/version/version.go`, commit, push, then tag `v{version}` and push tag.

## Serial Protocol — Read Before Touching

Before modifying anything in `internal/protocol/`:
1. Read `AGENTS.md` Section 3 (Serial Protocol Safety).
2. `ECU.busMu` must be held for entire send+receive cycles. Never release between send and receive.
3. `SetReadTimeout` takes `time.Duration` — use `500 * time.Millisecond`, not `500`.
4. Always `conn.Flush()` on error paths to clear stale serial data.

## File Scope

- `internal/protocol/` — Serial I/O and ECU protocol. Highest risk. Changes here affect vehicle communication.
- `internal/sensor/` — Pure data definitions and conversions. Low risk.
- `internal/logger/` — Poll loop and data writers. Medium risk (concurrency).
- `app.go` — Wails bindings and state machine. Changes affect frontend contract.
- `frontend/src/` — Svelte components. No Go impact but check event contract in `AGENTS.md` Section 5.
