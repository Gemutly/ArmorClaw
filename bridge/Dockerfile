# ArmorClaw Bridge Dockerfile
# This Dockerfile builds the bridge with CGO enabled for SQLCipher support
# NOTE: Must use Debian (glibc) instead of Alpine (musl) because go-sqlcipher
# requires glibc-specific functions (pread64, pwrite64, off64_t)

FROM golang:1.25-bookworm AS builder

# Install build dependencies for CGO, SQLCipher, and libolm
RUN echo "deb http://deb.debian.org/debian bookworm-backports main" > /etc/apt/sources.list.d/bookworm-backports.list && \
    apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc-dev \
    libsqlcipher-dev \
    libolm-dev \
    libyara-dev/bookworm-backports \
    pkg-config \
    git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with CGO enabled for SQLCipher support
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -o armorclaw-bridge \
    ./cmd/bridge

# Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies
RUN echo "deb http://deb.debian.org/debian bookworm-backports main" > /etc/apt/sources.list.d/bookworm-backports.list && \
    apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libsqlcipher0 \
    libolm3 \
    libyara10/bookworm-backports \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Download and install Catwalk (provider/model registry)
# Catwalk v0.28.3 for Linux amd64
ENV CATWALK_VERSION=0.28.3
RUN curl -sL "https://github.com/charmbracelet/catwalk/releases/download/v${CATWALK_VERSION}/catwalk_${CATWALK_VERSION}_linux_amd64.tar.gz" | tar xz -C /usr/local/bin catwalk && \
    chmod +x /usr/local/bin/catwalk

# Copy Docker CLI from official image (ensures API compatibility with modern Docker daemons)
COPY --from=docker:cli /usr/local/bin/docker /usr/local/bin/docker

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/armorclaw-bridge /usr/local/bin/

# Copy example config
COPY config.example.toml /etc/armorclaw/config.toml.example

# Create shared group for cross-service /run/armorclaw access
RUN groupadd -g 10001 armorclaw-runtime || true

# Create directories with correct permissions
RUN mkdir -p /etc/armorclaw /run/armorclaw /var/lib/armorclaw && \
    chown -R 10002:10002 /etc/armorclaw /var/lib/armorclaw && \
    chown 10002:10001 /run/armorclaw && \
    chmod 0750 /run/armorclaw

# Set up non-root user with supplementary group
USER 10002:10001

# Environment variables
ENV ARMORCLAW_CONFIG=/etc/armorclaw/config.toml
ENV ARMORCLAW_KEYSTORE_DB=/var/lib/armorclaw/keystore.db

# Health check
HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
    CMD pgrep armorclaw-bridge || exit 1

# Run the bridge
ENTRYPOINT ["/usr/local/bin/armorclaw-bridge"]

# Default: run in foreground (docker handles daemonization)
CMD []
