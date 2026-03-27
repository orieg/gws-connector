BINARY := gws-mcp
MODULE := github.com/orieg/gws-connector
VERSION := 0.1.0

GO ?= $(shell which go)

.PHONY: build install clean test lint release

build:
	$(GO) build -o bin/$(BINARY) ./cmd/gws-mcp

install: build
	@echo "Binary built at bin/$(BINARY)"
	@echo "Install the plugin with: /plugin install /path/to/gws-connector"

clean:
	rm -rf bin/

test:
	$(GO) test ./... -count=1

test-verbose:
	$(GO) test ./... -v -count=1

lint:
	$(GO) vet ./...

# Cross-compile for release
release:
	GOOS=darwin GOARCH=arm64 $(GO) build -o bin/$(BINARY)-darwin-arm64 ./cmd/gws-mcp
	GOOS=darwin GOARCH=amd64 $(GO) build -o bin/$(BINARY)-darwin-amd64 ./cmd/gws-mcp
	GOOS=linux GOARCH=amd64 $(GO) build -o bin/$(BINARY)-linux-amd64 ./cmd/gws-mcp
	GOOS=linux GOARCH=arm64 $(GO) build -o bin/$(BINARY)-linux-arm64 ./cmd/gws-mcp
