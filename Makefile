#!make

ROOT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
APP_NAME := meatshop
CMD_PATH := ./cmd/$(APP_NAME)
MIGRATIONS_DIR := $(ROOT_DIR)migrations
SQL_MIGRATIONS_DIR := $(MIGRATIONS_DIR)/sql
DEPLOY_DIR := $(ROOT_DIR)deploy
BUILD_OUTPUT := $(ROOT_DIR)$(APP_NAME)

API_BASE_URL ?= http://127.0.0.1:59000
API_USER ?= admin
API_PWD ?= 123456

-include $(ROOT_DIR).env

.PHONY: \
	build \
	build-max \
	run \
	run-max \
	test \
	test-integration \
	codegen \
	clean \
	prod \
	prod-max \
	migup \
	migdown \
	migfix \
	migadd \
	migver \
	runsql \
	deploy \
	deploy-full \
	deploy-backend \
	deploy-frontend \
	deploy-no-migrations

# Development build with debugging information.
build:
	@echo "Building $(APP_NAME)..."
	@go -C "$(ROOT_DIR)" build \
		-o "$(BUILD_OUTPUT)" \
		"$(CMD_PATH)"

# Build the MAX bot executable.
build-max:
	@echo "Building MAX bot..."
	@go -C "$(ROOT_DIR)" build \
		-o "$(ROOT_DIR)max" \
		./cmd/max

# Run the application directly from source.
run:
	@echo "Running $(APP_NAME)..."
	@go -C "$(ROOT_DIR)" run "$(CMD_PATH)"

# Run the MAX bot directly from source.
run-max:
	@echo "Running MAX bot..."
	@go -C "$(ROOT_DIR)" run ./cmd/max

# Run all unit tests.
test:
	@echo "Running unit tests..."
	@go -C "$(ROOT_DIR)" test ./...

# Run integration tests against an already-running API server.
test-integration:
	@echo "Running integration tests against $(API_BASE_URL)..."
	@API_BASE_URL="$(API_BASE_URL)" \
		API_USER="$(API_USER)" \
		API_PWD="$(API_PWD)" \
		go -C "$(ROOT_DIR)" test \
			./internal/apitest \
			-tags=integration \
			-count=1 \
			-v

# Remove generated build artifacts and clear the Go test cache.
clean:
	@echo "Cleaning build artifacts..."
	@rm -f "$(BUILD_OUTPUT)"
	@go -C "$(ROOT_DIR)" clean -testcache

# Optimized production build.
prod:
	@echo "Building production $(APP_NAME)..."
	@CGO_ENABLED=0 go -C "$(ROOT_DIR)" build \
		-trimpath \
		-ldflags="-s -w" \
		-o "$(BUILD_OUTPUT)" \
		"$(CMD_PATH)"

# Optimized production build of the MAX bot.
prod-max:
	@echo "Building production MAX bot..."
	@CGO_ENABLED=0 go -C "$(ROOT_DIR)" build \
		-trimpath \
		-ldflags="-s -w" \
		-o "$(ROOT_DIR)max" \
		./cmd/max

migup:
	@migrate \
		-database "$(DB_CONN)" \
		-path "$(MIGRATIONS_DIR)" \
		up 1

migdown:
	@migrate \
		-database "$(DB_CONN)" \
		-path "$(MIGRATIONS_DIR)" \
		down 1

migfix:
	@test -n "$(word 2, $(MAKECMDGOALS))" || \
		(echo "Usage: make migfix <version>" && exit 1)
	@migrate \
		-database "$(DB_CONN)" \
		-path "$(MIGRATIONS_DIR)" \
		force "$(word 2, $(MAKECMDGOALS))"

migadd:
	@test -n "$(word 2, $(MAKECMDGOALS))" || \
		(echo "Usage: make migadd <migration_name>" && exit 1)
	@migrate create \
		-ext sql \
		-dir "$(MIGRATIONS_DIR)" \
		-seq "$(word 2, $(MAKECMDGOALS))"

migver:
	@migrate \
		-database "$(DB_CONN)" \
		-path "$(MIGRATIONS_DIR)" \
		version

runsql:
	@test -n "$(word 2, $(MAKECMDGOALS))" || \
		(echo "Usage: make runsql <script_name>" && exit 1)
	@php "$(DEPLOY_DIR)/run_sql.php" \
		"$(DB_CONN)" \
		"$(word 2, $(MAKECMDGOALS))" \
		"$(SQL_MIGRATIONS_DIR)"

# Full deployment: backend and frontend.
deploy-full:
	@echo "🚀 Starting full deployment..."
	@"$(DEPLOY_DIR)/deploy.sh"

# Backend-only deployment.
deploy-backend:
	@echo "🔧 Deploying backend only..."
	@"$(DEPLOY_DIR)/deploy.sh" --backend-only

# Frontend-only deployment.
deploy-frontend:
	@echo "🎨 Deploying frontend only..."
	@"$(DEPLOY_DIR)/deploy.sh" --frontend-only

# Deploy without database migrations.
deploy-no-migrations:
	@echo "⚡ Deploying without migrations..."
	@"$(DEPLOY_DIR)/deploy.sh" --skip-migrations

deploy: deploy-full

# Run the Go code generator.
# Usage:
#   make codegen
#   make codegen gen
#   make codegen check
#   make codegen valid
codegen:
	@case "$(word 2, $(MAKECMDGOALS))" in \
		""|"gen") \
			echo "Running code generator..."; \
			go -C "$(ROOT_DIR)" tool codegen generate; \
			;; \
		"check") \
			echo "Checking generated code..."; \
			go -C "$(ROOT_DIR)" tool codegen check; \
			;; \
		"valid") \
			echo "Validating code generator definitions..."; \
			go -C "$(ROOT_DIR)" tool codegen validate; \
			;; \
		*) \
			echo "Usage: make codegen [gen|check|valid]"; \
			exit 1; \
			;; \
	esac

# Ignore positional arguments used by targets such as:
# make migadd create_products
# make migfix 15
%:
	@:

