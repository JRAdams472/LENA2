# syntax=docker/dockerfile:1
# Build stage — Go toolchain matches go.mod (go 1.26.6).
FROM golang:1.27.0-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Narrow copy: only the trees the binary needs (cmd, internal). The rest of
# the repository stays out of the build context — see .dockerignore.
COPY cmd/ cmd/
COPY internal/ internal/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /build/lena ./cmd/lena

# Runtime stage
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk add --no-cache ca-certificates curl

WORKDIR /app

COPY --from=builder /build/lena /app/lena

# Import inbox shared with the host for recipe OCR uploads.
RUN mkdir -p /data/import/inbox /data/import/work && chown -R 65534:65534 /data/import

EXPOSE 8080

USER nobody

ENTRYPOINT ["/app/lena"]
