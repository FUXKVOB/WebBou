.PHONY: all server client clean run-server run-client deps-update versions help

# Default target
all: server client

# Build server
server:
	@echo "Building Go server..."
	cd server && go build -v -o ../bin/server main_webbou.go
	@echo "Server built: bin/server"

# Build client
client:
	@echo "Building Rust client..."
	cd client && cargo build --release
	@mkdir -p bin
	@cp client/target/release/webbou-client bin/client
	@echo "Client built: bin/client"

# Build release versions
release: release-server release-client

release-server:
	@echo "Building server (release)..."
	cd server && go build -v -ldflags="-s -w" -o ../bin/server main_webbou.go

release-client:
	@echo "Building client (release)..."
	cd client && cargo build --release
	@mkdir -p bin
	@cp client/target/release/webbou-client bin/client

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	cd server && go clean
	cd client && cargo clean
	@echo "Clean complete"

# Run server
run-server: server
	@echo "Starting server..."
	./bin/server

# Run client
run-client: client
	@echo "Starting client..."
	./bin/client

# Update dependencies
deps-update:
	@echo "Updating Go dependencies..."
	cd server && go get -u ./... && go mod tidy
	@echo "Updating Rust dependencies..."
	cd client && cargo update

# Show versions
versions:
	@echo "=== Installed Versions ==="
	@echo "Go version:"
	@go version
	@echo ""
	@echo "Rust version:"
	@rustc --version
	@echo ""
	@echo "Cargo version:"
	@cargo --version

# Check code
check:
	@echo "Checking Go code..."
	cd server && go vet ./...
	@echo "Checking Rust code..."
	cd client && cargo check

# Format code
fmt:
	@echo "Formatting Go code..."
	cd server && go fmt ./...
	@echo "Formatting Rust code..."
	cd client && cargo fmt

# Lint code
lint:
	@echo "Linting Go code..."
	cd server && golangci-lint run
	@echo "Linting Rust code..."
	cd client && cargo clippy -- -D warnings

# Install dependencies
install-deps:
	@echo "Installing Go dependencies..."
	cd server && go mod download
	@echo "Installing Rust dependencies..."
	cd client && cargo fetch

# Help
help:
	@echo "WebBou Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all           - Build server and client (default)"
	@echo "  server        - Build Go server"
	@echo "  client        - Build Rust client"
	@echo "  release       - Build optimized release versions"
	@echo "  clean         - Remove build artifacts"
	@echo "  run-server    - Build and run server"
	@echo "  run-client    - Build and run client"
	@echo "  deps-update   - Update all dependencies"
	@echo "  versions      - Show installed tool versions"
	@echo "  check         - Check code for errors"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  install-deps  - Download dependencies"
	@echo "  help          - Show this help message"
