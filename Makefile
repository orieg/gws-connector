BINARY := gws-mcp
MODULE := github.com/orieg/gws-connector
VERSION := 0.4.2

GO ?= $(shell which go)

.PHONY: build install clean test lint release check-versions bump-version

build:
	$(GO) build -o bin/$(BINARY) ./cmd/gws-mcp

install: build
	@echo "Binary built at bin/$(BINARY)"
	@echo "Install the plugin with: /plugin install /path/to/gws-connector"

clean:
	rm -rf bin/

test:
	$(GO) test -race -count=1 ./...

test-verbose:
	$(GO) test -race -v -count=1 ./...

lint:
	$(GO) vet ./...

# Verify every release manifest carries the same version.
check-versions:
	scripts/check-versions.sh

# Set the version in every release manifest: make bump-version V=0.4.2
bump-version:
	@test -n "$(V)" || (echo "usage: make bump-version V=<major.minor.patch>" >&2; exit 2)
	scripts/bump-version.sh $(V)

# Cross-compile for release
release:
	GOOS=darwin GOARCH=arm64 $(GO) build -o bin/$(BINARY)-darwin-arm64 ./cmd/gws-mcp
	GOOS=darwin GOARCH=amd64 $(GO) build -o bin/$(BINARY)-darwin-amd64 ./cmd/gws-mcp
	GOOS=linux GOARCH=amd64 $(GO) build -o bin/$(BINARY)-linux-amd64 ./cmd/gws-mcp
	GOOS=linux GOARCH=arm64 $(GO) build -o bin/$(BINARY)-linux-arm64 ./cmd/gws-mcp
