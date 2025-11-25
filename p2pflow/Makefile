# P2PFlow Makefile
# Project configuration
PROJECT_NAME := p2pflow
BINARY_NAME := p2pflow
MODULE := github.com/JerryLegend254/p2pflow
CMD_PATH := ./cmd/p2pflowcli

# Version information (can be overridden by git tags)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build directories
BUILD_DIR := bin
DIST_DIR := dist

# Go build flags
LDFLAGS := -s -w \
	-X 'main.version=$(VERSION)' \
	-X 'main.commit=$(COMMIT)' \
	-X 'main.buildDate=$(BUILD_DATE)'

# Go commands
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOVET := $(GOCMD) vet
GOFMT := $(GOCMD) fmt
GOINSTALL := $(GOCMD) install

# Supported platforms for cross-compilation
PLATFORMS := \
	darwin/amd64 \
	darwin/arm64 \
	linux/amd64 \
	linux/arm64 \
	windows/amd64

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help message
	@echo "$(COLOR_BOLD)P2PFlow Build System$(COLOR_RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(COLOR_BLUE)<target>$(COLOR_RESET)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: build
build: ## Build the binary for current platform
	@echo "$(COLOR_GREEN)Building $(BINARY_NAME) $(VERSION)...$(COLOR_RESET)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)/*.go
	@echo "$(COLOR_GREEN)✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)$(COLOR_RESET)"

.PHONY: build-dev
build-dev: ## Build with debug symbols (no optimization)
	@echo "$(COLOR_GREEN)Building $(BINARY_NAME) (development mode)...$(COLOR_RESET)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)/*.go
	@echo "$(COLOR_GREEN)✓ Development build complete$(COLOR_RESET)"

.PHONY: install
install: build ## Build and install to $GOPATH/bin
	@echo "$(COLOR_GREEN)Installing $(BINARY_NAME)...$(COLOR_RESET)"
	$(GOINSTALL) -ldflags "$(LDFLAGS)" $(CMD_PATH)
	@echo "$(COLOR_GREEN)✓ Installed to $$($(GOCMD) env GOPATH)/bin/$(BINARY_NAME)$(COLOR_RESET)"

.PHONY: run
run: build ## Build and run the application
	@echo "$(COLOR_GREEN)Running $(BINARY_NAME)...$(COLOR_RESET)"
	./$(BUILD_DIR)/$(BINARY_NAME)

##@ Testing

.PHONY: test
test: ## Run all tests
	@echo "$(COLOR_GREEN)Running tests...$(COLOR_RESET)"
	$(GOTEST) -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	@echo "$(COLOR_GREEN)✓ Tests complete$(COLOR_RESET)"

.PHONY: test-short
test-short: ## Run tests (skip long-running tests)
	@echo "$(COLOR_GREEN)Running short tests...$(COLOR_RESET)"
	$(GOTEST) -v -short ./...

.PHONY: test-coverage
test-coverage: test ## Run tests with coverage report
	@echo "$(COLOR_GREEN)Generating coverage report...$(COLOR_RESET)"
	$(GOCMD) tool cover -html=coverage.txt -o coverage.html
	@echo "$(COLOR_GREEN)✓ Coverage report: coverage.html$(COLOR_RESET)"

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(COLOR_GREEN)Running integration tests...$(COLOR_RESET)"
	./scripts/test-bidirectional-sync.sh
	./scripts/test-dev-sync.sh

.PHONY: bench
bench: ## Run benchmarks
	@echo "$(COLOR_GREEN)Running benchmarks...$(COLOR_RESET)"
	$(GOTEST) -bench=. -benchmem ./...

##@ Code Quality

.PHONY: fmt
fmt: ## Format Go code
	@echo "$(COLOR_GREEN)Formatting code...$(COLOR_RESET)"
	$(GOFMT) ./...
	@echo "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

.PHONY: vet
vet: ## Run go vet
	@echo "$(COLOR_GREEN)Running go vet...$(COLOR_RESET)"
	$(GOVET) ./...
	@echo "$(COLOR_GREEN)✓ Vet complete$(COLOR_RESET)"

.PHONY: lint
lint: ## Run staticcheck linter
	@echo "$(COLOR_GREEN)Running staticcheck...$(COLOR_RESET)"
	@command -v staticcheck >/dev/null 2>&1 || { \
		echo "$(COLOR_YELLOW)Installing staticcheck...$(COLOR_RESET)"; \
		$(GOINSTALL) honnef.co/go/tools/cmd/staticcheck@latest; \
	}
	staticcheck ./...
	@echo "$(COLOR_GREEN)✓ Lint complete$(COLOR_RESET)"

.PHONY: check
check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)
	@echo "$(COLOR_GREEN)✓ All checks passed$(COLOR_RESET)"

##@ Dependencies

.PHONY: deps
deps: ## Download dependencies
	@echo "$(COLOR_GREEN)Downloading dependencies...$(COLOR_RESET)"
	$(GOMOD) download
	@echo "$(COLOR_GREEN)✓ Dependencies downloaded$(COLOR_RESET)"

.PHONY: deps-tidy
deps-tidy: ## Tidy dependencies
	@echo "$(COLOR_GREEN)Tidying dependencies...$(COLOR_RESET)"
	$(GOMOD) tidy
	@echo "$(COLOR_GREEN)✓ Dependencies tidied$(COLOR_RESET)"

.PHONY: deps-upgrade
deps-upgrade: ## Upgrade all dependencies
	@echo "$(COLOR_GREEN)Upgrading dependencies...$(COLOR_RESET)"
	$(GOCMD) get -u ./...
	$(GOMOD) tidy
	@echo "$(COLOR_GREEN)✓ Dependencies upgraded$(COLOR_RESET)"

.PHONY: deps-verify
deps-verify: ## Verify dependencies
	@echo "$(COLOR_GREEN)Verifying dependencies...$(COLOR_RESET)"
	$(GOMOD) verify
	@echo "$(COLOR_GREEN)✓ Dependencies verified$(COLOR_RESET)"

##@ Cross-platform Builds

.PHONY: build-all
build-all: clean ## Build for all supported platforms
	@echo "$(COLOR_GREEN)Building for all platforms...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)
	@$(foreach platform,$(PLATFORMS), \
		$(call build_platform,$(platform)) \
	)
	@echo "$(COLOR_GREEN)✓ All platform builds complete$(COLOR_RESET)"
	@ls -lh $(DIST_DIR)/

define build_platform
	$(eval OS := $(word 1,$(subst /, ,$(1))))
	$(eval ARCH := $(word 2,$(subst /, ,$(1))))
	$(eval OUTPUT := $(DIST_DIR)/$(BINARY_NAME)-$(OS)-$(ARCH)$(if $(filter windows,$(OS)),.exe,))
	@echo "$(COLOR_BLUE)→ Building $(OS)/$(ARCH)...$(COLOR_RESET)"
	@GOOS=$(OS) GOARCH=$(ARCH) $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(OUTPUT) $(CMD_PATH)/*.go
	@if [ "$(OS)" = "darwin" ]; then \
		codesign -s - $(OUTPUT) 2>/dev/null || true; \
	fi
endef

.PHONY: build-linux
build-linux: ## Build for Linux (amd64 and arm64)
	@echo "$(COLOR_GREEN)Building for Linux...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)/*.go
	@GOOS=linux GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)/*.go
	@echo "$(COLOR_GREEN)✓ Linux builds complete$(COLOR_RESET)"

.PHONY: build-darwin
build-darwin: ## Build for macOS (amd64 and arm64)
	@echo "$(COLOR_GREEN)Building for macOS...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)
	@GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)/*.go
	@GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)/*.go
	@codesign -s - $(DIST_DIR)/$(BINARY_NAME)-darwin-* 2>/dev/null || true
	@echo "$(COLOR_GREEN)✓ macOS builds complete$(COLOR_RESET)"

.PHONY: build-windows
build-windows: ## Build for Windows (amd64)
	@echo "$(COLOR_GREEN)Building for Windows...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)
	@GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)/*.go
	@echo "$(COLOR_GREEN)✓ Windows build complete$(COLOR_RESET)"

##@ Release

.PHONY: release-prep
release-prep: check build-all ## Prepare for release (checks + build all platforms)
	@echo "$(COLOR_GREEN)Release preparation complete$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Next steps:$(COLOR_RESET)"
	@echo "  1. Create and push a git tag: git tag -a v$(VERSION) -m 'Release v$(VERSION)'"
	@echo "  2. Push tag: git push origin v$(VERSION)"
	@echo "  3. Create GitHub release with binaries from $(DIST_DIR)/"

.PHONY: release-checksums
release-checksums: ## Generate SHA256 checksums for release binaries
	@echo "$(COLOR_GREEN)Generating checksums...$(COLOR_RESET)"
	@cd $(DIST_DIR) && shasum -a 256 * > checksums.txt
	@echo "$(COLOR_GREEN)✓ Checksums generated: $(DIST_DIR)/checksums.txt$(COLOR_RESET)"

.PHONY: release-archives
release-archives: build-all ## Create release archives (tar.gz and zip)
	@echo "$(COLOR_GREEN)Creating release archives...$(COLOR_RESET)"
	@mkdir -p $(DIST_DIR)/archives
	@for file in $(DIST_DIR)/$(BINARY_NAME)-*; do \
		if [ "$$(basename $$file)" != "checksums.txt" ] && [ -d "$$file" ] || [ "$${file##*.}" = "txt" ]; then \
			continue; \
		fi; \
		basename=$$(basename $$file); \
		name=$${basename%.*}; \
		if [ "$${file##*.}" = "exe" ]; then \
			echo "Creating $$name.zip..."; \
			cd $(DIST_DIR) && zip -q archives/$$name.zip $$basename && cd ..; \
		else \
			echo "Creating $$name.tar.gz..."; \
			cd $(DIST_DIR) && tar -czf archives/$$name.tar.gz $$basename && cd ..; \
		fi; \
	done
	@cd $(DIST_DIR)/archives && shasum -a 256 * > checksums.txt
	@echo "$(COLOR_GREEN)✓ Archives created in $(DIST_DIR)/archives/$(COLOR_RESET)"

##@ Docker (Future)

.PHONY: docker-build
docker-build: ## Build Docker image (to be implemented)
	@echo "$(COLOR_YELLOW)Docker support coming soon...$(COLOR_RESET)"

.PHONY: docker-push
docker-push: ## Push Docker image (to be implemented)
	@echo "$(COLOR_YELLOW)Docker support coming soon...$(COLOR_RESET)"

##@ Cleanup

.PHONY: clean
clean: ## Remove build artifacts
	@echo "$(COLOR_GREEN)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@rm -f coverage.txt coverage.html
	@echo "$(COLOR_GREEN)✓ Clean complete$(COLOR_RESET)"

.PHONY: clean-deps
clean-deps: ## Remove downloaded dependencies
	@echo "$(COLOR_GREEN)Cleaning dependencies cache...$(COLOR_RESET)"
	$(GOCMD) clean -modcache
	@echo "$(COLOR_GREEN)✓ Dependencies cache cleaned$(COLOR_RESET)"

.PHONY: clean-all
clean-all: clean clean-deps ## Remove all build artifacts and dependencies
	@echo "$(COLOR_GREEN)✓ Full cleanup complete$(COLOR_RESET)"

##@ Information

.PHONY: info
info: ## Display build information
	@echo "$(COLOR_BOLD)Build Information$(COLOR_RESET)"
	@echo "  Project:     $(PROJECT_NAME)"
	@echo "  Version:     $(VERSION)"
	@echo "  Commit:      $(COMMIT)"
	@echo "  Build Date:  $(BUILD_DATE)"
	@echo "  Go Version:  $$($(GOCMD) version)"
	@echo "  Module:      $(MODULE)"

.PHONY: version
version: ## Display version
	@echo $(VERSION)

##@ Git Helpers

.PHONY: tag
tag: ## Create and push a new version tag (usage: make tag VERSION=v1.0.0)
	@if [ -z "$(VERSION)" ]; then \
		echo "$(COLOR_YELLOW)Usage: make tag VERSION=v1.0.0$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo "$(COLOR_GREEN)Creating tag $(VERSION)...$(COLOR_RESET)"
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)
	@echo "$(COLOR_GREEN)✓ Tag $(VERSION) created and pushed$(COLOR_RESET)"

.PHONY: changelog
changelog: ## Generate changelog from git commits
	@echo "$(COLOR_GREEN)Generating changelog...$(COLOR_RESET)"
	@git log --pretty=format:"- %s (%h)" $$(git describe --tags --abbrev=0 2>/dev/null || echo "")..HEAD
	@echo ""
