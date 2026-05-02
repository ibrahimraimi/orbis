# Orbis

Orbis is a service discovery and API gateway system. It provides a persistent service registry with integrated health checking and an intelligent gateway with resilience and observability.

## Core Features

- **Service Registry**: Pluggable storage backends (BoltDB & Redis).
- **Health Checking**: Automatic HTTP and TCP health monitoring of registered instances.
- **Event-Driven Architecture**: Zero-latency gateway synchronization via a native Server-Sent Events (SSE) Pub/Sub stream (`/v1/watch`).
- **Dynamic Gateway**: Intelligent proxying with path rewriting, load balancing, and version-based routing (`X-API-Version`).
- **Consumer & API Key Management**: Securely generates, hashes, and validates API keys (`X-API-Key`) for distinct B2B client access control.
- **Resilience Stack**:
  - **Circuit Breaking**: Prevents cascading failures.
  - **Rate Limiting**: Token-bucket based IP limiting.
  - **Timeouts**: Enforced request deadlines.
- **Observability**: 
  - Structured logging (`zap`) and unique Request ID injection.
  - **Prometheus Metrics & OpenTelemetry**: Deep request tracing and performance charting, enriched with granular `consumer_id` tags.

## Project Structure

```
.
├── cmd/
│   ├── consul/       # Registry service entry point
│   ├── gateway/      # API Gateway entry point
│   └── orbisctl/     # Developer CLI tool
├── internal/
│   ├── api/          # Registry REST API handlers
│   ├── discovery/    # Client-side discovery resolver
│   ├── gateway/      # Proxy and middleware logic
│   ├── health/       # Background health checker
│   ├── models/       # Core data structures
│   └── registry/     # Registry and persistence layer
├── config/           # Default configurations (YAML)
├── docker/           # Service Dockerfiles
└── Makefile          # Build and test orchestration
```

## Developer CLI

Orbis includes `orbisctl`, a native command-line interface for interacting with the registry.

```bash
go build -o bin/orbisctl ./cmd/orbisctl

# View all registered services
./bin/orbisctl services list

# Inspect a specific service
./bin/orbisctl services get <service-id>
```

## Quick Start

The fastest way to run Orbis is using Docker Compose:

```bash
make docker-up
```

For detailed installation, configuration, and usage instructions, see:
- [docs/setup-guide.md](docs/setup-guide.md)
- [docs/usage-guide.md](docs/usage-guide.md)
