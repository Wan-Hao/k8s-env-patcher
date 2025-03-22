.PHONY: help deploy clean check-deps check-env create-cluster create-namespace test

# Default cluster name and namespace
CLUSTER_NAME ?= env-injector
NAMESPACE ?= injector
CREATE_CLUSTER ?= false
FORCE ?= false

# Scripts path
SCRIPTS_DIR = bin

help: ## 📚 Show this help message
	@echo "Usage: make [target] [CLUSTER_NAME=name] [NAMESPACE=name] [CREATE_CLUSTER=true|false] [FORCE=true|false]"
	@echo ""
	@echo "Targets:"
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
		helpMessage = match(lastLine, /^## (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 3, RLENGTH); \
			printf "  ${YELLOW}%-20s${NC} %s\n", helpCommand, helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "Examples:"
	@echo "  make deploy CLUSTER_NAME=my-cluster NAMESPACE=my-ns"
	@echo "  make deploy CREATE_CLUSTER=true FORCE=true"
	@echo "  make create-namespace NAMESPACE=my-ns FORCE=true"
	@echo "  make create-cluster CLUSTER_NAME=my-cluster FORCE=true"
	@echo "  make test NAMESPACE=my-ns"

check-deps: ## 🔍 Check if required dependencies are installed
	@echo "Checking dependencies..."
	@which kubectl >/dev/null 2>&1 || (echo "❌ kubectl is required but not installed. Please install kubectl first." && exit 1)
	@which kind >/dev/null 2>&1 || (echo "❌ kind is required but not installed. Please install kind first." && exit 1)
	@which docker >/dev/null 2>&1 || (echo "❌ docker is required but not installed. Please install docker first." && exit 1)
	@which jq >/dev/null 2>&1 || (echo "❌ jq is required but not installed. Please install jq first." && exit 1)
	@echo "✅ All dependencies are installed"

check-env: check-deps ## 🔎 Check cluster and namespace existence
	@echo "Checking environment..."
	@if ! kind get clusters | grep -q "^$(CLUSTER_NAME)$$"; then \
		echo "❌ Cluster $(CLUSTER_NAME) does not exist"; \
		exit 1; \
	fi
	@echo "✅ Cluster $(CLUSTER_NAME) exists"
	@if ! kubectl get namespace $(NAMESPACE) >/dev/null 2>&1; then \
		echo "❌ Namespace $(NAMESPACE) does not exist"; \
		exit 1; \
	fi
	@echo "✅ Namespace $(NAMESPACE) exists"

create-cluster: check-deps ## 🎯 Create a new cluster
	@echo "Creating cluster $(CLUSTER_NAME)..."
	@if [ "$(FORCE)" = "true" ]; then \
		$(SCRIPTS_DIR)/deploy.sh -c $(CLUSTER_NAME) -s -f; \
	else \
		$(SCRIPTS_DIR)/deploy.sh -c $(CLUSTER_NAME) -s; \
	fi

create-namespace: check-deps ## 📁 Create a new namespace in existing cluster
	@echo "Creating namespace $(NAMESPACE) in cluster $(CLUSTER_NAME)..."
	@if [ "$(FORCE)" = "true" ]; then \
		$(SCRIPTS_DIR)/deploy.sh -c $(CLUSTER_NAME) -n $(NAMESPACE) -f; \
	else \
		$(SCRIPTS_DIR)/deploy.sh -c $(CLUSTER_NAME) -n $(NAMESPACE); \
	fi

deploy: check-deps ## 🚀 Deploy the webhook to kubernetes cluster
	@echo "Starting deployment..."
	@if [ "$(CREATE_CLUSTER)" = "true" ]; then \
		if [ "$(FORCE)" = "true" ]; then \
			$(SCRIPTS_DIR)/deploy.sh -c $(CLUSTER_NAME) -n $(NAMESPACE) -s -f; \
		else \
			$(SCRIPTS_DIR)/deploy.sh -c $(CLUSTER_NAME) -n $(NAMESPACE) -s; \
		fi \
	else \
		if [ "$(FORCE)" = "true" ]; then \
			$(SCRIPTS_DIR)/deploy.sh -c $(CLUSTER_NAME) -n $(NAMESPACE) -f; \
		else \
			$(SCRIPTS_DIR)/deploy.sh -c $(CLUSTER_NAME) -n $(NAMESPACE); \
		fi \
	fi

test: check-deps ## 🧪 Run integration tests
	@echo "Running integration tests..."
	@$(SCRIPTS_DIR)/test.sh -n $(NAMESPACE)

test-all: deploy test ## 🔬 Deploy and run all tests
	@echo "Deployment and tests completed"

clean: ## 🧹 Clean up resources (delete namespace and cluster if specified)
	@echo "Cleaning up resources..."
	@kubectl delete namespace $(NAMESPACE) --ignore-not-found --timeout=60s
	@if [ "$(CREATE_CLUSTER)" = "true" ]; then \
		kind delete cluster --name $(CLUSTER_NAME); \
	fi
	@echo "✅ Cleanup completed"

# Define colors
YELLOW := \033[1;33m
NC := \033[0m 