# Stage 1: Build the Go binary
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY controlpanel-api-build/go.mod controlpanel-api-build/go.sum ./
RUN go mod download
COPY controlpanel-api-build/ .
RUN CGO_ENABLED=0 go build -o /bin/controlpanel-api ./cmd/api

# Install goose for migrations
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

# Stage 2: Final image
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /bin/controlpanel-api /bin/controlpanel-api
COPY --from=builder /go/bin/goose /bin/goose
COPY --from=builder /app/migrations /migrations
EXPOSE 8080
ENTRYPOINT ["/bin/controlpanel-api"]
