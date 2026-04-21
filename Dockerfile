# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies (including gcc and musl-dev for CGO/SQLite support)
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /app

# Copy module files and download deps
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY src ./src

# Build the binary with CGO enabled for SQLite support
# Note: CGO_ENABLED=1 is required for github.com/mattn/go-sqlite3
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/bincrypt ./src

# Runtime stage
FROM alpine:latest

# Install runtime dependencies (HTTPS chains, timezone data, health-check client, SQLite)
RUN apk --no-cache add ca-certificates tzdata wget sqlite

# Create non-root user
RUN addgroup -g 1000 -S bincrypt && \
    adduser -u 1000 -S bincrypt -G bincrypt

# Create data directory with proper ownership BEFORE switching user
# This is critical for SQLite backend functionality
RUN mkdir -p /data && \
    chown -R bincrypt:bincrypt /data

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bincrypt .

# Change ownership
RUN chown -R bincrypt:bincrypt /app

# Switch to non-root user
USER bincrypt

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# Declare volume for persistent data
VOLUME /data

# Run the binary
ENTRYPOINT ["./bincrypt"]
