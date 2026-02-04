# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy dependency files first for better caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the plugin binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o drone-gemini-plugin .

# Runtime stage
FROM alpine:3.19

# Add non-root user for security
RUN addgroup -S appuser && adduser -S appuser -G appuser

RUN apk add --no-cache ca-certificates tzdata git

WORKDIR /app

# Copy the compiled plugin binary with proper ownership
COPY --from=builder --chown=appuser:appuser /build/drone-gemini-plugin /bin/drone-gemini-plugin

# Switch to non-root user
USER appuser

# Set the entrypoint
ENTRYPOINT ["/bin/drone-gemini-plugin"]
