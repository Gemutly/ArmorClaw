# Stage 1: Build Go binary
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /observer \
    ./cmd/observer

# Stage 2: Download Lightpanda with checksum verification
FROM alpine:latest AS lightpanda-fetch

RUN apk --no-cache add curl coreutils
RUN curl -L -o /lightpanda https://github.com/lightpanda-io/browser/releases/download/v0.2.6/lightpanda-x86_64-linux

# SECURITY: Verify SHA256 checksum (from GitHub release asset digest)
RUN echo "f29442646d1c62a4175195b0a149b9e368e2f3487d3a493f96ba0702be8974cc  /lightpanda" | sha256sum -c -

RUN chmod +x /lightpanda

# Stage 3: Final minimal image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

# Copy binaries
COPY --from=builder /observer /usr/local/bin/observer
COPY --from=lightpanda-fetch /lightpanda /usr/local/bin/lightpanda

WORKDIR /app
EXPOSE 9222 9223

CMD ["observer", "--port=9222"]
