# -------------------------------------------------------------------
# General Purpose Go Project Makefile
# -------------------------------------------------------------------
.PHONY: all build run test lint lint-fix fmt clean deps help

# Default variables (can be overridden via `make VAR=value`)
APP_NAME ?= k4-chat-backend
BUILD_DIR ?= bin
SRC_DIR ?= .
MAIN_FILE ?= ./cmd/main.go

# -------------------------------------------------------------------
# Commands
# -------------------------------------------------------------------

all: build

build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_FILE)

run: build
	@echo "Running $(APP_NAME)..."
	@./$(BUILD_DIR)/$(APP_NAME)

test:
	@echo "Running tests..."
	@go test ./...

lint:
	@echo "Running lint..."
	@golangci-lint run || echo "Lint issues found."

lint-fix:
	@echo "Running lint fix..."
	@golangci-lint run --fix || echo "Lint issues found."

fmt:
	@echo "Formatting code..."
	@go fmt ./...

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)

deps:
	@echo "Tidying up dependencies..."
	@go mod tidy

help:
	@echo "Common targets:"
	@echo "  build       - Build the Go binary"
	@echo "  run         - Run the application"
	@echo "  test        - Run unit tests"
	@echo "  lint        - Run golangci-lint"
	@echo "  lint-fix    - Run golangci-lint with --fix"
	@echo "  fmt         - Format Go files"
	@echo "  clean       - Remove built files"
	@echo "  deps        - Run go mod tidy"
