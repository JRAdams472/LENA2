# syntax=docker/dockerfile:1
# Build stage — Go toolchain matches go.mod (go 1.26.6).
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS builder

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

# Created in the builder so the distroless stage can copy the directory
# (it has no mkdir); the import inbox is shared with the host for recipe
# OCR uploads.
RUN mkdir -p /data/import/inbox /data/import/work

# Runtime stage — distroless static: no shell, package manager, or OS
# tools an attacker could pivot with after an exploit. CA certs and a
# nonroot user (uid 65532) are baked into the image. This is the
# production target.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS runtime

WORKDIR /app

COPY --from=builder /build/lena /app/lena
COPY --from=builder --chown=65532:65532 /data/import /data/import

EXPOSE 8080

ENTRYPOINT ["/app/lena"]

# Debug stage — alpine with a shell and curl so `docker exec` works while
# testing. Selected via `target: debug` in docker-compose.debug.yml; never
# ship this target.
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6 AS debug

RUN apk add --no-cache ca-certificates curl

WORKDIR /app

COPY --from=builder /build/lena /app/lena
RUN mkdir -p /data/import/inbox /data/import/work && chown -R 65534:65534 /data/import

EXPOSE 8080

USER nobody

ENTRYPOINT ["/app/lena"]
