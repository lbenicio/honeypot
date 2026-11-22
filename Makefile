GO ?= go

.DEFAULT_GOAL := all

.PHONY: all help build test clean

all: test build ## Build the application.

help: ## Show available targets.
	@awk 'BEGIN { FS = ":.*## " } /^[a-zA-Z0-9_-]+:.*## / { printf "  %-12s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build the executable at dist/fakessh.
	mkdir -p dist
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-w -s" -o dist/fakessh ./src

test: ## Run the Go test suite.
	$(GO) test ./...

clean: ## Remove generated build artifacts.
	rm -rf dist
