# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.23.6-alpine AS builder

# Install build dependencies including C standard library headers
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimized flags
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -o player-api .

# Runtime stage
FROM alpine:latest

# Install runtime dependencies including ffmpeg
RUN apk --no-cache add ca-certificates tzdata ffmpeg wget bash libstdc++ libgomp

# Create non-root user for runtime
RUN adduser -D -g '' appuser

# Create directories
RUN mkdir -p /app/data /app/models /app/bin

# Copy binary from builder stage
COPY --from=builder /build/player-api /app/

# Copy any static files if needed
COPY --from=builder /build/data /app/data

# Copy Docker-specific config file
COPY --from=builder /build/config/docker.yaml /app/config.yaml

# Set proper ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Set working directory
WORKDIR /app

# Set whisper configuration environment variables
ENV KILLALL_TRANSCRIPTION_WHISPER_PATH=/app/bin/whisper \
    KILLALL_TRANSCRIPTION_MODEL_PATH=/app/models/ggml-base.en.bin \
    KILLALL_TRANSCRIPTION_LANGUAGE=en \
    KILLALL_TRANSCRIPTION_ENABLED=true

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
CMD ["./player-api", "serve"]