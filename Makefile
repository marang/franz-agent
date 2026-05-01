SHELL := /bin/sh

CGO_ENABLED ?= 0
GOEXPERIMENT ?= greenteagc
VERSION ?= $(shell git describe --long 2>/dev/null || echo "")
DIST_DIR ?= dist
BINARY ?= franz-agent
BUILD_FLAGS ?= -trimpath -buildvcs=false
LDFLAGS := -ldflags="-s -w $(if $(VERSION),-X github.com/marang/franz-agent/internal/version.Version=$(VERSION),)"

.PHONY: help build build-root build-dist run test test-update-golden lint schema deps-upgrade

help:
	@echo "Available targets:"
	@echo "  make build               Build $(BINARY) into $(DIST_DIR)/"
	@echo "  make build-root          Build $(BINARY) in project root"
	@echo "  make build-dist          Alias for build"
	@echo "  make run                 Build and run franz-agent (pass ARGS='...')"
	@echo "  make test                Run all tests"
	@echo "  make test-update-golden  Update golden snapshots"
	@echo "  make lint                Run go vet and golangci-lint"
	@echo "  make schema              Regenerate schema.json"
	@echo "  make deps-upgrade        Upgrade dependencies and run tests"

build:
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOEXPERIMENT=$(GOEXPERIMENT) go build -v $(BUILD_FLAGS) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY) .

build-root:
	CGO_ENABLED=$(CGO_ENABLED) GOEXPERIMENT=$(GOEXPERIMENT) go build -v $(BUILD_FLAGS) $(LDFLAGS) -o ./$(BINARY) .

build-dist: build

run: build
	./$(DIST_DIR)/$(BINARY) $(ARGS)

test:
	CGO_ENABLED=1 GOEXPERIMENT=$(GOEXPERIMENT) go test -race -failfast ./...
	CGO_ENABLED=$(CGO_ENABLED) GOEXPERIMENT=$(GOEXPERIMENT) go test ./internal/ui/diffview

test-update-golden:
	CGO_ENABLED=$(CGO_ENABLED) GOEXPERIMENT=$(GOEXPERIMENT) go test ./internal/ui/diffview -update

lint:
	CGO_ENABLED=$(CGO_ENABLED) GOEXPERIMENT=$(GOEXPERIMENT) go vet ./...
	CGO_ENABLED=$(CGO_ENABLED) GOEXPERIMENT=$(GOEXPERIMENT) golangci-lint run

schema:
	go run main.go schema > schema.json
	@echo "Generated schema.json"

deps-upgrade:
	go list -m -u all
	go get -u=patch ./...
	go get -u ./...
	go mod tidy
	$(MAKE) test
