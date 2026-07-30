.DEFAULT_GOAL := help

.PHONY: build check clean dev dev-api dev-smoke dev-web format format-check help install lint test typecheck

help: ## Show the available development commands
	@awk 'BEGIN {FS = ":.*## "; printf "IssueScout development commands:\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install the locked JavaScript dependencies
	pnpm install --frozen-lockfile

dev: ## Start the API and web stack with readiness and safe cleanup
	pnpm run dev

dev-api: ## Start the Go API
	@set -a; \
	if [ -f apps/api/.env ]; then . apps/api/.env; fi; \
	set +a; \
	pnpm run dev:api

dev-web: ## Start the Vite development server
	pnpm run dev:web

dev-smoke: ## Start, verify, and stop the deterministic mock stack
	pnpm run dev:smoke

format: ## Format all supported source files
	pnpm run format

format-check: ## Verify formatting without modifying files
	pnpm run format:check

lint: ## Run frontend and backend static analysis
	pnpm run lint

typecheck: ## Run strict TypeScript checks
	pnpm run typecheck

test: ## Run frontend and backend tests
	pnpm run test

build: ## Build production frontend and backend artifacts
	pnpm run build

check: ## Run the complete local quality gate
	pnpm run check

clean: ## Remove generated build and test output
	rm -rf bin apps/web/dist apps/web/coverage coverage
