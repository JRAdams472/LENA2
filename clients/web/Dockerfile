# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .
ENV NEXT_PUBLIC_LENA_API_URL=/graphql
RUN npm run build

# Runtime stage
FROM node:20-alpine

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_PUBLIC_LENA_API_URL=/graphql
ENV PORT=3000

COPY --from=builder /app/package*.json ./
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/.next ./.next

EXPOSE 3000

USER node

CMD ["npm", "start"]
