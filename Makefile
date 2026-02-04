.PHONY: help test test-race lint vuln fmt vet build clean install

# Default target
help:
	@echo "Available targets:"
	@echo "  make test       - Run all tests"
	@echo "  make test-race  - Run tests with race detector"
	@echo "  make lint       - Run linters (golangci-lint)"
	@echo "  make vuln       - Run security vulnerability checks (govulncheck)"
	@echo "  make fmt        - Format code with gofmt"
	@echo "  make vet        - Run go vet"
	@echo "  make build      - Build the binary"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make install    - Install golangci-lint and govulncheck"

# Run all tests
test:
	@echo "Running tests..."
	go test ./... -v

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	go test -race ./... -v

# Run linters
lint:
	@echo "Running linters..."
	@GOPATH=$$(go env GOPATH); \
	GOLANGCI_LINT=$$(command -v golangci-lint 2>/dev/null || echo "$$GOPATH/bin/golangci-lint"); \
	if [ ! -f "$$GOLANGCI_LINT" ]; then \
		echo "golangci-lint not found. Install it with: make install"; \
		echo "If you just installed it, make sure $$GOPATH/bin is in your PATH"; \
		exit 1; \
	fi
	@GOPATH=$$(go env GOPATH); \
	GOLANGCI_LINT=$$(command -v golangci-lint 2>/dev/null || echo "$$GOPATH/bin/golangci-lint"); \
	$$GOLANGCI_LINT run ./... --timeout=5m

# Run security vulnerability checks
vuln:
	@echo "Running security vulnerability checks..."
	@GOPATH=$$(go env GOPATH); \
	GOVULNCHECK=$$(command -v govulncheck 2>/dev/null || echo "$$GOPATH/bin/govulncheck"); \
	if [ ! -f "$$GOVULNCHECK" ]; then \
		echo "govulncheck not found. Install it with: make install"; \
		echo "If you just installed it, make sure $$GOPATH/bin is in your PATH"; \
		exit 1; \
	fi
	@GOPATH=$$(go env GOPATH); \
	GOVULNCHECK=$$(command -v govulncheck 2>/dev/null || echo "$$GOPATH/bin/govulncheck"); \
	$$GOVULNCHECK ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Checking if code is formatted..."
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "Code is not formatted. Run 'go fmt ./...' to fix."; \
		gofmt -s -d .; \
		exit 1; \
	fi

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Build the binary
build:
	@echo "Building binary..."
	go build -o tnr

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f tnr
	rm -f coverage.out coverage.html
	go clean ./...

# Install development tools
install:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	@GOPATH=$$(go env GOPATH); \
	echo "Tools installed successfully!"; \
	echo "Make sure $$GOPATH/bin is in your PATH (add to ~/.zshrc if needed):"; \
	echo "  export PATH=\"$$PATH:$$GOPATH/bin\""

