# Dockerfile for the OT Simulator plant binary.
# Build context must be the project root (not plant/) so that design/ is accessible.
# docker-compose.yml sets context: . (project root) and dockerfile: plant/Dockerfile.

# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy plant module files first for layer caching
COPY plant/go.mod plant/go.sum ./
RUN go mod download

# Copy plant source and build
COPY plant/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /plant ./cmd/plant

# Stage 2: Minimal runtime image
FROM scratch

# Copy CA certificates for any future TLS connections
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the plant binary
COPY --from=builder /plant /plant

# Copy design layer (device atoms, network atoms, environment templates)
# Source path is relative to the project root build context.
COPY design/ /design/

EXPOSE 5020-5039 8080 8081

ENTRYPOINT ["/plant"]
CMD ["--environment", "/design/environments/greenfield-water-mfg", "--log-level", "info"]
