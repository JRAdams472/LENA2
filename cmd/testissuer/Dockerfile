# syntax=docker/dockerfile:1
# Build stage — Go toolchain matches go.mod. Pinned by digest: golang 1.27.1-alpine.
FROM golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Narrow copy: only the trees the binary needs.
COPY cmd/ cmd/
COPY internal/ internal/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /build/testissuer ./cmd/testissuer

# Runtime stage — alpine 3.24, pinned by digest.
FROM alpine@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

WORKDIR /app

COPY --from=builder /build/testissuer /app/testissuer

EXPOSE 8085

USER nobody

ENTRYPOINT ["/app/testissuer"]
