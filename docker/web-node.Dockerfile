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
    software-properties-common gnupg2 curl ca-certificates && \
    add-apt-repository -y ppa:ondrej/php && \
    apt-get update && apt-get install -y --no-install-recommends \
    nginx \
    php8.5-fpm php8.5-mbstring php8.5-curl php8.5-mysql php8.5-pgsql php8.5-xml php8.5-zip \
    nodejs npm \
    python3 \
    ruby \
    openssh-server \
    bash \
    && rm -rf /var/lib/apt/lists/*

# Create hosting user structure
RUN mkdir -p /srv/hosting /etc/nginx/sites-enabled /var/log/nginx /run/nginx /var/www/storage /run/php && \
    rm -f /etc/nginx/sites-enabled/default

# Copy node-agent binary
COPY --from=builder /node-agent /usr/local/bin/node-agent

# Copy entrypoint script
COPY docker/web-node-entrypoint.sh /usr/local/bin/web-node-entrypoint.sh
RUN chmod +x /usr/local/bin/web-node-entrypoint.sh

# Expose ports: HTTP, HTTPS, node-agent gRPC
EXPOSE 80 443 9090

ENTRYPOINT ["/usr/local/bin/web-node-entrypoint.sh"]
