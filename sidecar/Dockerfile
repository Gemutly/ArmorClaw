# ---- Stage 1: Builder — compile Rust sidecar binary ----
FROM rust:1.82-slim AS builder

# Build dependencies for release: cmake + clang required by tract-onnx / aws-sdk-s3
RUN apt-get update && \
    apt-get install -y --no-install-recommends cmake clang protobuf-compiler && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Cache dependency compilation layer
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo "fn main() {}" > src/main.rs && \
    cargo build --release && rm -rf src

# Build actual binary with real source
COPY . .
RUN touch src/main.rs && cargo build --release

# ---- Stage 2: Runtime — minimal hardened image ----
FROM debian:bookworm-slim

# CA certs for S3/Azure TLS (rustls uses system root certs)
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Non-root user (UID 10001 matches other ArmorClaw sidecars)
RUN groupadd -g 10001 sidecar && \
    useradd -u 10001 -g 10001 -m -s /sbin/nologin sidecar

# Socket directory
RUN mkdir -p /run/armorclaw && chown sidecar:sidecar /run/armorclaw

# Copy binary from builder
COPY --from=builder /app/target/release/armorclaw-sidecar /usr/local/bin/armorclaw-sidecar

RUN chown sidecar:sidecar /usr/local/bin/armorclaw-sidecar

USER sidecar:sidecar

# Env defaults (overridable via compose environment)
ENV ARMORCLAW_SIDECAR__SOCKET_PATH=/run/armorclaw/sidecar-rust.sock

CMD ["armorclaw-sidecar"]
