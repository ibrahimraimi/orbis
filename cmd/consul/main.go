package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orbis/internal/api"
	"orbis/internal/health"
	"orbis/internal/registry"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	viper.SetDefault("port", 8500)
	viper.SetDefault("db_path", "consul.db")
	viper.SetDefault("health_interval", "10s")
	viper.SetDefault("health_timeout", "2s")
	viper.AutomaticEnv()

	dbPath := viper.GetString("db_path")
	healthInterval := viper.GetDuration("health_interval")
	healthTimeout := viper.GetDuration("health_timeout")

	store, err := registry.NewBoltStore(dbPath)
	if err != nil {
		logger.Fatal("failed to initialize store", zap.Error(err))
	}
	defer store.Close()

	reg, err := registry.NewRegistry(store)
	if err != nil {
		logger.Fatal("failed to initialize registry", zap.Error(err))
	}

	checker := health.NewChecker(reg, logger, healthInterval, healthTimeout)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go checker.Start(ctx)

	handler := api.NewHandler(reg)
	router := api.NewRouter(handler)

	server := &http.Server{
		Addr:    ":" + os.Getenv("PORT"),
		Handler: router,
	}
	if os.Getenv("PORT") == "" {
		server.Addr = ":8500"
	}

	go func() {
		logger.Info("Starting Consul API", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen and serve failed", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("Shutting down Consul API...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", zap.Error(err))
	}

	logger.Info("Consul API stopped")
}
