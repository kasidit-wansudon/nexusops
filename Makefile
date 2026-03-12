.PHONY: all build test clean run-api run-runner run-proxy lint fmt docker-build docker-up docker-down help

# Variables
BINARY_DIR := bin
GO := go
GOFLAGS := -trimpath -ldflags="-s -w"
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"

# Docker
DOCKER_COMPOSE := docker compose
DOCKER_REGISTRY := ghcr.io/kasidit-wansudon/nexusops

all: build ## Build all binaries

build: build-api build-runner build-proxy build-cli ## Build all binaries

build-api: ## Build the API server
	@echo "Building API server..."
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_DIR)/nexusops-api ./cmd/api

build-runner: ## Build the pipeline runner
	@echo "Building pipeline runner..."
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_DIR)/nexusops-runner ./cmd/runner

build-proxy: ## Build the reverse proxy
	@echo "Building reverse proxy..."
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_DIR)/nexusops-proxy ./cmd/proxy

build-cli: ## Build the CLI tool
	@echo "Building nexusctl CLI..."
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_DIR)/nexusctl ./cmd/cli

test: ## Run all tests
	@echo "Running tests..."
	$(GO) test -v -race -coverprofile=coverage.out ./...

test-short: ## Run short tests only
	$(GO) test -short -race ./...

test-coverage: test ## Generate coverage report
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run ./...

fmt: ## Format code
	$(GO) fmt ./...
	goimports -w .

vet: ## Run go vet
	$(GO) vet ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BINARY_DIR) coverage.out coverage.html
	@rm -rf /tmp/nexusops

run-api: build-api ## Run the API server
	@echo "Starting API server..."
	./$(BINARY_DIR)/nexusops-api

run-runner: build-runner ## Run the pipeline runner
	@echo "Starting pipeline runner..."
	./$(BINARY_DIR)/nexusops-runner

run-proxy: build-proxy ## Run the reverse proxy
	@echo "Starting reverse proxy..."
	./$(BINARY_DIR)/nexusops-proxy

# Docker targets
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker build -f deploy/docker/Dockerfile.api -t $(DOCKER_REGISTRY)/api:$(VERSION) .
	docker build -f deploy/docker/Dockerfile.runner -t $(DOCKER_REGISTRY)/runner:$(VERSION) .
	docker build -f deploy/docker/Dockerfile.proxy -t $(DOCKER_REGISTRY)/proxy:$(VERSION) .

docker-up: ## Start all services with Docker Compose
	$(DOCKER_COMPOSE) up -d

docker-down: ## Stop all Docker Compose services
	$(DOCKER_COMPOSE) down

docker-logs: ## View Docker Compose logs
	$(DOCKER_COMPOSE) logs -f

# Frontend targets
frontend-install: ## Install frontend dependencies
	cd frontend && npm install

frontend-dev: ## Run frontend in development mode
	cd frontend && npm run dev

frontend-build: ## Build frontend for production
	cd frontend && npm run build

frontend-lint: ## Lint frontend code
	cd frontend && npm run lint

# Development
dev: ## Run all services for development
	@echo "Starting development environment..."
	$(DOCKER_COMPOSE) -f docker-compose.yml up -d postgres redis
	@sleep 2
	@echo "Starting API server..."
	$(GO) run ./cmd/api &
	@echo "Starting pipeline runner..."
	$(GO) run ./cmd/runner &
	@echo "Starting frontend..."
	cd frontend && npm run dev &
	@echo "Development environment ready!"
	@echo "  API:      http://localhost:8080"
	@echo "  Frontend: http://localhost:3000"
	@echo "  Runner:   http://localhost:8081"

# Database
migrate-up: ## Run database migrations
	$(GO) run ./cmd/migrate up

migrate-down: ## Rollback database migrations
	$(GO) run ./cmd/migrate down

# Release
release: test lint build ## Create a release build
	@echo "Release build complete: $(VERSION)"

# Generate
generate: ## Run go generate
	$(GO) generate ./...

proto: ## Generate protobuf code
	protoc --go_out=. --go-grpc_out=. proto/*.proto

help: ## Show this help
	@echo "NexusOps — Self-hosted Developer Platform"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
