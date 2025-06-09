# -------------------------------------------------------------------
# General Purpose Go Project Makefile
# -------------------------------------------------------------------
.PHONY: all build run test lint lint-fix fmt clean deps check-coverage help

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

check-coverage:
	@PKG_NAME=$(pkg) && \
	PKG_PATH=$$(find . -type d -name $$PKG_NAME | head -n 1) && \
	if [ -z "$$PKG_PATH" ]; then echo "Package '$$PKG_NAME' not found."; exit 1; fi && \
	COVERAGE_OUT=$${PKG_NAME}_coverage.out && \
	echo "Running tests for $$PKG_PATH..." && \
	go test $$PKG_PATH -coverprofile=$$COVERAGE_OUT -coverpkg=$$PKG_PATH && \
	echo "Coverage summary for $$PKG_NAME:" && \
	go tool cover -func=$$COVERAGE_OUT | grep "$$PKG_NAME" || echo "No matching lines found for $$PKG_NAME"

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
