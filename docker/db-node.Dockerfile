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
    curl ca-certificates bash gnupg \
    && curl -fsSL https://repo.mysql.com/RPM-GPG-KEY-mysql-2023 | gpg --dearmor -o /usr/share/keyrings/mysql.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/mysql.gpg trusted=yes] http://repo.mysql.com/apt/ubuntu/ noble mysql-8.4-lts" \
       > /etc/apt/sources.list.d/mysql.list \
    && apt-get update && apt-get install -y --no-install-recommends \
    mysql-server mysql-client \
    && rm -rf /var/lib/apt/lists/*

# Copy node-agent binary
COPY --from=builder /node-agent /usr/local/bin/node-agent

# Expose ports: MySQL, node-agent gRPC
EXPOSE 3306 9090

# Custom entrypoint that starts both MySQL and node-agent
COPY docker/db-node-entrypoint.sh /usr/local/bin/db-node-entrypoint.sh
RUN chmod +x /usr/local/bin/db-node-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/db-node-entrypoint.sh"]
