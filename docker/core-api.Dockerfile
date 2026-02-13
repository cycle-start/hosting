FROM golang:1.25-alpine AS builder
RUN go install github.com/swaggo/swag/cmd/swag@latest
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN swag init -g internal/api/doc.go -o internal/api/docs --parseDependency --parseInternal
RUN CGO_ENABLED=0 go build -o /bin/core-api ./cmd/core-api

FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /bin/core-api /bin/core-api
COPY --from=builder /app/migrations /migrations
ENTRYPOINT ["/bin/core-api"]
