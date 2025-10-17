# Makefile for Knot DNS Exporter

# Build variables
VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GO_VERSION ?= $(shell go version | awk '{print $3}')

# Go build flags
LDFLAGS = -X main.version=$(VERSION) \
          -X main.buildTime=$(BUILD_TIME) \
          -X main.gitCommit=$(GIT_COMMIT)

# Build flags for CGO
CGO_ENABLED = 1

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	CGO_ENABLED=$(CGO_ENABLED) \
	go build -ldflags "$(LDFLAGS)" -o knot-exporter .

# Build with race detector
.PHONY: build-race
build-race:
	CGO_ENABLED=$(CGO_ENABLED) \
	CGO_CFLAGS="$(CGO_CFLAGS)" \
	CGO_LDFLAGS="$(CGO_LDFLAGS)" \
	go build -race -ldflags "$(LDFLAGS)" -o knot-exporter .

# Static build (if supported)
.PHONY: build-static
build-static:
	CGO_ENABLED=$(CGO_ENABLED) \
	CGO_CFLAGS="$(CGO_CFLAGS)" \
	CGO_LDFLAGS="$(CGO_LDFLAGS) -static" \
	go build -ldflags "$(LDFLAGS) -extldflags '-static'" -o knot-exporter .

# Test
.PHONY: test
test:
	go test -v ./...

# Test with race detector
.PHONY: test-race
test-race:
	go test -race -v ./...

# Clean
.PHONY: clean
clean:
	rm -f knot-exporter

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod verify

# Update dependencies
.PHONY: update-deps
update-deps:
	go get -u ./...
	go mod tidy

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run

# Vet code
.PHONY: vet
vet:
	go vet ./...

# Directory for security-related scripts and outputs
SECURITY_DIR := .security

# Security scan with filtered gosec results
.PHONY: security
security:
	@command -v gosec >/dev/null 2>&1 || { echo >&2 "gosec is required but not installed. Run: go install github.com/securego/gosec/v2/cmd/gosec@latest"; exit 1; }
	@command -v jq >/dev/null 2>&1 || { echo >&2 "jq is required but not installed. Run: apt-get install jq or yum install jq"; exit 1; }
	@mkdir -p $(SECURITY_DIR)
	@echo "Running security scan..."
	@gosec -quiet -fmt=json -out=$(SECURITY_DIR)/gosec-output.json ./... || true
	@echo "Filtering results..."
	@jq '.Issues = (.Issues | map(select((.file | contains(".cache/go-build") | not))))' \
		$(SECURITY_DIR)/gosec-output.json > $(SECURITY_DIR)/filtered-output.json
	@FILTERED_COUNT=$$(jq '.Issues | length' $(SECURITY_DIR)/filtered-output.json); \
	if [ "$$FILTERED_COUNT" -gt 0 ]; then \
		echo "Found $$FILTERED_COUNT security issues after filtering:"; \
		jq -r '.Issues[] | "[\(.file):\(.line)] - \(.rule_id) (CWE-\(.cwe.id)): \(.details) (Confidence: \(.confidence), Severity: \(.severity))\n\(.code)"' $(SECURITY_DIR)/filtered-output.json; \
		exit 1; \
	else \
		echo "No security issues found after filtering !"; \
	fi

# Check dependencies
.PHONY: check-deps
check-deps:
	@command -v pkg-config >/dev/null 2>&1 || { echo >&2 "pkg-config is required but not installed. Aborting."; exit 1; }
	@pkg-config --exists libknot || { echo >&2 "libknot development files are required but not found. Aborting."; exit 1; }
	@echo "Dependencies check passed"

# Install (requires root for systemd service)
.PHONY: install
install: build
	install -D -m 755 knot-exporter /usr/local/bin/knot-exporter

# Show version information
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Go Version: $(GO_VERSION)"

# Development build with debug symbols
.PHONY: dev
dev:
	CGO_ENABLED=$(CGO_ENABLED) \
	CGO_CFLAGS="$(CGO_CFLAGS) -g" \
	CGO_LDFLAGS="$(CGO_LDFLAGS)" \
	go build -gcflags="all=-N -l" -ldflags "$(LDFLAGS)" -o knot-exporter .

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  build-race  - Build with race detector"
	@echo "  build-static- Build static binary"
	@echo "  test        - Run tests"
	@echo "  test-race   - Run tests with race detector"
	@echo "  clean       - Remove built binary"
	@echo "  deps        - Download dependencies"
	@echo "  update-deps - Update dependencies"
	@echo "  fmt         - Format code"
	@echo "  lint        - Lint code (requires golangci-lint)"
	@echo "  vet         - Vet code"
	@echo "  check-deps  - Check if required dependencies are installed"
	@echo "  install     - Install binary to /usr/local/bin/"
	@echo "  version     - Show version information"
	@echo "  dev         - Build development version with debug symbols"
	@echo "  help        - Show this help"

# GoReleaser targets
.PHONY: release-snapshot
release-snapshot:
	goreleaser release --snapshot --rm-dist

.PHONY: release-check
release-check:
	goreleaser check

.PHONY: release
release:
	goreleaser release --clean
