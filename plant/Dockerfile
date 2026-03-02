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

# EXPOSE documents ports that plant services bind. It is metadata for orchestration
# tooling (Docker Compose, Kubernetes) -- not a firewall rule. EXPOSE does not publish
# ports to the host or restrict access. Actual host binding is controlled by the
# ports: key in docker-compose.yml or -p on docker run.
#
# 5020-5039: water/mfg environment Modbus TCP (greenfield-water-mfg)
# 5040-5049: pipeline environment Modbus TCP (pipeline-monitoring)
# 8080, 8081: HMI (water treatment and manufacturing) -- reserved, not yet active
EXPOSE 5020-5039 5040-5049 8080 8081

ENTRYPOINT ["/plant"]
# Default environment for standalone docker run.
# Docker Compose services override this via the command: key per service,
# allowing both plant-water and plant-pipeline to use the same image with
# different --environment paths.
CMD ["--environment", "/design/environments/greenfield-water-mfg", "--log-level", "info"]
