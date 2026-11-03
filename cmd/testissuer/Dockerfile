# syntax=docker/dockerfile:1
# Build stage — Go toolchain matches go.mod. Pinned by digest: golang 1.27.1-alpine.
FROM golang@sha256:2920bb124f1a2e12a1d39f0cab756ae0068441ee1da7655dec7f5e374944095b AS builder

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
FROM alpine@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

WORKDIR /app

COPY --from=builder /build/testissuer /app/testissuer

EXPOSE 8085

USER nobody

ENTRYPOINT ["/app/testissuer"]
