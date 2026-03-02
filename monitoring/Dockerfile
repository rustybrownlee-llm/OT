# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy module files first for layer caching.
# go.sum is required for dependency resolution.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /monitor ./cmd/monitor

# Stage 2: Minimal runtime image
FROM scratch

# Copy CA certificates for any future TLS connections
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the monitor binary
COPY --from=builder /monitor /monitor

# Copy config directory as fallback. The docker-compose volume mount (./monitoring/config:/config:ro)
# takes precedence at runtime. This COPY ensures the image is self-contained when run standalone.
COPY config/ /config/

EXPOSE 8090 8091

ENTRYPOINT ["/monitor"]
CMD ["--config", "/config/monitor.yaml"]
