# Load environment variables from .env if it exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

MAIN_PACKAGE_PATH = ./cmd/api/
BINARY_NAME = $(SERVICE_NAME)
BUILD_DIR = ./bin
MIGRATIONS_DIR := internal/db/migrations
BACKUP_DIR := backups

.PHONY: help setup run dev build build-local lint vet test confirm clean ci up down migrate-up migrate-down new-migration sqlc db-backup db-restore

.DEFAULT_GOAL := help

## help: show this help message
help:
	@echo "Available commands:"
	@grep -E '^## ' Makefile | sed 's/## /  /' | column -t -s ':'

## setup: install dependencies
setup:
	go mod tidy

## up: start db, redis and api in docker compose (does not run migrations)
up:            ; docker compose up -d --build
## down: stop the docker compose stack
down:          ; docker compose down
## migrate-up: apply one goose migration
migrate-up:    ; go tool goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" up
## migrate-down: roll back one goose migration
migrate-down:  ; go tool goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" down
## new-migration: create a migration, e.g. make new-migration name=add_foo
new-migration:
	@test -n "$(name)" || { echo "usage: make new-migration name=add_foo"; exit 1; }
	go tool goose -dir $(MIGRATIONS_DIR) create $(name) sql
## sqlc: regenerate internal/store from db/queries
sqlc:          ; go tool sqlc generate

## run: run the application locally
run:
	@echo "Starting application..."
	@DATABASE_URL="$(DB_DSN)" go run $(MAIN_PACKAGE_PATH)

## dev: run the application with live-reload
dev:
	@echo "Starting application using air..."
	@DATABASE_URL="$(DB_DSN)" go tool air

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

## db-backup: dump the compose db to backups/
db-backup:
	@mkdir -p $(BACKUP_DIR)
	@echo "Backing up database..."
	docker compose exec -T db pg_dump -U $(POSTGRES_USER) -d $(POSTGRES_DB) -F c > $(BACKUP_DIR)/db_backup_$$(date +%Y%m%d_%H%M%S).dump
	@echo "Backup complete!"

## db-restore: restore the compose db, e.g. make db-restore file=backups/db_backup_XXX.dump
db-restore:
	@test -n "$(file)" || { echo "usage: make db-restore file=backups/db_backup_XXX.dump"; exit 1; }
	@echo "Restoring database from $(file)..."
	docker compose exec -T db pg_restore -U $(POSTGRES_USER) -d $(POSTGRES_DB) -1 --clean --if-exists < $(file)
	@echo "Restore complete!"