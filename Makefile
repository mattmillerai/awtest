VERSION ?= dev
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)"

.DEFAULT_GOAL := help

.PHONY: build test test-coverage lint install clean snapshot help

build: ## Build awtest binary with version embedding
	go build $(LDFLAGS) -o awtest ./cmd/awtest

test: ## Run all tests with race detector and coverage
	go test -v -race -coverprofile=coverage.out ./...

test-coverage: ## View coverage report in browser (run 'make test' first)
	go tool cover -html=coverage.out

lint: ## Run linter (requires golangci-lint)
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Please install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1)
	golangci-lint run

install: ## Install awtest to $GOPATH/bin
	go install $(LDFLAGS) ./cmd/awtest

clean: ## Remove build artifacts
	rm -f awtest coverage.out
	rm -rf dist/

snapshot: ## Build local multi-platform snapshot with GoReleaser (requires goreleaser)
	@which goreleaser > /dev/null || (echo "goreleaser not found. Please install: brew install goreleaser/tap/goreleaser"; exit 1)
	goreleaser build --snapshot --clean

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
