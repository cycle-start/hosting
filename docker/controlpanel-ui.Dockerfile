# Build context: . (repo root)

# Stage 1: Build the React SPA
FROM node:22-alpine AS frontend
WORKDIR /app
COPY web/controlpanel/package.json web/controlpanel/package-lock.json ./
RUN npm ci
COPY web/controlpanel/ .
RUN npm run build

# Stage 2: Build the Go SPA server
FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/controlpanel-ui/ cmd/controlpanel-ui/
RUN CGO_ENABLED=0 go build -o /bin/controlpanel-ui ./cmd/controlpanel-ui

# Stage 3: Final image
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=backend /bin/controlpanel-ui /bin/controlpanel-ui
COPY --from=frontend /app/dist ./dist
EXPOSE 3002
CMD ["/bin/controlpanel-ui"]
