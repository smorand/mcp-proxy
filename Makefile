.PHONY: help build test format lint check clean

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the mcp-proxy binary
	@echo "Building mcp-proxy..."
	@go build -o mcp-proxy .

test: ## Run all tests
	@echo "Running tests..."
	@cd tests && go test -v

format: ## Format Go code
	@echo "Formatting code..."
	@go fmt ./...

lint: ## Run linter (requires golangci-lint)
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

check: format lint test ## Run all quality checks (format, lint, test)
	@echo "All quality checks passed!"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f mcp-proxy
	@rm -f tests/mcp-proxy-test
	@go clean
