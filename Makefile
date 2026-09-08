# Every target here is what CI runs, so `make ci` locally == the PR pipeline.

BINARY      := server
PKG         := github.com/ValentinoTriadi/ci-cd-example
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
IMAGE       ?= ghcr.io/valentinotriadi/ci-cd-example
COVERAGE_MIN ?= 80

LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.BuildDate=$(BUILD_DATE)

GO_FILES := $(shell find . -name '*.go' -not -path './vendor/*')

.DEFAULT_GOAL := help
.PHONY: help fmt fmt-check vet lint test test-short cover build build-all run docker docker-run smoke tidy-check clean ci

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

fmt: ## Rewrite files with gofmt
	gofmt -w $(GO_FILES)

fmt-check: ## Fail if any file is not gofmt-clean
	@unformatted=$$(gofmt -l $(GO_FILES)); \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt-clean:"; echo "$$unformatted"; \
		gofmt -d $$unformatted; exit 1; \
	fi
	@echo "gofmt: clean"

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (falls back to the official Docker image)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout 5m; \
	else \
		echo "golangci-lint not installed locally, running via Docker"; \
		docker run --rm -v "$$(pwd)":/app -w /app golangci/golangci-lint:v2.13.2 golangci-lint run --timeout 5m; \
	fi

test: ## Run all tests with the race detector and coverage
	go test ./... -race -covermode=atomic -coverprofile=coverage.out

test-short: ## Run tests without the race detector (fast feedback)
	go test ./... -short

cover: test ## Run tests and enforce the coverage threshold
	@COVERAGE_MIN=$(COVERAGE_MIN) ./scripts/coverage.sh coverage.out

build: ## Build the binary for the host platform into bin/
	mkdir -p bin
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/server

build-all: ## Cross-compile release archives into dist/
	./scripts/build-release.sh

run: ## Run the server locally
	go run -ldflags="$(LDFLAGS)" ./cmd/server

docker: ## Build the container image for the host platform
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE):local .

docker-run: docker ## Build and run the container image
	docker run --rm -p 8080:8080 $(IMAGE):local

smoke: ## Smoke-test a running server (BASE_URL overrides the target)
	./scripts/smoke-test.sh $(or $(BASE_URL),http://localhost:8080)

tidy-check: ## Fail if go.mod/go.sum are not tidy
	go mod tidy
	git diff --exit-code -- go.mod go.sum

clean: ## Remove build and coverage output
	rm -rf bin dist coverage.out coverage.html

ci: fmt-check vet lint cover build ## Everything the PR pipeline runs
