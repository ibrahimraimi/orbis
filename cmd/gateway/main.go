package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orbis/internal/discovery"
	"orbis/internal/gateway"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	viper.SetDefault("port", 8080)
	viper.SetDefault("consul_addr", "http://localhost:8500")
	viper.SetDefault("rate_limit_rps", 10.0)
	viper.SetDefault("rate_limit_burst", 20)
	viper.AutomaticEnv()

	consulAddr := viper.GetString("consul_addr")
	rps := viper.GetFloat64("rate_limit_rps")
	burst := viper.GetInt("rate_limit_burst")

	resolver := discovery.NewResolver(consulAddr)

	proxy := gateway.NewProxy(resolver, logger)

	handler := gateway.RequestID(
		gateway.Logger(logger)(
			gateway.RateLimiter(rps, burst)(
				gateway.CircuitBreaker(
					gateway.Timeout(5 * time.Second)(
						proxy,
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
		server.Addr = ":8080"
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
