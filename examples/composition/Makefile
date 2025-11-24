# Makefile demonstrating fo pattern composition in real-world build workflows
#
# This Makefile shows how to use fo's pattern-based design system
# to create comprehensive build dashboards that combine multiple
# visualization patterns.

.PHONY: help build test lint quality-dashboard build-dashboard all

# Default target
help:
	@echo "Available targets:"
	@echo "  build            - Build the project with fo formatting"
	@echo "  test             - Run tests with fo formatting"
	@echo "  lint             - Run linters with fo formatting"
	@echo "  quality-dashboard - Generate quality metrics dashboard"
	@echo "  build-dashboard  - Generate build metrics dashboard"
	@echo "  all              - Run build, test, and lint"

# Build target using fo for formatted output
build:
	@fo -l "Building application" -- go build ./...

# Test target using fo for formatted output
test:
	@fo -l "Running tests" -- go test -json ./...

# Lint target using fo for formatted output
lint:
	@fo -l "Running linter" -- golangci-lint run ./...

# Quality dashboard - composes multiple patterns to show quality metrics
quality-dashboard:
	@echo "Generating quality metrics dashboard..."
	@go run examples/composition/quality.go

# Build dashboard - composes multiple patterns to show build metrics
build-dashboard:
	@echo "Generating build metrics dashboard..."
	@go run examples/composition/dashboard.go

# Combined workflow - demonstrates pattern composition in CI-like pipeline
all: build test lint
	@echo ""
	@echo "═══════════════════════════════════════════════════════════"
	@echo "Build pipeline complete! All checks passed."
	@echo "═══════════════════════════════════════════════════════════"

# Example: Custom build with multiple steps
build-with-dashboard: build
	@go run examples/composition/dashboard.go

# Example: Quality check workflow
quality-check: test lint quality-dashboard
	@echo ""
	@echo "Quality check complete!"

