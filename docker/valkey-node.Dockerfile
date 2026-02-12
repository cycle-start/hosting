# Build stage: compile the node-agent binary
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /node-agent ./cmd/node-agent

# Runtime stage
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    valkey-server valkey-tools \
    curl ca-certificates \
    bash \
    && rm -rf /var/lib/apt/lists/*

# Create data directories
RUN mkdir -p /var/lib/valkey /etc/valkey /run/valkey

# Copy node-agent binary
COPY --from=builder /node-agent /usr/local/bin/node-agent

# Expose ports: Valkey, node-agent gRPC
EXPOSE 6379 9090

# Custom entrypoint
COPY docker/valkey-node-entrypoint.sh /usr/local/bin/valkey-node-entrypoint.sh
RUN chmod +x /usr/local/bin/valkey-node-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/valkey-node-entrypoint.sh"]
