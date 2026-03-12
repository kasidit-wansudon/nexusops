# ---- Dependencies Stage ----
FROM node:20-alpine AS deps

WORKDIR /app

# Copy only package files first for optimal layer caching.
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev

# ---- Build Stage ----
FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci

COPY . .

# If a build step exists (e.g., TypeScript compilation), run it here.
RUN npm run build --if-present

# ---- Runtime Stage ----
FROM node:20-alpine

RUN apk add --no-cache tini && \
    addgroup -S appgroup && \
    adduser -S appuser -G appgroup

WORKDIR /app

# Copy production dependencies from the deps stage.
COPY --from=deps /app/node_modules ./node_modules

# Copy application source from the builder stage.
COPY --from=builder /app/src ./src
COPY --from=builder /app/package.json ./

# Drop privileges.
USER appuser

EXPOSE 3000

# Use tini as PID 1 to handle signals correctly.
ENTRYPOINT ["/sbin/tini", "--"]

CMD ["node", "src/index.js"]

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD ["wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:3000/health"]
