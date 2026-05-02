package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orbis/internal/discovery"
	"orbis/internal/gateway"
	"orbis/internal/observability"

	"github.com/fsnotify/fsnotify"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// OpenTelemetry init
	tp, err := observability.InitTracer(context.Background(), "orbis-gateway", logger)
	if err != nil {
		logger.Fatal("failed to initialize tracer", zap.Error(err))
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			logger.Error("failed to shutdown tracer provider", zap.Error(err))
		}
	}()

	viper.SetDefault("port", 8080)
	viper.SetDefault("consul_addr", "http://localhost:8500")
	viper.SetDefault("rate_limit_rps", 10.0)
	viper.SetDefault("rate_limit_burst", 20)
	viper.SetDefault("jwt_secret", "supersecretkey")
	viper.AutomaticEnv()

	viper.SetConfigName("gateway")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	if err := viper.ReadInConfig(); err != nil {
		logger.Warn("could not read config file, relying on defaults/env", zap.Error(err))
	}

	consulAddr := viper.GetString("consul_addr")
	rps := viper.GetFloat64("rate_limit_rps")
	burst := viper.GetInt("rate_limit_burst")

	resolver := discovery.NewResolver(consulAddr)
	resolver.Watch(context.Background(), logger)
	proxy := gateway.NewProxy(resolver, logger)

	// Apply initial routes if configured
	if routes := viper.GetStringMapString("routes"); len(routes) > 0 {
		proxy.ReloadRoutes(routes)
	}

	viper.OnConfigChange(func(e fsnotify.Event) {
		logger.Info("Config file changed", zap.String("file", e.Name))
		if routes := viper.GetStringMapString("routes"); len(routes) > 0 {
			proxy.ReloadRoutes(routes)
		}
	})
	viper.WatchConfig()

	// Metrics endpoint
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		logger.Info("Starting Gateway metrics server", zap.String("addr", ":2112"))
		if err := http.ListenAndServe(":2112", mux); err != nil {
			logger.Error("metrics server failed", zap.Error(err))
		}
	}()

	handler := gateway.RequestID(
		gateway.Logger(logger)(
			gateway.MetricsAndTracing("orbis-gateway")(
				middleware.Compress(5)(
					gateway.RateLimiter(rps, burst)(
						gateway.APIKeyAuth(resolver)(
							gateway.CircuitBreaker(
								gateway.Timeout(5 * time.Second)(
									proxy,
								),
							),
						),
					),
				),
			),
		),
	)

	server := &http.Server{
		Addr:    ":" + os.Getenv("PORT"),
		Handler: handler,
	}
	if os.Getenv("PORT") == "" {
		server.Addr = fmt.Sprintf(":%d", viper.GetInt("port"))
	}

	go func() {
		logger.Info("Starting API Gateway", zap.String("addr", server.Addr), zap.String("consul", consulAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen and serve failed", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("Shutting down API Gateway...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", zap.Error(err))
	}

	logger.Info("API Gateway stopped")
}
