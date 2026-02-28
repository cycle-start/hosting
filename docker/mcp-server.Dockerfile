FROM golang:1.25-alpine AS builder
RUN go install github.com/swaggo/swag/cmd/swag@latest
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN swag init -g internal/api/doc.go -o internal/api/docs --parseDependency --parseInternal --exclude internal/controlpanel
RUN CGO_ENABLED=0 go build -o /bin/mcp-server ./cmd/mcp-server

FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /bin/mcp-server /bin/mcp-server
COPY --from=builder /app/internal/api/docs/swagger.json /etc/mcp-server/swagger.json
COPY --from=builder /app/mcp.yaml /etc/mcp-server/mcp.yaml
ENTRYPOINT ["/bin/mcp-server", "--config", "/etc/mcp-server/mcp.yaml", "--spec", "/etc/mcp-server/swagger.json"]
