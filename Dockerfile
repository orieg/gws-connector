# syntax=docker/dockerfile:1

# --- Build stage ---
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -buildvcs=false \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/gws-mcp ./cmd/gws-mcp

# --- Runtime stage ---
# Minimal static image; distroless static includes CA certificates for HTTPS
# to the Google APIs.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/gws-mcp /usr/local/bin/gws-mcp

# GWS Connector speaks MCP over stdio.
#
# NOTE: interactive OAuth (accounts.add / reauth) opens a browser and stores
# secrets in the OS keychain, which needs host access — so day-to-day use is via
# the native binary, the Claude Code plugin, or the Gemini extension. This image
# is aimed at MCP registry/introspection checks and headless stdio integrations.
# To persist the account registry across runs, mount a volume and point
# GWS_STATE_DIR at it, e.g.:
#   docker run -i -v gws-state:/state -e GWS_STATE_DIR=/state ghcr.io/orieg/gws-connector
ENTRYPOINT ["/usr/local/bin/gws-mcp", "--use-dot-names"]
