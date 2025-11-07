.PHONY: build clean test deploy install-deps

# Build the trading system binary
build:
	@echo "Building trading system..."
	go build -o trading-system cmd/trading-system/main.go
	@echo "Build complete: ./trading-system"

# Build for Linux (for GCP deployment)
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -o trading-system-linux cmd/trading-system/main.go
	@echo "Build complete: ./trading-system-linux"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f trading-system trading-system-linux
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Install dependencies
install-deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run

# Run read module locally (for testing)
run-read:
	@echo "Running read module..."
	./trading-system -module=read

# Run trigger module locally (for testing)
run-trigger:
	@echo "Running trigger module..."
	./trading-system -module=trigger

# Deploy to GCP (requires GCP_PROJECT_ID, GCP_INSTANCE_NAME, GCP_ZONE env vars)
deploy:
	@echo "Deploying to GCP..."
	@chmod +x scripts/deploy.sh
	@scripts/deploy.sh

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the trading system binary"
	@echo "  build-linux  - Build for Linux (GCP deployment)"
	@echo "  clean        - Remove build artifacts"
	@echo "  test         - Run tests"
	@echo "  install-deps - Install Go dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  run-read     - Run read module locally"
	@echo "  run-trigger  - Run trigger module locally"
	@echo "  deploy       - Deploy to GCP instance"
	@echo "  help         - Show this help message"


