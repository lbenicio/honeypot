GO ?= go

.DEFAULT_GOAL := all

.PHONY: all help build test clean

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

clean: ## Remove generated build artifacts.
	rm -rf dist
