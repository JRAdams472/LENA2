# Build stage
FROM golang:1.27-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /build/lena ./cmd/lena

# Runtime stage
FROM alpine:3.24

RUN apk add --no-cache ca-certificates curl

WORKDIR /app

COPY --from=builder /build/lena /app/lena

# Import inbox shared with the host for recipe OCR uploads.
RUN mkdir -p /data/import/inbox /data/import/work && chown -R 65534:65534 /data/import

EXPOSE 8080

USER nobody

ENTRYPOINT ["/app/lena"]
