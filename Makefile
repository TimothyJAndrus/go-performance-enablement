.PHONY: help build test clean install-tools setup-local start stop logs fmt lint docker-build deploy-dev deploy-stage deploy-prod test-race benchmark terraform-init terraform-plan terraform-apply

# Variables
GO := go
DOCKER := docker
DOCKER_COMPOSE := docker-compose
AWS := aws
KUBECTL := kubectl

# Colors
CYAN := \033[0;36m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m

help: ## Display this help message
	@echo "$(CYAN)Available targets:$(NC)"
	@awk 'BEGIN {FS = ":.*##"; printf "\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(CYAN)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

install-tools: ## Install required development tools
	@echo "$(YELLOW)Installing development tools...$(NC)"
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/cosmtrek/air@latest
	$(GO) install github.com/aws/aws-sam-cli@latest
	@echo "$(GREEN)✓ Tools installed$(NC)"

setup-local: ## Setup local development environment
	@echo "$(YELLOW)Setting up local environment...$(NC)"
	chmod +x scripts/setup-local.sh
	./scripts/setup-local.sh

start: ## Start all services
	@echo "$(YELLOW)Starting services...$(NC)"
	$(DOCKER_COMPOSE) up -d
	@echo "$(GREEN)✓ Services started$(NC)"
	@echo "Kafka UI: http://localhost:8080"
	@echo "Grafana:  http://localhost:3000"

stop: ## Stop all services
	@echo "$(YELLOW)Stopping services...$(NC)"
	$(DOCKER_COMPOSE) down
	@echo "$(GREEN)✓ Services stopped$(NC)"

restart: stop start ## Restart all services

logs: ## View logs from all services
	$(DOCKER_COMPOSE) logs -f

logs-consumer: ## View Kafka consumer logs
	$(DOCKER_COMPOSE) logs -f kafka-consumer

logs-kafka: ## View Kafka logs
	$(DOCKER_COMPOSE) logs -f kafka

##@ Build

build: ## Build all applications
	@echo "$(YELLOW)Building applications...$(NC)"
	$(GO) build -o bin/kafka-consumer ./kafka-consumer/main.go
	@echo "$(GREEN)✓ Build complete$(NC)"

build-lambdas: ## Build all Lambda functions
	@echo "$(YELLOW)Building Lambda functions...$(NC)"
	mkdir -p bin/lambdas
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-w -s" -o bin/lambdas/event-router lambdas/event-router/main.go
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-w -s" -o bin/lambdas/stream-processor lambdas/stream-processor/main.go
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-w -s" -o bin/lambdas/event-transformer lambdas/event-transformer/main.go
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-w -s" -o bin/lambdas/health-checker lambdas/health-checker/main.go
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-w -s" -o bin/lambdas/authorizer lambdas/authorizer/main.go
	@echo "$(GREEN)✓ Lambda functions built$(NC)"

docker-build: ## Build Docker images
	@echo "$(YELLOW)Building Docker images...$(NC)"
	$(DOCKER) build -t go-kafka-consumer:latest -f kafka-consumer/Dockerfile .
	@echo "$(GREEN)✓ Docker images built$(NC)"

##@ Testing

test: ## Run all tests
	@echo "$(YELLOW)Running tests...$(NC)"
	$(GO) test -v ./...

test-coverage: ## Run tests with coverage
	@echo "$(YELLOW)Running tests with coverage...$(NC)"
	$(GO) test -cover -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report: coverage.html$(NC)"

test-integration: ## Run integration tests
	@echo "$(YELLOW)Running integration tests...$(NC)"
	$(GO) test -v -tags=integration ./tests/integration/...

benchmark: ## Run benchmarks
	@echo "$(YELLOW)Running benchmarks...$(NC)"
	$(GO) test -bench=. -benchmem ./benchmarks/...

benchmark-all: ## Run all benchmarks with CPU profiling
	@echo "$(YELLOW)Running full benchmarks...$(NC)"
	$(GO) test -bench=. -benchmem -count=5 ./benchmarks/... | tee benchmark-results.txt
	@echo "$(GREEN)✓ Results saved to benchmark-results.txt$(NC)"

test-race: ## Run tests with race detector
	@echo "$(YELLOW)Running tests with race detector...$(NC)"
	$(GO) test -race ./...

##@ Code Quality

fmt: ## Format code
	@echo "$(YELLOW)Formatting code...$(NC)"
	$(GO) fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

lint: ## Run linter
	@echo "$(YELLOW)Running linter...$(NC)"
	golangci-lint run --timeout 5m
	@echo "$(GREEN)✓ Linting complete$(NC)"

vet: ## Run go vet
	@echo "$(YELLOW)Running go vet...$(NC)"
	$(GO) vet ./...
	@echo "$(GREEN)✓ Vet complete$(NC)"

check: fmt vet lint test ## Run all checks

##@ Deployment

deploy-local-lambda: build-lambdas ## Deploy Lambda functions locally with SAM
	@echo "$(YELLOW)Deploying Lambda functions locally...$(NC)"
	sam local start-lambda

deploy-dev: ## Deploy to development environment
	@echo "$(YELLOW)Deploying to dev...$(NC)"
	$(KUBECTL) apply -k k8s/overlays/dev
	@echo "$(GREEN)✓ Deployed to dev$(NC)"

deploy-stage: ## Deploy to staging environment
	@echo "$(YELLOW)Deploying to staging...$(NC)"
	$(KUBECTL) apply -k k8s/overlays/stage
	@echo "$(GREEN)✓ Deployed to staging$(NC)"

deploy-prod: ## Deploy to production environment
	@echo "$(YELLOW)Deploying to production...$(NC)"
	@read -p "Are you sure you want to deploy to production? [y/N] " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		$(KUBECTL) apply -k k8s/overlays/prod; \
		echo "$(GREEN)✓ Deployed to prod$(NC)"; \
	else \
		echo "$(YELLOW)Deployment cancelled$(NC)"; \
	fi

##@ Utilities

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning...$(NC)"
	rm -rf bin/
	rm -rf coverage.out coverage.html
	rm -rf vendor/
	$(GO) clean -cache
	@echo "$(GREEN)✓ Cleaned$(NC)"

deps: ## Download dependencies
	@echo "$(YELLOW)Downloading dependencies...$(NC)"
	$(GO) mod download
	$(GO) mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(NC)"

generate: ## Run go generate
	@echo "$(YELLOW)Running go generate...$(NC)"
	$(GO) generate ./...
	@echo "$(GREEN)✓ Generation complete$(NC)"

kafka-topics: ## List Kafka topics
	$(DOCKER_COMPOSE) exec kafka kafka-topics --list --bootstrap-server localhost:9092

kafka-consumer-groups: ## List Kafka consumer groups
	$(DOCKER_COMPOSE) exec kafka kafka-consumer-groups --list --bootstrap-server localhost:9092

kafka-consume: ## Consume messages from Kafka topic (usage: make kafka-consume TOPIC=qlik.customers)
	$(DOCKER_COMPOSE) exec kafka kafka-console-consumer \
		--bootstrap-server localhost:9092 \
		--topic $(TOPIC) \
		--from-beginning

aws-local: ## Run AWS CLI against LocalStack
	@echo "Using LocalStack endpoint: http://localhost:4566"
	AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test $(AWS) --endpoint-url=http://localhost:4566 $(CMD)

##@ Terraform

terraform-init: ## Initialize Terraform
	@echo "$(YELLOW)Initializing Terraform...$(NC)"
	cd terraform/sandbox && terraform init
	@echo "$(GREEN)✓ Terraform initialized$(NC)"

terraform-plan: ## Plan Terraform changes (usage: make terraform-plan ENV=dev)
	@echo "$(YELLOW)Planning Terraform changes...$(NC)"
	cd terraform/sandbox && terraform plan -var-file=$(ENV).tfvars

terraform-apply: ## Apply Terraform changes (usage: make terraform-apply ENV=dev)
	@echo "$(YELLOW)Applying Terraform changes...$(NC)"
	cd terraform/sandbox && terraform apply -var-file=$(ENV).tfvars

terraform-destroy: ## Destroy Terraform resources (usage: make terraform-destroy ENV=dev)
	@echo "$(YELLOW)Destroying Terraform resources...$(NC)"
	cd terraform/sandbox && terraform destroy -var-file=$(ENV).tfvars

##@ Monitoring

metrics: ## View Prometheus metrics
	@echo "Opening Prometheus: http://localhost:9090"
	open http://localhost:9090 || xdg-open http://localhost:9090

grafana: ## Open Grafana dashboard
	@echo "Opening Grafana: http://localhost:3000"
	open http://localhost:3000 || xdg-open http://localhost:3000

kafka-ui: ## Open Kafka UI
	@echo "Opening Kafka UI: http://localhost:8080"
	open http://localhost:8080 || xdg-open http://localhost:8080

##@ Default

.DEFAULT_GOAL := help
