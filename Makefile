.PHONY: setup build run clean lint format test

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)

# Default target
all: build

# Install development tools
setup:
	@echo "Installing development tools..."
	@go install mvdan.cc/gofumpt@latest
	@echo "✅ Development tools installed"

# Build the application
build:
	@echo "Building linkding-ssg..."
	@go build -ldflags "$(LDFLAGS)" -o linkding-ssg .
	@echo "✅ Built: linkding-ssg ($(VERSION))"

# Run the application
run: build
	./linkding-ssg

# Clean build and test artifacts
clean:
	@rm -f linkding-ssg linkding-ssg-*
	@rm -rf test_dist test_rss_dist test_eject test_build_check
	@echo "✅ Cleaned"

# Format code
format:
	@go fmt ./...
	@if command -v gofumpt >/dev/null 2>&1; then \
		gofumpt -l -w .; \
		echo "✅ Formatted with gofumpt"; \
	else \
		echo "ℹ️  Run 'make setup' to install gofumpt for strict formatting"; \
	fi

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race ./...
	@echo "✅ Tests complete"

# Eject embedded templates to data/ for customization
eject: build
	./linkding-ssg eject
