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
COVERAGE_FILE := coverage.txt

# Every target here is phony. .PHONY is declared next to each recipe rather than
# as one list at the top so a new target cannot be added without it.

.DEFAULT_GOAL := help

## help: show this help message
.PHONY: help
help:
	@echo "Available commands:"
	@grep -E '^## ' Makefile | sed 's/## /  /' | column -t -s ':'

## setup: install dependencies
.PHONY: setup
setup:
	go mod tidy

## up: start db, redis and api in docker compose (does not run migrations)
.PHONY: up
up:            ; docker compose up -d --build

## down: stop the docker compose stack
.PHONY: down
down:          ; docker compose down

## migrate-up: apply one goose migration
.PHONY: migrate-up
migrate-up:    ; go tool goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" up

## migrate-down: roll back one goose migration
.PHONY: migrate-down
migrate-down:  ; go tool goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" down

## new-migration: create a migration, e.g. make new-migration name=add_foo
.PHONY: new-migration
new-migration:
	@test -n "$(name)" || { echo "usage: make new-migration name=add_foo"; exit 1; }
	go tool goose -dir $(MIGRATIONS_DIR) create $(name) sql

## sqlc: regenerate internal/store from db/queries
.PHONY: sqlc
sqlc:          ; go tool sqlc generate

## run: run the application locally
.PHONY: run
run:
	@echo "Starting application..."
	@DATABASE_URL="$(DB_DSN)" go run $(MAIN_PACKAGE_PATH)

## dev: run the application with live-reload
.PHONY: dev
dev:
	@echo "Starting application using air..."
	@DATABASE_URL="$(DB_DSN)" go tool air

## build-local: build the application for the host platform
.PHONY: build-local
build-local:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE_PATH)

## build: cross-compile the application for darwin/linux/windows
.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	GOARCH=amd64 GOOS=darwin go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin $(MAIN_PACKAGE_PATH)
	GOARCH=amd64 GOOS=linux go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux $(MAIN_PACKAGE_PATH)
	GOARCH=amd64 GOOS=windows go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows.exe $(MAIN_PACKAGE_PATH)

## vet: run go vet
.PHONY: vet
vet:
	@echo "Running go vet..."
	@go vet ./...

## lint: run golangci-lint (requires it to be installed)
.PHONY: lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

## test: run all tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v ./...

## cover: run tests with coverage and open the html report
.PHONY: cover
cover:
	@echo "Running tests with coverage..."
	@go test -covermode=atomic -coverprofile=$(COVERAGE_FILE) ./...
	@go tool cover -html=$(COVERAGE_FILE)

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

## clean: remove compiled binaries
.PHONY: clean
clean: confirm
	@echo "Cleaning..."
	@rm -rf tmp/
	@rm -rf $(BUILD_DIR)
	@rm -f $(COVERAGE_FILE)
	@go clean

## ci: setup, vet, test, and build pipeline
.PHONY: ci
ci: setup vet test build-local

## db-backup: dump the compose db to backups/
.PHONY: db-backup
db-backup:
	@mkdir -p $(BACKUP_DIR)
	@echo "Backing up database..."
	docker compose exec -T db pg_dump -U $(POSTGRES_USER) -d $(POSTGRES_DB) -F c > $(BACKUP_DIR)/db_backup_$$(date +%Y%m%d_%H%M%S).dump
	@echo "Backup complete!"

## db-restore: restore the compose db, e.g. make db-restore file=backups/db_backup_XXX.dump
.PHONY: db-restore
db-restore:
	@test -n "$(file)" || { echo "usage: make db-restore file=backups/db_backup_XXX.dump"; exit 1; }
	@echo "Restoring database from $(file)..."
	docker compose exec -T db pg_restore -U $(POSTGRES_USER) -d $(POSTGRES_DB) -1 --clean --if-exists < $(file)
	@echo "Restore complete!"
