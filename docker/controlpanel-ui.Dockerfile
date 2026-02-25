# Build context: hosting repo (default), ../controlpanel (named "controlpanel")
# Build with: docker build --build-context controlpanel=../controlpanel ...

# Stage 1: Build the React SPA from the controlpanel repo
FROM node:22-alpine AS frontend
WORKDIR /app
COPY --from=controlpanel package.json package-lock.json ./
RUN npm ci
COPY --from=controlpanel . .
RUN npm run build

# Stage 2: Build the Go SPA server from the hosting repo
FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/controlpanel-ui ./cmd/controlpanel-ui

# Stage 3: Final image
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=backend /bin/controlpanel-ui /bin/controlpanel-ui
COPY --from=frontend /app/dist ./dist
EXPOSE 3002
CMD ["/bin/controlpanel-ui"]
