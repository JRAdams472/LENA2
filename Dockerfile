# Build stage
FROM golang:1.27-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /build/lena ./cmd/lena

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates curl

WORKDIR /app

COPY --from=builder /build/lena /app/lena

EXPOSE 8080

USER nobody

ENTRYPOINT ["/app/lena"]
