.PHONY: all build test lint fmt fmt-fix vet clean lint-sarif

all: build test lint ## Full QA pass

build: ## Install binary to $GOBIN
	go install ./cmd/fo

test: ## Run tests with race detector and coverage
	go test -race -cover ./...

lint: vet ## Run all linters (vet + golangci-lint)
	golangci-lint run --output.text.path=stdout ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Check formatting (exits non-zero if unformatted)
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

fmt-fix: ## Fix formatting in-place
	gofmt -w .

lint-sarif: vet ## Run linters with SARIF output (pipe through fo)
	golangci-lint run --output.sarif.path=stdout ./...

clean: ## Remove build artifacts
	rm -f fo
	rm -rf build/

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
