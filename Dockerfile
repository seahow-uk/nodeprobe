# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with static linking and optimization
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o nodeprobe \
    ./cmd/nodeprobe

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies and certificates
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    sqlite \
    curl \
    && update-ca-certificates

# Create non-root user for security
RUN adduser -D -s /bin/sh nodeprobe

# Create application directories
RUN mkdir -p /app/data /app/certs /app/logs && \
    chown -R nodeprobe:nodeprobe /app

# Copy the binary from builder stage
COPY --from=builder /build/nodeprobe /app/nodeprobe

# Copy any additional files if needed
# COPY configs/ /app/configs/

# Set proper permissions
RUN chmod +x /app/nodeprobe

# Switch to non-root user
USER nodeprobe

# Set working directory
WORKDIR /app

# Expose HTTPS port
EXPOSE 443

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -k -f https://localhost:443/health || exit 1

# Set environment variables
ENV NODE_ENV=production
ENV DATA_DIR=/app/data
ENV CERT_DIR=/app/certs

# Run the application
CMD ["./nodeprobe"] 