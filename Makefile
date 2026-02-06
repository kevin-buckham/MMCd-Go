.PHONY: build run dev clean test cli frontend install setup check check-go check-node check-wails check-linux-deps ensure-wails release release-cli release-desktop

# Ensure GOPATH/bin is in PATH so wails CLI is found
export PATH := $(PATH):$(shell go env GOPATH)/bin

# Strip debug symbols and DWARF info; inject git hash and build timestamp
VERSION_PKG := github.com/kbuckham/mmcd/internal/version
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w -X $(VERSION_PKG).GitHash=$(GIT_HASH) -X $(VERSION_PKG).BuildTime=$(BUILD_TIME)

# ── Prerequisite checks ─────────────────────────────────────────────

check-go:
	@command -v go >/dev/null 2>&1 || { \
		echo "ERROR: Go is not installed."; \
		echo "  Install Go 1.22+: https://go.dev/dl/"; \
		exit 1; \
	}
	@echo "✓ Go $$(go version | awk '{print $$3}')" found

check-node:
	@command -v node >/dev/null 2>&1 || { \
		echo "ERROR: Node.js is not installed."; \
		echo "  Install Node 18+: https://nodejs.org/"; \
		exit 1; \
	}
	@echo "✓ Node $$(node --version) found"

check-wails:
	@command -v wails >/dev/null 2>&1 || { \
		echo "WARNING: Wails CLI not found."; \
		echo "  Run 'make ensure-wails' to install it, or:"; \
		echo "  go install github.com/wailsapp/wails/v2/cmd/wails@latest"; \
		exit 1; \
	}
	@echo "✓ Wails $$(wails version 2>/dev/null | head -1) found"

check-linux-deps:
	@if [ "$$(uname)" = "Linux" ]; then \
		MISSING=""; \
		pkg-config --exists gtk+-3.0 2>/dev/null || MISSING="$$MISSING libgtk-3-dev"; \
		pkg-config --exists webkit2gtk-4.0 2>/dev/null || MISSING="$$MISSING libwebkit2gtk-4.0-dev"; \
		if [ -n "$$MISSING" ]; then \
			echo "ERROR: Missing Linux system libraries:$$MISSING"; \
			echo ""; \
			echo "  Ubuntu/Debian:"; \
			echo "    sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev"; \
			echo ""; \
			echo "  Fedora:"; \
			echo "    sudo dnf install gtk3-devel webkit2gtk4.0-devel"; \
			echo ""; \
			echo "  Arch:"; \
			echo "    sudo pacman -S gtk3 webkit2gtk"; \
			exit 1; \
		fi; \
		echo "✓ Linux system libraries (GTK3, WebKit2GTK) found"; \
	fi

# Check all prerequisites (informational)
check: check-go check-node check-linux-deps check-wails
	@echo ""
	@echo "All prerequisites satisfied."

# Install wails CLI if missing
ensure-wails: check-go
	@command -v wails >/dev/null 2>&1 && echo "Wails CLI already installed." || { \
		echo "Installing Wails CLI..."; \
		go install github.com/wailsapp/wails/v2/cmd/wails@latest; \
		echo "✓ Wails CLI installed"; \
	}

# ── Full setup (run this first on a new machine) ────────────────────

setup: check-go check-node check-linux-deps ensure-wails install
	@echo ""
	@echo "Setup complete. Run 'make dev' or 'make build'."

# ── Build targets ───────────────────────────────────────────────────

# Install dependencies
install:
	cd frontend && npm install
	go mod tidy

# Build the full Wails desktop app
build: check-wails check-linux-deps frontend
	wails build

# Build and run the desktop app
run: build
	@echo "Launching MMCD Datalogger..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		open build/bin/mmcd.app; \
	else \
		./build/bin/mmcd; \
	fi

# Run in Wails dev mode (hot reload)
dev: check-wails check-linux-deps
	wails dev

# Build just the CLI (no frontend/Wails dependency)
cli:
	go build -tags cli -ldflags '$(LDFLAGS)' -o bin/mmcd .

# Build the frontend
frontend:
	cd frontend && npm run build

# Run Go tests
test:
	go test ./internal/...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf frontend/dist/*
	rm -rf build/

# Cross-compile CLI for multiple platforms (stripped)
# UPX: Linux only — Windows triggers AV false positives, macOS breaks Gatekeeper
release-cli:
	rm -rf bin/
	GOOS=linux   GOARCH=amd64 go build -tags cli -ldflags '$(LDFLAGS)' -o bin/mmcd-cli-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -tags cli -ldflags '$(LDFLAGS)' -o bin/mmcd-cli-linux-arm64 .
	GOOS=darwin  GOARCH=arm64 go build -tags cli -ldflags '$(LDFLAGS)' -o bin/mmcd-cli-darwin-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -tags cli -ldflags '$(LDFLAGS)' -o bin/mmcd-cli-darwin-amd64 .
	GOOS=windows GOARCH=amd64 go build -tags cli -ldflags '$(LDFLAGS)' -o bin/mmcd-cli-windows-amd64.exe .
	@UPX=""; \
	command -v upx >/dev/null 2>&1 && UPX=upx; \
	command -v upx-ucl >/dev/null 2>&1 && UPX=upx-ucl; \
	if [ -n "$$UPX" ]; then \
		echo "Compressing Linux binaries with $$UPX..."; \
		$$UPX --best bin/mmcd-cli-linux-amd64; \
		$$UPX --best bin/mmcd-cli-linux-arm64; \
		echo "✓ UPX compression complete"; \
	else \
		echo "Note: UPX not found, skipping compression."; \
		echo "  Install with: sudo apt install upx-ucl  (or: sudo apt install upx)"; \
	fi

# Build desktop app for the current platform (stripped)
# Wails cannot cross-compile desktop apps (macOS requires macOS, etc.)
# Use GitHub Actions (release.yml) for multi-platform desktop builds.
release-desktop: check-wails frontend
	@echo "Building desktop app for $$(uname)..."
	@case "$$(uname)" in \
	Darwin) \
		wails build -platform darwin/universal -ldflags '$(LDFLAGS)'; \
		;; \
	Linux) \
		wails build -platform linux/amd64 -ldflags '$(LDFLAGS)'; \
		;; \
	MINGW*|MSYS*|CYGWIN*) \
		wails build -platform windows/amd64 -ldflags '$(LDFLAGS)'; \
		;; \
	*) \
		echo "ERROR: Unsupported platform: $$(uname)"; \
		exit 1; \
		;; \
	esac
	@echo "✓ Desktop build complete. Check build/bin/"

# Build everything for all platforms (CLI + desktop)
# Copies desktop build into bin/ alongside CLI binaries for a single release directory.
release: release-cli release-desktop
	@echo "Collecting release artifacts into bin/..."
	@case "$$(uname)" in \
	Darwin) \
		cp -r build/bin/mmcd.app bin/mmcd-desktop-darwin-universal.app; \
		;; \
	Linux) \
		cp build/bin/mmcd bin/mmcd-desktop-linux-amd64; \
		;; \
	MINGW*|MSYS*|CYGWIN*) \
		cp build/bin/mmcd.exe bin/mmcd-desktop-windows-amd64.exe; \
		;; \
	esac
	@echo ""
	@ls -lh bin/
	@echo ""
	@echo "✓ Full release build complete. All artifacts in bin/"
