.PHONY: all server client clean run-server run-client deps-update versions help test dev-cert

ifeq ($(OS),Windows_NT)
SERVER_BIN=bin\server.exe
CLIENT_BIN=bin\client.exe
SERVER_OUT=../bin/server.exe
CLIENT_OUT=../bin/client.exe
MKDIR_BIN=if not exist bin mkdir bin
COPY_CLIENT=copy /Y client\target\release\webbou-client.exe bin\client.exe >NUL
RUN_SERVER=.\bin\server.exe
RUN_CLIENT=.\bin\client.exe
else
SERVER_BIN=bin/server
CLIENT_BIN=bin/client
SERVER_OUT=../bin/server
CLIENT_OUT=../bin/client
MKDIR_BIN=mkdir -p bin
COPY_CLIENT=cp client/target/release/webbou-client bin/client
RUN_SERVER=./bin/server
RUN_CLIENT=./bin/client
endif

# Default target
all: server client

# Build server
server:
	@echo "Building Go server..."
	cd server && go build -v -o $(SERVER_OUT) main_webbou.go
	@echo "Server built: $(SERVER_BIN)"

# Build client
client:
	@echo "Building Rust client..."
	cd client && cargo build --release
	@$(MKDIR_BIN)
	@$(COPY_CLIENT)
	@echo "Client built: $(CLIENT_BIN)"

# Build release versions
release: release-server release-client

release-server:
	@echo "Building server (release)..."
	cd server && go build -v -ldflags="-s -w" -o $(SERVER_OUT) main_webbou.go

release-client:
	@echo "Building client (release)..."
	cd client && cargo build --release
	@$(MKDIR_BIN)
	@$(COPY_CLIENT)

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
	@$(RUN_SERVER)

# Run client
run-client: client
	@echo "Starting client..."
	@$(RUN_CLIENT)

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

# Run repository tests
test:
	@echo "Running Go tests..."
	cd server && go test ./...
	@echo "Running Rust tests..."
	cd client && cargo test

# Generate local development certificate on Windows
dev-cert:
	powershell -ExecutionPolicy Bypass -File scripts\\generate-dev-cert.ps1

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
	@echo "  dev-cert      - Generate cert.pem and key.pem for local Windows development"
	@echo "  run-server    - Build and run server"
	@echo "  run-client    - Build and run client"
	@echo "  deps-update   - Update all dependencies"
	@echo "  versions      - Show installed tool versions"
	@echo "  check         - Check code for errors"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  install-deps  - Download dependencies"
	@echo "  help          - Show this help message"
