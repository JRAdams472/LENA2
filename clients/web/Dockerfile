# syntax=docker/dockerfile:1
# Build stage — Node 22 LTS matches CI (node-version: 22 in test.yml).
FROM node:26.10.0-alpine@sha256:0b36e8c136b94cd4fcf02188228e76c31ad5872eef3fec8cbd2eee500cfd9e80 AS builder
WORKDIR /app

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY . .
ARG NEXT_PUBLIC_API_BASE_URL="http://localhost"
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL
ARG NEXT_PUBLIC_GOOGLE_CLIENT_ID
ENV NEXT_PUBLIC_GOOGLE_CLIENT_ID=$NEXT_PUBLIC_GOOGLE_CLIENT_ID
ARG NEXT_PUBLIC_DISCORD_CLIENT_ID=""
ENV NEXT_PUBLIC_DISCORD_CLIENT_ID=$NEXT_PUBLIC_DISCORD_CLIENT_ID
ARG NEXT_PUBLIC_DISCORD_REDIRECT_URI=""
ENV NEXT_PUBLIC_DISCORD_REDIRECT_URI=$NEXT_PUBLIC_DISCORD_REDIRECT_URI
ARG NEXT_PUBLIC_MICROSOFT_CLIENT_ID=""
ENV NEXT_PUBLIC_MICROSOFT_CLIENT_ID=$NEXT_PUBLIC_MICROSOFT_CLIENT_ID
ARG NEXT_PUBLIC_MICROSOFT_TENANT=""
ENV NEXT_PUBLIC_MICROSOFT_TENANT=$NEXT_PUBLIC_MICROSOFT_TENANT
ARG NEXT_PUBLIC_MICROSOFT_REDIRECT_URI=""
ENV NEXT_PUBLIC_MICROSOFT_REDIRECT_URI=$NEXT_PUBLIC_MICROSOFT_REDIRECT_URI
ARG NEXT_PUBLIC_FACEBOOK_CLIENT_ID=""
ENV NEXT_PUBLIC_FACEBOOK_CLIENT_ID=$NEXT_PUBLIC_FACEBOOK_CLIENT_ID
ARG NEXT_PUBLIC_FACEBOOK_REDIRECT_URI=""
ENV NEXT_PUBLIC_FACEBOOK_REDIRECT_URI=$NEXT_PUBLIC_FACEBOOK_REDIRECT_URI
RUN if [ -z "$NEXT_PUBLIC_GOOGLE_CLIENT_ID" ] || [ "$NEXT_PUBLIC_GOOGLE_CLIENT_ID" = "__YOUR_GOOGLE_CLIENT_ID__" ] || [ "$NEXT_PUBLIC_GOOGLE_CLIENT_ID" = "dummy" ]; then \
        echo "ERROR: NEXT_PUBLIC_GOOGLE_CLIENT_ID build arg is required and cannot be the placeholder value."; \
        exit 1; \
    fi
RUN npm run build

# Runtime stage — distroless nodejs: no shell, npm, or OS tools for an
# attacker to pivot with. The image's entrypoint is node itself, and the
# :nonroot tag runs as uid 65532. This is the production target.
FROM gcr.io/distroless/nodejs24-debian13:nonroot@sha256:9eeb7f5887d0e239e78264b06f7f11d2e14be534050481803a9e4728fcdd278e AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

EXPOSE 3000

CMD ["server.js"]

# Debug stage — node alpine with a shell so `docker exec` works while
# testing. Selected via `target: debug` in docker-compose.debug.yml;
# never ship this target.
FROM node:26.10.0-alpine@sha256:0b36e8c136b94cd4fcf02188228e76c31ad5872eef3fec8cbd2eee500cfd9e80 AS debug
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

USER node

EXPOSE 3000

CMD ["node", "server.js"]
