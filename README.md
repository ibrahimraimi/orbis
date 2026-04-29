# Orbis Service Discovery & API Gateway

Orbis is a production-grade service discovery and API gateway system built in Go. It provides a persistent service registry with integrated health checking and an intelligent gateway with resilience and observability features.

## Core Features

- **Service Registry**: Persistent storage of service metadata using `bbolt`.
- **Health Checking**: Automatic HTTP and TCP health monitoring of registered instances.
- **Dynamic Routing**: Automatic discovery and routing to healthy upstream services.
- **Load Balancing**: Round-Robin selection of healthy service instances.
- **Resilience Stack**:
  - **Circuit Breaking**: Prevents cascading failures.
  - **Rate Limiting**: Token-bucket based IP limiting.
  - **Timeouts**: Enforced request deadlines.
- **Observability**: Structured logging (`zap`) and unique Request ID injection.

## Project Structure

```
.
├── cmd/
│   ├── consul/       # Registry service entry point
│   └── gateway/      # API Gateway entry point
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

## Quick Start

The fastest way to run Orbis is using Docker Compose:

```bash
make docker-up
```

For detailed installation, configuration, and usage instructions, see:
- [docs/setup-guide.md](docs/setup-guide.md)
- [docs/usage-guide.md](docs/usage-guide.md)
