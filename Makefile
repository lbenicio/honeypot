GO ?= go

.PHONY: build test clean

build:
	mkdir -p dist
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-w -s" -o dist/fakessh ./src

test:
	$(GO) test ./...

clean:
	rm -rf dist
