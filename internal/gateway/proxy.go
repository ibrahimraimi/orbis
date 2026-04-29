package gateway

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"orbis/internal/discovery"
	"strings"
	"time"

	"go.uber.org/zap"
)

type RetryTransport struct {
	next       http.RoundTripper
	maxRetries int
	logger     *zap.Logger
}

func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	
	backoff := 100 * time.Millisecond

	for i := 0; i <= t.maxRetries; i++ {
		if i > 0 {
			t.logger.Warn("Retrying upstream request", zap.Int("attempt", i), zap.String("url", req.URL.String()))
			time.Sleep(backoff)
			backoff *= 2
		}
		
		resp, err = t.next.RoundTrip(req)
		// Only retry on network errors (transient upstream failures)
		if err == nil {
			return resp, nil
		}
	}
	return resp, err
}

// Proxy handles forwarding requests to upstream services.
type Proxy struct {
	resolver  *discovery.Resolver
	logger    *zap.Logger
	transport http.RoundTripper
}

func NewProxy(resolver *discovery.Resolver, logger *zap.Logger) *Proxy {
	return &Proxy{
		resolver: resolver,
		logger:   logger,
		transport: &RetryTransport{
			next:       http.DefaultTransport,
			maxRetries: 3,
			logger:     logger,
		},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	trimmed := strings.TrimPrefix(path, "/api/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "missing service name", http.StatusBadRequest)
		return
	}

	serviceName := parts[0]
	apiVersion := r.Header.Get("X-API-Version")
	
	instance, err := p.resolver.Resolve(serviceName, apiVersion)
	if err != nil {
		p.logger.Error("failed to resolve service", zap.String("service", serviceName), zap.String("version", apiVersion), zap.Error(err))
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	targetURL, _ := url.Parse(fmt.Sprintf("http://%s:%d", instance.Address, instance.Port))
	
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = p.transport
	
	r.URL.Path = "/" + strings.Join(parts[1:], "/")
	
	p.logger.Info("proxying request", 
		zap.String("service", serviceName), 
		zap.String("version", apiVersion),
		zap.String("target", targetURL.String()),
		zap.String("path", r.URL.Path))

	proxy.ServeHTTP(w, r)
}
