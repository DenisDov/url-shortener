# Load environment variables from .env if it exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

MAIN_PACKAGE_PATH = ./cmd/api/
BINARY_NAME = url-shortener
BUILD_DIR = ./bin

.PHONY: help setup run dev build build-local lint vet test confirm clean ci

.DEFAULT_GOAL := help

## help: show this help message
help:
	@echo "Available commands:"
	@grep -E '^## ' Makefile | sed 's/## /  /' | column -t -s ':'

## setup: install dependencies
setup:
	go mod tidy

## run: run the application locally
run:
	@echo "Starting application..."
	@go run $(MAIN_PACKAGE_PATH)

## dev: run the application with live-reload
dev:
	@echo "Starting application using air..."
	@go tool air

## build-local: build the application for the host platform
build-local:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE_PATH)

## build: cross-compile the application for darwin/linux/windows
build:
	@mkdir -p $(BUILD_DIR)
	GOARCH=amd64 GOOS=darwin go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin $(MAIN_PACKAGE_PATH)
	GOARCH=amd64 GOOS=linux go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux $(MAIN_PACKAGE_PATH)
	GOARCH=amd64 GOOS=windows go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows.exe $(MAIN_PACKAGE_PATH)

## vet: run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

## lint: run golangci-lint (requires it to be installed)
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

## test: run all tests
test:
	@echo "Running tests..."
	@go test -v ./...

confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

## clean: remove compiled binaries
clean: confirm
	@echo "Cleaning..."
	@rm -rf tmp/
	@rm -rf $(BUILD_DIR)
	@go clean

## ci: setup, vet, test, and build pipeline
ci: setup vet test build-local