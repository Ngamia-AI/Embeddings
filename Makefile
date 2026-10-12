SHELL := /bin/bash
COMPOSE := docker compose -f deployments/docker-compose.yml
ENV_FILE := deployments/.env
MODELS_DIR ?= $(CURDIR)/models

.PHONY: help build run test vet fmt tidy lint models manifest verify-models \
        up down logs ps restart smoke clean docker-build deploy \
        engine-py-build engine-py-up engine-py-test

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Compile the gateway
	go build -o bin/embeddings-api ./cmd/api

run: ## Run the gateway locally (needs DATABASE_URL, ENGINE_URL)
	go run ./cmd/api

test: ## Run unit tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format the tree
	gofmt -w .

tidy: ## Tidy modules
	go mod tidy

lint: ## Run golangci-lint if installed
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || \
		echo "golangci-lint not installed, skipping"

models: ## Download + pin the GGUF models on the VPS
	MODELS_DIR=$(MODELS_DIR) scripts/download-models.sh

manifest: ## Regenerate models/MANIFEST.sha256 from present .gguf files
	MODELS_DIR=$(MODELS_DIR) scripts/gen-manifest.sh

verify-models: ## Verify present model files against the manifest
	MODELS_DIR=$(MODELS_DIR) scripts/verify-models.sh

up: ## Start the full stack (postgres + engine + gateway + caddy)
	$(COMPOSE) --env-file $(ENV_FILE) up -d --build

deploy: ## One-command deploy (models + build + up + TLS). See scripts/deploy.sh
	./scripts/deploy.sh

down: ## Stop the stack
	$(COMPOSE) --env-file $(ENV_FILE) down

logs: ## Tail logs
	$(COMPOSE) logs -f --tail=100

ps: ## Show container status
	$(COMPOSE) ps

restart: ## Restart the gateway only
	$(COMPOSE) --env-file $(ENV_FILE) restart gateway

smoke: ## Hit the local gateway
	curl -fsS http://127.0.0.1:$${GATEWAY_PORT:-8090}/readyz && echo

clean: ## Remove build artifacts
	rm -rf bin

engine-py-build: ## Build the Python video engine image
	docker build -t embeddings-engine-py:latest engine-py

engine-py-up: ## Start postgres + engine + gateway + Python video engine
	$(COMPOSE) --profile video --env-file $(ENV_FILE) up -d --build

engine-py-test: ## Run the Python engine tests (needs deps installed)
	cd engine-py && python -m pytest
