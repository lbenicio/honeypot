GO ?= go
DOCKER ?= docker
COMPOSE ?= $(DOCKER) compose
GOLANGCI_LINT ?= golangci-lint
IMAGE ?= fffaraz/fakessh
VERSION ?= $(shell grep -E '\[[0-9]+\.[0-9]+\.[0-9]+.*\] - [0-9]{4}-[0-9]{2}-[0-9]{2}$$' CHANGELOG.md | head -n 1 | cut -d '[' -f 2 | cut -d ']' -f 1)

.DEFAULT_GOAL := all

.PHONY: all help build test clean fmt lint docker-build docker-push docker-up docker-down

all: test build ## Build the application.

help: ## Show available targets.
	@awk 'BEGIN { FS = ":.*## " } /^[a-zA-Z0-9_-]+:.*## / { printf "  %-12s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build the executable at dist/fakessh.
	mkdir -p dist
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-w -s" -o dist/fakessh ./src

test: ## Run tests and require more than 98% statement coverage.
	@set -eu; \
	test_log=$$(mktemp); \
	trap 'rm -f "$$test_log"' EXIT; \
	if ! $(GO) test -covermode=atomic ./... > "$$test_log" 2>&1; then cat "$$test_log"; exit 1; fi; \
	cat "$$test_log"; \
	if ! awk '/coverage: [0-9.]+% of statements/ { found = 1; for (field = 1; field <= NF; field++) if ($$field == "coverage:") { coverage = $$(field + 1); sub(/%$$/, "", coverage); if ((coverage + 0) <= 98) { printf "Package coverage is %s%%; it must be greater than 98%%.\n", coverage > "/dev/stderr"; failed = 1 } } } END { if (!found || failed) exit 1 }' "$$test_log"; then \
		echo "Every Go package must report more than 98% statement coverage." >&2; \
		exit 1; \
	fi; \
	echo "Coverage requirement met: every Go package is above 98%."

fmt: ## Format Go files and apply available lint fixes.
	gofmt -w ./src
	$(GOLANGCI_LINT) run --fix ./...

lint: ## Check Go formatting and run static analysis.
	@unformatted="$$(gofmt -l ./src)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Go files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	$(GOLANGCI_LINT) run ./...

docker-build: ## Build versioned and latest Docker image tags from CHANGELOG.md.
	@test -n "$(VERSION)" || { echo "No dated release version found in CHANGELOG.md." >&2; exit 1; }
	$(DOCKER) build --file deployment/Dockerfile --tag $(IMAGE):$(VERSION) --tag $(IMAGE):latest .

docker-push: docker-build ## Build and push the versioned and latest Docker tags.
	$(DOCKER) push $(IMAGE):$(VERSION)
	$(DOCKER) push $(IMAGE):latest

docker-up: docker-build ## Build and start the Compose service.
	$(COMPOSE) --file deployment/docker-compose.yml up --detach --no-build

docker-down: ## Stop and remove the Compose service.
	$(COMPOSE) --file deployment/docker-compose.yml down

clean: ## Remove generated build artifacts.
	rm -rf dist
