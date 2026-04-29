# Setup & Deployment Guide

This guide provides complete instructions for setting up, running, and testing the Orbis system.

## Prerequisites

- **Go**: 1.23 or higher
- **Docker & Docker Compose**: For containerized deployment
- **Make**: For command orchestration

## Local Development

### 1. Build from Source
To build the binaries for both Consul and the Gateway:
```bash
make build
```
Binaries will be placed in the `bin/` directory.

### 2. Run Tests
Ensure everything is working correctly:
```bash
make test
```

### 3. Running Locally (Non-Docker)
Start the Consul registry:
```bash
./bin/consul
```
Start the API Gateway in a separate terminal:
```bash
./bin/gateway
```

## Docker Deployment

The recommended way to run Orbis in production or staging environments is via Docker.

### 1. Build Containers
```bash
make docker-build
```

### 2. Launch Services
```bash
make docker-up
```
This starts:
- **Consul**: Listening on `:8500`
- **API Gateway**: Listening on `:8080`

## Using the System

### 1. Register a Service
Services must register themselves with the Consul registry to be discoverable.
```bash
curl -X POST http://localhost:8500/v1/services/register \
-H "Content-Type: application/json" \
-d '{
  "id": "user-service-1",
  "name": "user-service",
  "address": "localhost",
  "port": 9000,
  "tags": ["version:1.0", "env:prod"]
}'
```

### 2. Health Checking
By default, Consul will check `http://<address>:<port>/health` every 10 seconds. You can override the path using metadata:
```json
"meta": { "health_check_path": "/status" }
```
To use TCP checks instead of HTTP, add the `protocol:tcp` tag.

### 3. Accessing Services via Gateway
The Gateway routes traffic based on the path prefix `/api/<service-name>/`.
If `user-service` is registered and healthy, you can access it via:
```bash
curl http://localhost:8080/api/user-service/profile
```

## Configuration

Configurations are managed via YAML files (`config/gateway.yaml`, `config/consul.yaml`) and can be overridden by environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8500 / 8080 | Listening port |
| `CONSUL_ADDR` | http://localhost:8500 | Gateway's link to Consul |
| `DB_PATH` | consul.db | Path to BoltDB file |
| `RATE_LIMIT_RPS` | 10.0 | Requests per second per IP |
| `RATE_LIMIT_BURST`| 20 | Maximum burst limit for rate limiter |
| `HEALTH_INTERVAL`| 10s | Frequency of health checks |
| `HEALTH_TIMEOUT` | 2s | Timeout for each health check |
| `JWT_SECRET` | supersecretkey | Secret used for HMAC JWT validation |

> **Note on Hot Reloading:** Orbis supports dynamic configuration using Viper. Editing `config/gateway.yaml` while the API Gateway is running will automatically hot-reload gateway routing rules without dropping connections.
