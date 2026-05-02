package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	GatewayRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orbis_gateway_requests_total",
			Help: "Total number of HTTP requests routed through the gateway",
		},
		[]string{"method", "path", "status", "consumer_id"},
	)

	GatewayRequestLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "orbis_gateway_latency_seconds",
			Help:    "Latency of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "consumer_id"},
	)

	RegistryActiveServices = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "orbis_registry_active_services",
			Help: "Current number of registered and active services by name",
		},
		[]string{"service"},
	)
)
