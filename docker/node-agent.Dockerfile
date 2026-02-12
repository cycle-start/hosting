FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/node-agent ./cmd/node-agent

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates nginx php8.3-fpm mysql-client openssh-server curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/node-agent /bin/node-agent
ENTRYPOINT ["/bin/node-agent"]
