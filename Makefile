# golem — Run 'make' or 'make help' to see available commands

.DEFAULT_GOAL := help

BIN         := ./bin
BINARY      := $(BIN)/golem
MODULE      := github.com/mab-go/golem
VERSION_PKG := $(MODULE)/internal/version
GOLANGCI    := $(BIN)/golangci-lint
GOIMPORTS   := $(BIN)/goimports
GOCYCLO     := $(BIN)/gocyclo
# Pinned golangci-lint release for reproducible `make lint`; bump if unsupported on current Go (see go.mod).
# v2.6.x binaries were built with Go 1.25 and reject go.mod go 1.26+; use v2.9+ for Go 1.26 toolchains.
GOLANGCI_LINT_VERSION ?= v2.11.3
# Pinned goimports (golang.org/x/tools); bump if `make fmt` fails or is incompatible with go.mod Go version.
GOIMPORTS_VERSION ?= v0.38.0
# Pinned gocyclo (github.com/fzipp/gocyclo); bump for `make cyclo` reproducibility.
GOCYCLO_VERSION ?= v0.6.0
# Pinned protoc Go plugins for reproducible proto generation.
PROTOC_GEN_GO_VERSION      ?= v1.36.6
PROTOC_GEN_GO_GRPC_VERSION ?= v1.5.1
# protoc binary — override if not on PATH (e.g. PROTOC=/path/to/protoc).
PROTOC ?= protoc

# golangci-lint must be built with Go >= go.mod; auto follows deps' older go version, so pin to module Go.
GO_MOD_VERSION := $(shell grep -E '^go ' go.mod | head -1 | awk '{print $$2}')
TOOLCHAIN_FOR_TOOLS ?= go$(GO_MOD_VERSION)

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
DATE    := $(shell date -u +%Y-%m-%d)

LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION) \
           -X $(VERSION_PKG).Commit=$(COMMIT) \
           -X $(VERSION_PKG).Date=$(DATE)

RACE ?= 1
COVER ?= 0
OPEN ?= $(shell command -v xdg-open 2>/dev/null || echo "open")

.PHONY: help \
        setup setup\:go setup\:sidecar \
        proto proto\:go proto\:ts \
        build build\:go build\:sidecar build\:tui install run \
        test test\:cover test\:sidecar \
        lint lint\:fix fmt vet cyclo \
        mod\:tidy mod\:verify \
        clean clean\:cache clean\:all \
        docs docs\:check \
        versions

#------------------------------------------------------------------------------
# Help
#------------------------------------------------------------------------------

help: ## Show available commands
	@awk '\
		/^#-+$$/ { next } \
		/^# [A-Za-z]/ { section = substr($$0, 3); next } \
		/^[a-zA-Z_:\\-]+:.*## / { \
			match($$0, /## /); \
			desc = substr($$0, RSTART + 3); \
			prefix = substr($$0, 1, RSTART - 1); \
			gsub(/\\:/, "\x01", prefix); \
			sub(/: .*/, "", prefix); \
			gsub(/\x01/, ":", prefix); \
			target = prefix; \
			targets[section] = targets[section] sprintf("  \033[36m%-22s\033[0m %s\n", target, desc); \
			order[section] = order[section] ? order[section] : ++count; \
		} \
		END { \
			for (i = 1; i <= count; i++) { \
				for (s in order) { \
					if (order[s] == i) { \
						if (i > 1) printf "\n"; \
						printf "\033[1m%s\033[0m\n", s; \
						printf "%s", targets[s]; \
					} \
				} \
			} \
		}' $(MAKEFILE_LIST)

#------------------------------------------------------------------------------
# Setup
#------------------------------------------------------------------------------

setup: setup\:go setup\:sidecar ## Install Go tools and sidecar dependencies

setup\:go: ## Install required Go tools into ./bin (project-local)
	@mkdir -p $(BIN)
	GOTOOLCHAIN=$(TOOLCHAIN_FOR_TOOLS) GOBIN=$(abspath $(BIN)) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	GOTOOLCHAIN=$(TOOLCHAIN_FOR_TOOLS) GOBIN=$(abspath $(BIN)) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	GOTOOLCHAIN=$(TOOLCHAIN_FOR_TOOLS) GOBIN=$(abspath $(BIN)) go install github.com/fzipp/gocyclo/cmd/gocyclo@$(GOCYCLO_VERSION)
	GOTOOLCHAIN=$(TOOLCHAIN_FOR_TOOLS) GOBIN=$(abspath $(BIN)) go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOTOOLCHAIN=$(TOOLCHAIN_FOR_TOOLS) GOBIN=$(abspath $(BIN)) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	@echo ""
	@echo "Setup complete: $(GOLANGCI), $(GOIMPORTS), $(GOCYCLO), protoc-gen-go, protoc-gen-go-grpc"
	@echo ""

setup\:sidecar: ## Install sidecar npm dependencies and patch prismarine-viewer
	cd sidecar && npm install

#------------------------------------------------------------------------------
# Proto
#------------------------------------------------------------------------------

proto: proto\:go proto\:ts ## Generate Go + TypeScript code from proto

proto\:go: ## Generate Go code from proto/minecraft.proto
	@mkdir -p internal/grpc/pb
	PATH="$(abspath $(BIN)):$$PATH" $(PROTOC) \
		--proto_path=proto \
		--go_out=internal/grpc/pb --go_opt=paths=source_relative \
		--go-grpc_out=internal/grpc/pb --go-grpc_opt=paths=source_relative \
		proto/minecraft.proto

proto\:ts: ## Generate TypeScript code from proto/minecraft.proto
	cd sidecar && PATH="$(abspath $(BIN)):$$PATH" npm run generate:proto

#------------------------------------------------------------------------------
# Build
#------------------------------------------------------------------------------

build: build\:go build\:sidecar build\:tui ## Build Go binary, sidecar, and TUI

build\:go: ## Build Go binary to ./bin/golem with version ldflags
	@mkdir -p $(BIN)
	go build -o $(BINARY) -ldflags "$(LDFLAGS)" ./cmd/golem

build\:sidecar: ## Build sidecar TypeScript
	cd sidecar && npm run build

build\:tui: ## Build TUI binary to ./bin/golem-tui with version ldflags
	@mkdir -p $(BIN)
	go build -o $(BIN)/golem-tui -ldflags "$(LDFLAGS)" ./cmd/golem-tui

install: ## Run go install with same ldflags as 'build' target
	go install -ldflags "$(LDFLAGS)" ./cmd/golem

#------------------------------------------------------------------------------
# Run
#------------------------------------------------------------------------------

run: ## Run via go run (optional: ARGS="--flags")
	go run -ldflags "$(LDFLAGS)" ./cmd/golem $(ARGS)

#------------------------------------------------------------------------------
# Test
#------------------------------------------------------------------------------

test: ## Run all tests (RACE=1 default; COVER=1 for coverage; RACE=0 to disable -race)
	go test $(if $(filter 1,$(RACE)),-race,) $(if $(filter 1,$(COVER)),-coverprofile=coverage.out -covermode=atomic,) ./...
ifeq ($(COVER),1)
	go tool cover -html=coverage.out -o coverage.html
endif
	cd sidecar && npx vitest run $(if $(filter 1,$(COVER)),--coverage,)
ifeq ($(COVER),1)
	@if [ -z "$$CI" ]; then $(OPEN) coverage.html; $(OPEN) sidecar/coverage/index.html; else echo "Wrote coverage.html and sidecar/coverage/index.html"; fi
endif

test\:cover: ## Go coverage report; opens HTML unless CI is set (override OPEN=...)
	go test $(if $(filter 1,$(RACE)),-race,) -coverprofile=coverage.out -covermode=atomic ./...
	grep -v 'internal/grpc/pb/' coverage.out > coverage.filtered.out
	go tool cover -func=coverage.filtered.out
	go tool cover -html=coverage.filtered.out -o coverage.html
	@if [ -z "$$CI" ]; then $(OPEN) coverage.html; else echo "Wrote coverage.html (CI set, skipping browser)"; fi

test\:sidecar: ## Run sidecar unit tests (COVER=1 for coverage)
	cd sidecar && npx vitest run $(if $(filter 1,$(COVER)),--coverage,)
ifeq ($(COVER),1)
	@if [ -z "$$CI" ]; then $(OPEN) sidecar/coverage/index.html; else echo "Wrote sidecar/coverage/index.html"; fi
endif

#------------------------------------------------------------------------------
# Lint and Format
#------------------------------------------------------------------------------

lint: ## Run golangci-lint
	@test -x $(GOLANGCI) || (echo "Run 'make setup' to install golangci-lint" && exit 1)
	$(GOLANGCI) run ./...

lint\:fix: ## Run golangci-lint with --fix
	@test -x $(GOLANGCI) || (echo "Run 'make setup' to install golangci-lint" && exit 1)
	$(GOLANGCI) run --fix ./...

fmt: ## Format Go (goimports) and sidecar (Prettier)
	@test -x $(GOIMPORTS) || (echo "Run 'make setup' to install goimports into ./bin" && exit 1)
	$(GOIMPORTS) -l -w .
	@test -f sidecar/node_modules/.bin/prettier || (echo "Run 'make setup' to install sidecar dependencies (includes Prettier)" && exit 1)
	cd sidecar && npm run format

vet: ## Run go vet
	go vet ./...

cyclo: ## Run gocyclo; run 'make setup' first
	@test -x $(GOCYCLO) || (echo "Run 'make setup' to install gocyclo" && exit 1)
	$(GOCYCLO) -over 10 .

#------------------------------------------------------------------------------
# Module
#------------------------------------------------------------------------------

mod\:tidy: ## Run go mod tidy
	go mod tidy

mod\:verify: ## Run go mod verify
	go mod verify

#------------------------------------------------------------------------------
# Documentation
#------------------------------------------------------------------------------

docs: ## Regenerate auto-populated README sections
	cd tools/docgen && go run . -root $(CURDIR)

docs\:check: ## Dry-run: fail if generated README sections are stale
	cd tools/docgen && go run . -root $(CURDIR) -check

#------------------------------------------------------------------------------
# Clean
#------------------------------------------------------------------------------

clean: ## Remove built binaries and coverage artifacts
	rm -f $(BINARY) $(BIN)/golem-tui coverage.out coverage.filtered.out coverage.html
	rm -rf sidecar/coverage

clean\:cache: ## Clear Go test cache
	go clean -testcache

clean\:all: clean ## Run clean plus remove ./bin (Go tools)
	rm -rf $(BIN)

#------------------------------------------------------------------------------
# Utilities
#------------------------------------------------------------------------------

versions: ## Show Go and required tool versions
	@echo "Go: $$(go version)"
	@if test -x $(GOLANGCI); then $(GOLANGCI) version; else echo "golangci-lint: not installed (run make setup)"; fi
	@if test -x $(GOIMPORTS); then echo "goimports (module metadata):"; go version -m $(GOIMPORTS) 2>&1 | head -4; else echo "goimports: not installed (run make setup)"; fi
	@if test -x $(GOCYCLO); then echo "gocyclo (module metadata):"; go version -m $(GOCYCLO) 2>&1 | head -4; else echo "gocyclo: not installed (run make setup)"; fi
