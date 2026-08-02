# Load environment variables from .env if it exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

main_package_path = ./cmd/api/
binary_name = url-shortener
build_dir = ./bin

.PHONY: help run dev build test confirm clean

## help: show this help message
help:
	@echo "Available commands:"
	@grep -E '^## ' Makefile | sed 's/## /  /' | column -t -s ':'

## run: run the application locally
run:
	@echo "Starting application..."
	@go run ${main_package_path}

## dev: run the application with live-reload
dev:
	@echo "Starting application using air..."
	@go tool air

## build: build the application
build:
	@mkdir -p ${build_dir}
	GOARCH=amd64 GOOS=darwin go build -o ${build_dir}/${binary_name}-darwin ${main_package_path}
	GOARCH=amd64 GOOS=linux go build -o ${build_dir}/${binary_name}-linux ${main_package_path}
	GOARCH=amd64 GOOS=windows go build -o ${build_dir}/${binary_name}-windows.exe ${main_package_path}

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
	@rm -rf ${build_dir}
	@go clean