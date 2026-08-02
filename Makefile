# Load environment variables from .env if it exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif


BINARY_NAME=url-shortener
.DEFAULT_GOAL := help

.PHONY: help all run dev test clean

## help: show this help message
help:
	@echo "Available commands:"
	@grep -E '^## ' Makefile | sed 's/## /  /' | column -t -s ':'

## run: run the application locally
run:
	@echo "Starting application..."
	@go run main.go

## dev: run the application with live-reload
dev:
	@echo "Starting application using air..."
	@go tool air

## test: run all tests
test:
	@echo "Running tests..."
	@go test -v ./...

## clean: remove compiled binaries
clean:
	@echo "Cleaning..."
	@rm -rf tmp/
	@go clean