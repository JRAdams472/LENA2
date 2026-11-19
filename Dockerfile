# syntax=docker/dockerfile:1
# Build stage — toolchain image must be >= the go.mod directive
# (currently `go 1.26.10`); keep both in sync when bumping either.
FROM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS builder

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
FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf AS debug

RUN apk add --no-cache ca-certificates curl

WORKDIR /app

COPY --from=builder /build/lena /app/lena
RUN mkdir -p /data/import/inbox /data/import/work && chown -R 65534:65534 /data/import

EXPOSE 8080

USER nobody

ENTRYPOINT ["/app/lena"]
