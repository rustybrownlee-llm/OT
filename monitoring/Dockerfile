# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy module files first for layer caching
COPY go.mod ./
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

EXPOSE 8090 8091

ENTRYPOINT ["/monitor"]
CMD ["--config", "/config/monitor.yaml"]
