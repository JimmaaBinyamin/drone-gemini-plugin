# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates upx

WORKDIR /build

# Copy dependency files first for better caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build with maximum optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -trimpath \
    -o drone-gemini-plugin .

# Compress binary with UPX (reduces size by ~60%)
RUN upx --best --lzma drone-gemini-plugin || true

# Runtime stage - use scratch for minimal image
FROM scratch

# Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the compressed binary
COPY --from=builder /build/drone-gemini-plugin /bin/drone-gemini-plugin

# Set the entrypoint
ENTRYPOINT ["/bin/drone-gemini-plugin"]
