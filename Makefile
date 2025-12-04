# GitHub PR Age Watcher Makefile

.PHONY: build clean test run help install deps install-systemd uninstall-systemd

BINARY_NAME=pr-watcher
VERSION?=1.0.0
BUILD_TIME=$(shell date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT=$(shell git rev-parse --short HEAD)
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

all: deps build

# install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

build:
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) .

build-all:
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe .

test:
	@echo "Running tests..."
	go test -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

watch: build
	@echo "Running $(BINARY_NAME) in watch mode..."
	./$(BINARY_NAME) -watch

run-config: build
	@echo "Running $(BINARY_NAME) with custom config..."
	./$(BINARY_NAME) -config=config.yaml

clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	rm -f coverage.out coverage.html

install: build
	@echo "Installing $(BINARY_NAME) to GOPATH/bin..."
	go install .

install-systemd: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin (requires sudo)..."
	sudo install -m 0755 ./$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Creating config directory /etc/pr-watcher (requires sudo)..."
	sudo install -d -o root -g root -m 0755 /etc/pr-watcher
	@if [ ! -f /etc/pr-watcher/config.yaml ]; then \
		echo "Copying example config to /etc/pr-watcher/config.yaml"; \
		sudo install -m 0644 ./config.yaml.example /etc/pr-watcher/config.yaml; \
	else \
		echo "/etc/pr-watcher/config.yaml already exists; leaving as-is"; \
	fi
	@echo "Installing systemd unit (requires sudo)..."
	sudo install -m 0644 ./contrib/systemd/$(BINARY_NAME).service /etc/systemd/system/$(BINARY_NAME).service
	@echo "Creating service user '$(BINARY_NAME)' if missing (requires sudo)..."
	@if ! id -u $(BINARY_NAME) >/dev/null 2>&1; then \
		sudo useradd --system --no-create-home --home-dir /var/lib/$(BINARY_NAME) --shell /usr/sbin/nologin $(BINARY_NAME); \
	fi
	@echo "Ensuring directories exist and owned by service user (requires sudo)..."
	sudo install -d -o $(BINARY_NAME) -g $(BINARY_NAME) -m 0755 /var/lib/$(BINARY_NAME)
	sudo chown -R $(BINARY_NAME):$(BINARY_NAME) /etc/pr-watcher || true
	@echo "Reloading systemd daemon and enabling service (requires sudo)..."
	sudo systemctl daemon-reload
	sudo systemctl enable $(BINARY_NAME).service
	@echo "You can now start the service with: sudo systemctl start $(BINARY_NAME)"

uninstall-systemd:
	@echo "Stopping and disabling service (requires sudo)..."
	- sudo systemctl stop $(BINARY_NAME).service || true
	- sudo systemctl disable $(BINARY_NAME).service || true
	@echo "Removing unit file and binary (requires sudo)..."
	- sudo rm -f /etc/systemd/system/$(BINARY_NAME).service
	- sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Reloading systemd daemon (requires sudo)..."
	- sudo systemctl daemon-reload || true

fmt:
	@echo "Formatting code..."
	go fmt ./...

lint:
	@echo "Linting code..."
	golangci-lint run

release: clean test build-all
	@echo "Creating release $(VERSION)..."
	@mkdir -p release
	@cp $(BINARY_NAME)-* release/
	@cp config.yaml.example release/
	@cp README.md release/
	@echo "Release files created in release/ directory"

setup: deps
	@echo "Setting up development environment..."
	@if [ ! -f config.yaml ]; then cp config.yaml.example config.yaml; fi
	@echo "Development environment ready!"

docker-build:
	@echo "Building Docker image..."
	docker build -t pr-watcher:$(VERSION) .

docker-run: docker-build
	@echo "Running Docker container..."
	docker run --rm -v $(PWD)/config.yaml:/app/config.yaml pr-watcher:$(VERSION)

help:
	@echo "Available targets:"
	@echo "  deps          - Install dependencies"
	@echo "  build         - Build the application"
	@echo "  build-all     - Build for multiple platforms"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  run           - Run the application"
	@echo "  watch         - Run in watch mode"
	@echo "  run-config    - Run with custom config"
	@echo "  clean         - Clean build artifacts"
	@echo "  install       - Install to GOPATH/bin"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  release       - Create release package"
	@echo "  setup         - Setup development environment"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  help          - Show this help"
