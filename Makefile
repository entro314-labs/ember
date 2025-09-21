# Ember Makefile
# Build system for cross-platform Windows USB creator

BINARY_NAME=ember
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0-dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -s -w"
BUILD_DIR=dist
COVERAGE_DIR=coverage

# Go build configuration
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
CGO_ENABLED=0

# Colors for output
GREEN=\033[0;32m
YELLOW=\033[0;33m
RED=\033[0;31m
NC=\033[0m # No Color

# Default target
.PHONY: all
all: clean lint test build

# Development workflow
.PHONY: dev
dev: deps fmt lint test build run

# Build for current platform
.PHONY: build
build:
	@echo "$(GREEN)Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "$(GREEN)✓ Build completed: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

# Build with race detection for development
.PHONY: build-race
build-race:
	@echo "$(GREEN)Building $(BINARY_NAME) with race detection...$(NC)"
	@mkdir -p $(BUILD_DIR)
	go build -race $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-race .

# Build for all platforms
.PHONY: build-all
build-all: build-linux build-darwin build-windows
	@echo "$(GREEN)✓ All platform builds completed$(NC)"

.PHONY: build-linux
build-linux:
	@echo "$(GREEN)Building for Linux...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .

.PHONY: build-darwin
build-darwin:
	@echo "$(GREEN)Building for macOS...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .

.PHONY: build-windows
build-windows:
	@echo "$(GREEN)Building for Windows...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .

# Development
.PHONY: run
run: build
	@echo "$(GREEN)Running $(BINARY_NAME)...$(NC)"
	@./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

# Testing
.PHONY: test
test:
	@echo "$(GREEN)Running tests...$(NC)"
	go test -v -timeout=30s ./...

.PHONY: test-race
test-race:
	@echo "$(GREEN)Running tests with race detection...$(NC)"
	go test -v -race -timeout=60s ./...

.PHONY: test-coverage
test-coverage:
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	@mkdir -p $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "$(GREEN)✓ Coverage report generated: $(COVERAGE_DIR)/coverage.html$(NC)"

.PHONY: test-coverage-open
test-coverage-open: test-coverage
	@if command -v open >/dev/null 2>&1; then \
		open $(COVERAGE_DIR)/coverage.html; \
	elif command -v xdg-open >/dev/null 2>&1; then \
		xdg-open $(COVERAGE_DIR)/coverage.html; \
	else \
		echo "$(YELLOW)Coverage report available at: $(COVERAGE_DIR)/coverage.html$(NC)"; \
	fi

.PHONY: benchmark
benchmark:
	@echo "$(GREEN)Running benchmarks...$(NC)"
	go test -bench=. -benchmem -timeout=10m ./...

# Code quality
.PHONY: fmt
fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	gofmt -s -w .
	goimports -w .

.PHONY: fmt-check
fmt-check:
	@echo "$(GREEN)Checking code formatting...$(NC)"
	@if [ "$(shell gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "$(RED)✗ Code is not formatted properly:$(NC)"; \
		gofmt -s -l .; \
		exit 1; \
	else \
		echo "$(GREEN)✓ Code is properly formatted$(NC)"; \
	fi

.PHONY: vet
vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	go vet ./...

.PHONY: lint
lint:
	@echo "$(GREEN)Running linters...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "$(GREEN)✓ Linting completed$(NC)"; \
	else \
		echo "$(YELLOW)⚠ golangci-lint not installed, skipping linting$(NC)"; \
		echo "$(YELLOW)  Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)"; \
	fi

.PHONY: lint-fix
lint-fix:
	@echo "$(GREEN)Running linters with auto-fix...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --fix; \
		echo "$(GREEN)✓ Auto-fix completed$(NC)"; \
	else \
		echo "$(RED)✗ golangci-lint not installed$(NC)"; \
		exit 1; \
	fi

# Security
.PHONY: security-scan
security-scan:
	@echo "$(GREEN)Running security scan...$(NC)"
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
		echo "$(GREEN)✓ Security scan completed$(NC)"; \
	else \
		echo "$(YELLOW)⚠ gosec not installed, skipping security scan$(NC)"; \
		echo "$(YELLOW)  Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest$(NC)"; \
	fi

.PHONY: vuln-check
vuln-check:
	@echo "$(GREEN)Checking for known vulnerabilities...$(NC)"
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
		echo "$(GREEN)✓ Vulnerability check completed$(NC)"; \
	else \
		echo "$(YELLOW)⚠ govulncheck not installed, skipping vulnerability check$(NC)"; \
		echo "$(YELLOW)  Install with: go install golang.org/x/vuln/cmd/govulncheck@latest$(NC)"; \
	fi

# Dependencies
.PHONY: deps
deps:
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	go mod download
	go mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(NC)"

.PHONY: deps-update
deps-update:
	@echo "$(GREEN)Updating dependencies...$(NC)"
	go get -u ./...
	go mod tidy
	@echo "$(GREEN)✓ Dependencies updated to latest versions$(NC)"

.PHONY: deps-graph
deps-graph:
	@echo "$(GREEN)Generating dependency graph...$(NC)"
	@if command -v dot >/dev/null 2>&1; then \
		go mod graph | modgraphviz | dot -Tpng -o deps-graph.png; \
		echo "$(GREEN)✓ Dependency graph saved as deps-graph.png$(NC)"; \
	else \
		echo "$(YELLOW)⚠ graphviz not installed, cannot generate dependency graph$(NC)"; \
	fi

# Build ms-sys if available
.PHONY: build-ms-sys
build-ms-sys:
	@echo "$(GREEN)Building ms-sys...$(NC)"
	@if [ -d "ms-sys" ] && [ -f "ms-sys/Makefile" ]; then \
		echo "Building ms-sys from submodule..."; \
		cd ms-sys && make && mkdir -p ../binaries && cp -a ./build/bin/ms-sys ../binaries/ms-sys; \
		echo "$(GREEN)✓ ms-sys built successfully$(NC)"; \
	else \
		echo "$(YELLOW)⚠ ms-sys submodule not available or not initialized$(NC)"; \
		echo "$(YELLOW)  Initialize with: git submodule update --init --recursive$(NC)"; \
	fi

# Clean
.PHONY: clean
clean:
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	rm -rf $(BUILD_DIR)/
	rm -rf $(COVERAGE_DIR)/
	rm -f $(BINARY_NAME)
	rm -f deps-graph.png
	go clean -cache
	go clean -testcache
	@echo "$(GREEN)✓ Clean completed$(NC)"

.PHONY: clean-all
clean-all: clean
	@echo "$(GREEN)Cleaning all generated files...$(NC)"
	go clean -modcache
	rm -rf vendor/

# Setup development environment
.PHONY: setup
setup: 
	@echo "$(GREEN)Setting up development environment...$(NC)"
	@echo "$(GREEN)Installing development tools...$(NC)"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest || echo "$(YELLOW)⚠ Failed to install golangci-lint$(NC)"
	@go install golang.org/x/tools/cmd/goimports@latest || echo "$(YELLOW)⚠ Failed to install goimports$(NC)"
	@go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest || echo "$(YELLOW)⚠ Failed to install gosec$(NC)"
	@go install golang.org/x/vuln/cmd/govulncheck@latest || echo "$(YELLOW)⚠ Failed to install govulncheck$(NC)"
	@$(MAKE) deps
	@$(MAKE) build-ms-sys
	@echo "$(GREEN)✓ Development environment setup completed$(NC)"

# Docker support
.PHONY: docker-build
docker-build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t $(BINARY_NAME):$(VERSION) .

.PHONY: docker-run
docker-run: docker-build
	@echo "$(GREEN)Running Docker container...$(NC)"
	docker run --rm -it $(BINARY_NAME):$(VERSION)

# Release
.PHONY: release-check
release-check: clean fmt-check vet lint test security-scan vuln-check
	@echo "$(GREEN)✓ Release checks passed$(NC)"

.PHONY: release-build
release-build: release-check build-all
	@echo "$(GREEN)Creating release checksums...$(NC)"
	@cd $(BUILD_DIR) && find . -type f -exec sha256sum {} \; > checksums.txt
	@echo "$(GREEN)✓ Release build completed$(NC)"

# Install/Uninstall
.PHONY: install
install: build
	@echo "$(GREEN)Installing $(BINARY_NAME)...$(NC)"
	@if [ "$(shell uname)" = "Darwin" ]; then \
		sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/; \
	else \
		sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/; \
	fi
	@echo "$(GREEN)✓ $(BINARY_NAME) installed to /usr/local/bin/$(NC)"

.PHONY: uninstall
uninstall:
	@echo "$(GREEN)Uninstalling $(BINARY_NAME)...$(NC)"
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "$(GREEN)✓ $(BINARY_NAME) uninstalled$(NC)"

# Monitoring and profiling
.PHONY: cpu-profile
cpu-profile: build
	@echo "$(GREEN)Running CPU profiling...$(NC)"
	./$(BUILD_DIR)/$(BINARY_NAME) --help  # Run with profiling enabled
	go tool pprof cpu.prof

.PHONY: mem-profile
mem-profile: build
	@echo "$(GREEN)Running memory profiling...$(NC)"
	./$(BUILD_DIR)/$(BINARY_NAME) --help  # Run with profiling enabled
	go tool pprof mem.prof

# Documentation
.PHONY: docs
docs:
	@echo "$(GREEN)Generating documentation...$(NC)"
	@if command -v godoc >/dev/null 2>&1; then \
		echo "$(GREEN)Starting godoc server at http://localhost:6060$(NC)"; \
		godoc -http=:6060; \
	else \
		echo "$(YELLOW)⚠ godoc not installed$(NC)"; \
		echo "$(YELLOW)  Install with: go install golang.org/x/tools/cmd/godoc@latest$(NC)"; \
	fi

# Status and information
.PHONY: status
status:
	@echo "$(GREEN)Project Status:$(NC)"
	@echo "  Binary: $(BINARY_NAME)"
	@echo "  Version: $(VERSION)"
	@echo "  Platform: $(GOOS)/$(GOARCH)"
	@echo "  Go Version: $(shell go version)"
	@echo ""
	@echo "$(GREEN)Dependencies:$(NC)"
	@go list -m all | head -10
	@echo ""
	@echo "$(GREEN)Build Status:$(NC)"
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		echo "  ✓ Binary exists: $(BUILD_DIR)/$(BINARY_NAME)"; \
		ls -la $(BUILD_DIR)/$(BINARY_NAME); \
	else \
		echo "  ✗ Binary not found: $(BUILD_DIR)/$(BINARY_NAME)"; \
	fi

# Help
.PHONY: help
help:
	@echo "$(GREEN)Ember Build System$(NC)"
	@echo ""
	@echo "$(GREEN)Build Commands:$(NC)"
	@echo "  build           - Build for current platform"
	@echo "  build-all       - Build for all platforms"
	@echo "  build-race      - Build with race detection"
	@echo "  build-ms-sys    - Build ms-sys binary"
	@echo ""
	@echo "$(GREEN)Development Commands:$(NC)"
	@echo "  dev             - Full development workflow"
	@echo "  run             - Build and run the application"
	@echo "  setup           - Setup development environment"
	@echo ""
	@echo "$(GREEN)Testing Commands:$(NC)"
	@echo "  test            - Run tests"
	@echo "  test-race       - Run tests with race detection"
	@echo "  test-coverage   - Run tests with coverage report"
	@echo "  benchmark       - Run benchmarks"
	@echo ""
	@echo "$(GREEN)Code Quality Commands:$(NC)"
	@echo "  fmt             - Format code"
	@echo "  fmt-check       - Check code formatting"
	@echo "  vet             - Run go vet"
	@echo "  lint            - Run linters"
	@echo "  lint-fix        - Run linters with auto-fix"
	@echo ""
	@echo "$(GREEN)Security Commands:$(NC)"
	@echo "  security-scan   - Run security scanner"
	@echo "  vuln-check      - Check for vulnerabilities"
	@echo ""
	@echo "$(GREEN)Dependency Commands:$(NC)"
	@echo "  deps            - Download and tidy dependencies"
	@echo "  deps-update     - Update all dependencies"
	@echo "  deps-graph      - Generate dependency graph"
	@echo ""
	@echo "$(GREEN)Release Commands:$(NC)"
	@echo "  release-check   - Run all release validation checks"
	@echo "  release-build   - Build release artifacts"
	@echo ""
	@echo "$(GREEN)Utility Commands:$(NC)"
	@echo "  clean           - Clean build artifacts"
	@echo "  clean-all       - Clean all generated files"
	@echo "  install         - Install binary to /usr/local/bin"
	@echo "  uninstall       - Remove binary from /usr/local/bin"
	@echo "  status          - Show project status"
	@echo "  help            - Show this help"