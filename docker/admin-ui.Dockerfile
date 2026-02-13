# Stage 1: Build the React SPA
FROM node:22-alpine AS frontend
WORKDIR /app
COPY web/admin/package.json web/admin/package-lock.json ./
RUN npm ci
COPY web/admin/ .
RUN npm run build

# Stage 2: Build the Go binary
FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/admin-ui ./cmd/admin-ui

# Stage 3: Final image
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=backend /bin/admin-ui /bin/admin-ui
COPY --from=frontend /app/dist ./dist
EXPOSE 3001
CMD ["/bin/admin-ui"]
