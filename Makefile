BINARY := gws-mcp
MODULE := github.com/orieg/claude-multi-gws
VERSION := 0.1.0

GO := /opt/homebrew/bin/go

.PHONY: build install clean test lint release

build:
	$(GO) build -o bin/$(BINARY) ./cmd/gws-mcp

install: build
	@echo "Binary built at bin/$(BINARY)"
	@echo "Add to your Claude Code plugin by pointing .mcp.json to this binary."

clean:
	rm -rf bin/

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...

# Cross-compile for release
release:
	GOOS=darwin GOARCH=arm64 $(GO) build -o bin/$(BINARY)-darwin-arm64 ./cmd/gws-mcp
	GOOS=darwin GOARCH=amd64 $(GO) build -o bin/$(BINARY)-darwin-amd64 ./cmd/gws-mcp
	GOOS=linux GOARCH=amd64 $(GO) build -o bin/$(BINARY)-linux-amd64 ./cmd/gws-mcp
	GOOS=linux GOARCH=arm64 $(GO) build -o bin/$(BINARY)-linux-arm64 ./cmd/gws-mcp
