package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orbis/internal/api"
	"orbis/internal/health"
	"orbis/internal/observability"
	"orbis/internal/registry"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// OpenTelemetry init
	tp, err := observability.InitTracer(context.Background(), "orbis-consul", logger)
	if err != nil {
		logger.Fatal("failed to initialize tracer", zap.Error(err))
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			logger.Error("failed to shutdown tracer provider", zap.Error(err))
		}
	}()

	viper.SetDefault("port", 8500)
	viper.SetDefault("db_path", "consul.db")
	viper.SetDefault("health_interval", "10s")
	viper.SetDefault("health_timeout", "2s")
	viper.AutomaticEnv()

	viper.SetConfigName("consul")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	if err := viper.ReadInConfig(); err != nil {
		logger.Warn("could not read config file, relying on defaults/env", zap.Error(err))
	}

	viper.OnConfigChange(func(e fsnotify.Event) {
		logger.Info("Config file changed", zap.String("file", e.Name))
	})
	viper.WatchConfig()

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
		server.Addr = fmt.Sprintf(":%d", viper.GetInt("port"))
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
