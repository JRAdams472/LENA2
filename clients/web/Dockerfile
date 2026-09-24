# syntax=docker/dockerfile:1
# Build stage — Node 22 LTS matches CI (node-version: 22 in test.yml).
FROM node:26.9.0-alpine@sha256:dbaa92e5758cbbcf85d65d5403fdb530fe3442cbe8c6dbfb7ef23365450d5070 AS builder
WORKDIR /app

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY . .
ARG NEXT_PUBLIC_API_BASE_URL="http://localhost"
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL
ARG NEXT_PUBLIC_GOOGLE_CLIENT_ID
ENV NEXT_PUBLIC_GOOGLE_CLIENT_ID=$NEXT_PUBLIC_GOOGLE_CLIENT_ID
RUN if [ -z "$NEXT_PUBLIC_GOOGLE_CLIENT_ID" ] || [ "$NEXT_PUBLIC_GOOGLE_CLIENT_ID" = "__YOUR_GOOGLE_CLIENT_ID__" ] || [ "$NEXT_PUBLIC_GOOGLE_CLIENT_ID" = "dummy" ]; then \
        echo "ERROR: NEXT_PUBLIC_GOOGLE_CLIENT_ID build arg is required and cannot be the placeholder value."; \
        exit 1; \
    fi
RUN npm run build

# Runtime stage
FROM node:26.9.0-alpine@sha256:dbaa92e5758cbbcf85d65d5403fdb530fe3442cbe8c6dbfb7ef23365450d5070 AS runner
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
