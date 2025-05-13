# Example Makefile for using fo utility

.PHONY: build test lint clean all

all: lint test build

build:
	@fo -l "Building binary" -- go build -o bin/app ./main.go

test:
	@fo -l "Running tests" -- go test -v ./...

lint:
	@fo -l "Linting code" -- golangci-lint run ./...

# Example with stream mode for verbose output
test-verbose:
	@fo -l "Running verbose tests" -s -- go test -v ./...

# Example showing output regardless of success/failure
vet:
	@fo -l "Vetting code" --show-output always -- go vet ./...

clean:
	@fo -l "Cleaning up" -- rm -rf bin/
