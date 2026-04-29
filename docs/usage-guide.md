# Usage Guide

This guide provides practical examples and workflows for interacting with the Orbis Service Discovery and API Gateway system.

## 1. Consul API (Service Registry)

The Consul API is available on port `:8500` by default.

### Register a Service
Register a new service instance. If the service `id` already exists, it will be updated.

```bash
curl -X POST http://localhost:8500/v1/services/register \
  -H "Content-Type: application/json" \
  -d '{
    "id": "order-service-v1-1",
    "name": "order-service",
    "address": "10.0.0.5",
    "port": 8080,
    "tags": ["version:1.0", "region:us-east"],
    "meta": {
      "health_check_path": "/healthz",
      "max_conns": "100"
    }
  }'
```

### List All Services
Retrieve a list of all currently registered services and their metadata.

```bash
curl http://localhost:8500/v1/services
```

### Lookup Healthy Instances
Find all **healthy** instances of a specific service by name. This is used by the Gateway internally but can be queried manually.

```bash
curl http://localhost:8500/v1/services/order-service
```

### Send a Heartbeat
If a service is configured to expect heartbeats (manual health reporting), use this endpoint to keep the `updated_at` timestamp fresh.

```bash
curl -X PUT http://localhost:8500/v1/services/order-service-v1-1/heartbeat
```

### Deregister a Service
Remove a service instance from the registry.

```bash
curl -X DELETE http://localhost:8500/v1/services/order-service-v1-1/deregister
```

---

## 2. API Gateway

The Gateway is available on port `:8080` by default.

### Routing Logic
The gateway uses path-based routing: `http://gateway:8080/api/<service-name>/<path>`.

**Example:**
To call the `/v1/orders` endpoint on `order-service`:
```bash
curl -H "Authorization: Bearer <your-jwt>" http://localhost:8080/api/order-service/v1/orders
```
The gateway will:
1. Resolve a healthy instance of `order-service` from Consul.
2. Strip `/api/order-service` from the path.
3. Proxy the request to `http://<instance-addr>:<port>/v1/orders`.

### Version-Based Routing
The Gateway supports routing traffic to specific service versions using the `X-API-Version` header.

```bash
curl -H "Authorization: Bearer <your-jwt>" -H "X-API-Version: v2" http://localhost:8080/api/order-service/orders
```
This will exclusively route the request to instances tagged with `version:v2`.

### JWT Authentication
All gateway routes are protected by a JWT authentication middleware.
You must provide a valid `Authorization: Bearer <token>` signed with the configured `JWT_SECRET`.

### Resilience Features

#### Rate Limiting
If you exceed the configured `RATE_LIMIT_RPS` (default: 10 requests/sec), you will receive a `429 Too Many Requests` response. IP tracking uses `X-Real-IP` and `X-Forwarded-For`.

```bash
# Test rate limiting (requires 'hey' or 'ab' tool)
hey -n 100 -c 10 -H "Authorization: Bearer <your-jwt>" http://localhost:8080/api/order-service/data
```

#### Circuit Breaking & Retries
If the upstream service fails repeatedly (5xx errors), the circuit breaker will "open," and the gateway will return `503 Service Unavailable (circuit open)` without attempting to contact the upstream. This prevents overloading a failing service.

Additionally, the gateway proxy automatically handles **transient network errors** (e.g., EOF, connection refused) by retrying the request up to 3 times with exponential backoff.

#### Timeouts
The gateway enforces a 5-second timeout by default. If the upstream takes longer to respond, the gateway will terminate the connection.

---

## 3. Advanced Configuration

### Health Check Protocols
Orbis supports two health check protocols:

1. **HTTP (Default)**: Consul performs a GET request to `health_check_path`. A `200 OK` status indicates health.
2. **TCP**: To use TCP port checks, add `protocol:tcp` to the service tags.
   ```json
   "tags": ["protocol:tcp"]
   ```

### Request Tracking
Every request passing through the gateway is injected with an `X-Request-ID` header if not already present. This ID is logged across all services for distributed tracing.

```bash
curl -I -H "Authorization: Bearer <your-jwt>" http://localhost:8080/api/user-service/profile
# Check response headers for X-Request-ID
```

---

## 4. Observability

### Prometheus Metrics
Orbis natively exposes Prometheus metrics on both the Registry and Gateway.
- **API Gateway Metrics**: `http://localhost:2112/metrics`
- **Consul API Metrics**: `http://localhost:8500/metrics`

Key metrics include `orbis_gateway_requests_total`, `orbis_gateway_latency_seconds`, and `orbis_registry_active_services`.

### OpenTelemetry Tracing
The Gateway automatically injects OpenTelemetry context and spans into requests. By default, it uses the OTLP HTTP exporter. Make sure you have a collector (like Jaeger or Tempo) to receive these traces for deep request lifecycle visibility.
