# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY src/go.mod src/go.sum ./src/
WORKDIR /app/src

# Download dependencies
RUN go mod download

# Copy source code
COPY src/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/bincrypt .

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S bincrypt && \
    adduser -u 1000 -S bincrypt -G bincrypt

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
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Run the binary
ENTRYPOINT ["./bincrypt"]