# Makefile for fo utility
.PHONY: build clean help

# Variables
SERVICE_NAME := fo
BINARY_NAME  := $(SERVICE_NAME)
MODULE_PATH  := github.com/davidkoosis/fo
CMD_PATH     := ./cmd
BIN_PATH     := ./bin

# Build variables
VERSION := $(shell git describe --tags --always --dirty --match=v* 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-s -w \
          -X '$(MODULE_PATH)/internal/version.Version=$(VERSION)' \
          -X '$(MODULE_PATH)/internal/version.CommitHash=$(COMMIT)' \
          -X '$(MODULE_PATH)/internal/version.BuildDate=$(DATE)'"

# Default target
all: build

build:
	@mkdir -p $(BIN_PATH)
	@echo "Building $(BINARY_NAME)..."
	@go build $(LDFLAGS) -o $(BIN_PATH)/$(BINARY_NAME) $(CMD_PATH)
	@echo "Built: $(BIN_PATH)/$(BINARY_NAME)"

clean:
	@rm -rf $(BIN_PATH)
	@go clean -cache
	@echo "Cleaned"

help:
	@echo "$(SERVICE_NAME) Makefile"
	@echo ""
	@echo "  make        Build the binary"
	@echo "  make clean  Remove build artifacts"
	@echo "  make help   Show this message"
