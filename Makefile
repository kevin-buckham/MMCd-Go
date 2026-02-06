.PHONY: build dev clean test cli frontend install

# Ensure GOPATH/bin is in PATH so wails CLI is found
export PATH := $(PATH):$(shell go env GOPATH)/bin

# Install dependencies
install:
	cd frontend && npm install
	go mod tidy

# Build the full Wails desktop app
build: frontend
	wails build

# Run in Wails dev mode (hot reload)
dev:
	wails dev

# Build just the CLI (no frontend/Wails dependency)
cli:
	go build -tags cli -o bin/mmcd ./...

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

# Cross-compile CLI for multiple platforms
release-cli:
	GOOS=linux GOARCH=amd64 go build -tags cli -o bin/mmcd-linux-amd64 ./...
	GOOS=linux GOARCH=arm64 go build -tags cli -o bin/mmcd-linux-arm64 ./...
	GOOS=darwin GOARCH=arm64 go build -tags cli -o bin/mmcd-darwin-arm64 ./...
	GOOS=windows GOARCH=amd64 go build -tags cli -o bin/mmcd-windows-amd64.exe ./...
