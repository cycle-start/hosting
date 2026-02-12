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
    pdns-server pdns-backend-pgsql \
    curl ca-certificates \
    bash \
    && rm -rf /var/lib/apt/lists/*

# Copy node-agent binary
COPY --from=builder /node-agent /usr/local/bin/node-agent

# Expose ports: DNS (UDP+TCP), node-agent gRPC
EXPOSE 53/udp 53/tcp 9090

# Custom entrypoint that starts both PowerDNS and node-agent
COPY docker/dns-node-entrypoint.sh /usr/local/bin/dns-node-entrypoint.sh
RUN chmod +x /usr/local/bin/dns-node-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/dns-node-entrypoint.sh"]
