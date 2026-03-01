# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy module files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /plant ./cmd/plant

# Stage 2: Minimal runtime image
FROM scratch

# Copy CA certificates for any future TLS connections
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the plant binary
COPY --from=builder /plant /plant

# Copy default configuration
COPY --from=builder /build/config/ /config/

EXPOSE 5020-5039 8080 8081

ENTRYPOINT ["/plant"]
CMD ["--config", "/config/plant.yaml"]
