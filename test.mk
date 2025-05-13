# test.mk - Test makefile for fo utility

# Build fo first
.PHONY: build-fo
build-fo:
	@mkdir -p bin
	@go build -o bin/fo ./cmd
	@echo "fo utility built successfully"

# Simple command example
.PHONY: simple
simple: build-fo
	@bin/fo -- echo "This is a simple command"

# Command with custom label
.PHONY: labeled
labeled: build-fo
	@bin/fo -l "Custom Label Test" -- echo "This command has a custom label"

# Stream mode example
.PHONY: stream
stream: build-fo
	@bin/fo -s -- bash -c "for i in {1..5}; do echo \"Line \$$i\"; sleep 0.2; done"

# Show output regardless of success
.PHONY: show-always
show-always: build-fo
	@bin/fo --show-output always -- echo "This output is always shown"

# Test with a failing command
.PHONY: fail
fail: build-fo
	@bin/fo -- bash -c "echo 'About to fail'; exit 1" || echo "Command failed as expected"

# Test preset label (for go commands)
.PHONY: preset-go
preset-go: build-fo
	@bin/fo -- go version

# Test various flag combinations
.PHONY: no-timer
no-timer: build-fo
	@bin/fo --no-timer -- echo "Command with no timer displayed"

.PHONY: no-color
no-color: build-fo
	@bin/fo --no-color -- echo "Command with no color output"

.PHONY: ci-mode
ci-mode: build-fo
	@bin/fo --ci -- echo "Command in CI mode (plain text, no color, no timer)"

# Combined target to run all tests
.PHONY: test-all
test-all: simple labeled stream show-always fail preset-go no-timer no-color ci-mode
	@echo "All tests completed"

# Default target
.PHONY: all
all: build-fo test-all