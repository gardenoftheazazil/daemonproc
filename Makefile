# ===========================================================================
# Garden of the Azazil — DaemonProc Makefile
# ===========================================================================

# Go parameters
GOCMD      := go
GOTEST     := $(GOCMD) test
GOBUILD    := $(GOCMD) build
GOVET      := $(GOCMD) vet
GOFMT      := gofmt
GOMOD      := $(GOCMD) mod
LINT       := golangci-lint

# Build parameters
BINARY     := daemonproc
BUILD_DIR  := ./bin
MAIN_PKG   := ./cmd/daemonproc

# Test parameters
TEST_FLAGS := -race -count=1 -v
COVER_FILE := coverage.out

# ===========================================================================
# Development
# ===========================================================================

.PHONY: all
all: fmt lint test build ## Run format, lint, test, and build.

.PHONY: fmt
fmt: ## Format all Go source files.
	@echo ">>> Formatting..."
	@$(GOFMT) -w -s .
	@echo "✓ Done"

.PHONY: lint
lint: ## Run golangci-lint on all packages.
	@echo ">>> Linting..."
	@$(LINT) run ./...
	@echo "✓ Done"

.PHONY: vet
vet: ## Run go vet on all packages.
	@echo ">>> Vetting..."
	@$(GOVET) ./...
	@echo "✓ Done"

.PHONY: test
test: ## Run all tests with race detector.
	@echo ">>> Testing..."
	@$(GOTEST) ./... $(TEST_FLAGS)
	@echo "✓ Done"

.PHONY: test-short
test-short: ## Run tests in short mode (skip long-running tests).
	@echo ">>> Testing (short)..."
	@$(GOTEST) ./... -short $(TEST_FLAGS)
	@echo "✓ Done"

.PHONY: cover
cover: ## Run tests with coverage report.
	@echo ">>> Coverage..."
	@$(GOTEST) ./... -coverprofile=$(COVER_FILE) -covermode=atomic $(TEST_FLAGS)
	@$(GOCMD) tool cover -html=$(COVER_FILE) -o coverage.html
	@echo "✓ Coverage report: coverage.html"

.PHONY: build
build: ## Build the daemon binary.
	@echo ">>> Building..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) -o $(BUILD_DIR)/$(BINARY) $(MAIN_PKG)
	@echo "✓ Built: $(BUILD_DIR)/$(BINARY)"

# ===========================================================================
# Dependencies
# ===========================================================================

.PHONY: deps
deps: ## Download and tidy Go module dependencies.
	@echo ">>> Downloading dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy
	@echo "✓ Done"

.PHONY: deps-upgrade
deps-upgrade: ## Upgrade all dependencies to latest minor/patch versions.
	@echo ">>> Upgrading dependencies..."
	@$(GOCMD) get -u ./...
	@$(GOMOD) tidy
	@echo "✓ Done"

# ===========================================================================
# CI / Quality Gates
# ===========================================================================

.PHONY: check
check: fmt lint vet test ## Run all quality checks (CI equivalent).
	@echo ""
	@echo "═══════════════════════════════════"
	@echo "  ✓ All checks passed"
	@echo "═══════════════════════════════════"

.PHONY: ci
ci: lint test ## Run CI checks only (lint + test, no format).

# ===========================================================================
# Cleanup
# ===========================================================================

.PHONY: clean
clean: ## Remove build artifacts and coverage files.
	@echo ">>> Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(COVER_FILE) coverage.html
	@echo "✓ Done"

# ===========================================================================
# Help
# ===========================================================================

.PHONY: help
help: ## Show this help message.
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
