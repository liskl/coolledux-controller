package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/liskl/coolledux-controller/internal/api"
	"github.com/liskl/coolledux-controller/internal/ble"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/controller"
	"github.com/liskl/coolledux-controller/internal/mqtt"
)

func main() {
	configPath := flag.String("config", "", "path to config.yaml")
	flag.Parse()

	// Load configuration (YAML + env vars + defaults).
	path := *configPath
	if path == "" {
		path = os.Getenv("COOLLEDUX_CONFIG_FILE")
	}
	cfg, err := config.Load(path)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Set up structured logging.
	logger := setupLogger(cfg.Log)
	logger.Info("starting coolledux-controller",
		"device_mac", cfg.BLE.DeviceMAC,
		"device_id", cfg.DeviceID(),
		"api_listen", cfg.API.Listen,
		"mqtt_broker", cfg.MQTT.Broker,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// BLE layer.
	bleClient := ble.NewClient(logger)
	transport := ble.NewTransport(bleClient, logger)
	ctrl := controller.New(bleClient, transport, cfg, logger)

	// Connect to BLE device.
	logger.Info("connecting to BLE device", "mac", cfg.BLE.DeviceMAC)
	if err := ctrl.Connect(ctx); err != nil {
		logger.Error("BLE connection failed, continuing without device", "error", err)
		// Continue anyway so the API and MQTT are still available.
		// The controller will report disconnected state.
	} else {
		logger.Info("BLE connected")
	}

	// MQTT layer.
	mqttHandler := mqtt.NewCommandHandler(ctrl, logger)
	mqttClient := mqtt.NewClient(&cfg.MQTT, cfg.DeviceID(), mqttHandler, logger)

	logger.Info("connecting to MQTT broker", "broker", cfg.MQTT.Broker)
	if err := mqttClient.Connect(ctx); err != nil {
		logger.Error("MQTT connection failed, continuing without MQTT", "error", err)
	} else {
		logger.Info("MQTT connected, HA discovery published")
	}

	// REST API.
	apiServer := api.NewServer(ctrl, cfg, logger)
	go func() {
		if err := apiServer.Start(); err != nil {
			logger.Error("API server error", "error", err)
			cancel()
		}
	}()

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("received shutdown signal", "signal", sig)
	case <-ctx.Done():
	}

	// Graceful shutdown.
	logger.Info("shutting down")

	if err := apiServer.Shutdown(); err != nil {
		logger.Error("API shutdown error", "error", err)
	}

	mqttClient.Disconnect()

	if err := ctrl.Disconnect(ctx); err != nil {
		logger.Error("BLE disconnect error", "error", err)
	}

	logger.Info("shutdown complete")
}

func setupLogger(cfg config.LogConfig) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
