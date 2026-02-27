# Build context: . (repo root)
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/controlpanel-api ./cmd/controlpanel-api

# Install goose for migrations (run via init container)
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /bin/controlpanel-api /bin/controlpanel-api
COPY --from=builder /go/bin/goose /bin/goose
COPY --from=builder /app/migrations/controlpanel /migrations
EXPOSE 8080
ENTRYPOINT ["/bin/controlpanel-api"]
