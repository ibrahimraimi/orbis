package gateway

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"orbis/internal/discovery"
	"strings"

	"go.uber.org/zap"
)

// Proxy handles forwarding requests to upstream services.
type Proxy struct {
	resolver *discovery.Resolver
	logger   *zap.Logger
}

func NewProxy(resolver *discovery.Resolver, logger *zap.Logger) *Proxy {
	return &Proxy{
		resolver: resolver,
		logger:   logger,
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Simple routing logic: /api/{service}/*
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
	
	instance, err := p.resolver.Resolve(serviceName)
	if err != nil {
		p.logger.Error("failed to resolve service", zap.String("service", serviceName), zap.Error(err))
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	targetURL, _ := url.Parse(fmt.Sprintf("http://%s:%d", instance.Address, instance.Port))
	
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	
	// Strip /api/{service} from path if needed, or keep it.
	// For now, let's strip it to be clean.
	r.URL.Path = "/" + strings.Join(parts[1:], "/")
	
	p.logger.Info("proxying request", 
		zap.String("service", serviceName), 
		zap.String("target", targetURL.String()),
		zap.String("path", r.URL.Path))

	proxy.ServeHTTP(w, r)
}
