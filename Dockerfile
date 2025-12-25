
# syntax=docker/dockerfile:1.7

## Multi-target Dockerfile for Go backends (dev + prod)
##
## Usage:
##   Dev (live reload):
##     docker build --target=dev -t signed-webhook:dev .
##     docker run --rm -p 8085:8085 -v "$PWD":/workspace signed-webhook:dev
##
##   Prod:
##     docker build --target=prod -t signed-webhook:prod .
##     docker run --rm -p 8443:8443 signed-webhook:prod

ARG GO_VERSION=1.23
ARG APP_PATH=./cmd/webhook
ARG BIN_NAME=webhook

#############################
# Base build environment
#############################
FROM golang:${GO_VERSION}-bookworm AS build-base
WORKDIR /src

# Git is often needed for go modules; ca-certs for TLS.
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates git && \
    rm -rf /var/lib/apt/lists/*

#############################
# Dependencies layer (cache)
#############################
FROM build-base AS deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

#############################
# Development target
#############################
FROM build-base AS dev
WORKDIR /workspace

# Install a lightweight live-reload tool.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/air-verse/air@latest

# Copy source for a default "docker run" without a volume; in practice you can mount your code.
COPY . .

ENV PORT=8085
ENV TLS_ENABLED=false
EXPOSE 8085

# Default air configuration watches the current directory.
CMD ["air", "-c", "/workspace/.air.toml"]

#############################
# Build (compile binary)
#############################
FROM deps AS build
ARG APP_PATH
ARG BIN_NAME

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/${BIN_NAME} ${APP_PATH}

#############################
# Production runtime target
#############################
FROM gcr.io/distroless/base-debian12:nonroot AS prod
ARG BIN_NAME

WORKDIR /
COPY --from=build /out/${BIN_NAME} /${BIN_NAME}

ENV PORT=8443
EXPOSE 8443

USER nonroot:nonroot
ENTRYPOINT ["/webhook"]
