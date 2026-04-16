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
	"github.com/liskl/coolledux-controller/internal/mqtt"
	"github.com/liskl/coolledux-controller/internal/registry"
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

	logger := setupLogger(cfg.Log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build the device registry from config, and optionally augment it
	// with a startup BLE scan so users who haven't pinned MACs still
	// get their panels discovered automatically.
	reg := registry.New()
	devices := cfg.BLE.ResolveDevices()
	for _, dev := range devices {
		entry := registry.BuildEntry(dev, cfg, logger)
		if err := reg.Add(entry); err != nil {
			logger.Warn("skipping duplicate device", "mac", dev.MAC, "error", err)
			continue
		}
	}

	if cfg.BLE.ScanOnStartupEnabled() {
		logger.Info("scanning for CoolLEDUX devices at startup", "timeout", cfg.BLE.ScanTimeout)
		results, err := ble.Scan(ctx, "CoolLEDUX", cfg.BLE.ScanTimeout, logger)
		if err != nil {
			logger.Warn("startup scan failed, continuing with configured devices only", "error", err)
		}
		for _, r := range results {
			if cfg.BLE.IsExcluded(r.MAC) {
				logger.Info("skipping excluded device", "mac", r.MAC, "name", r.Name)
				continue
			}
			dev := config.DeviceConfig{Name: r.Name, MAC: r.MAC}
			if _, exists := reg.Get(dev.ID()); exists {
				// Already registered from static config.
				continue
			}
			entry := registry.BuildEntry(dev, cfg, logger)
			if err := reg.Add(entry); err != nil {
				logger.Warn("skipping scanned device", "mac", r.MAC, "error", err)
				continue
			}
			logger.Info("registered scanned device", "id", entry.ID, "mac", entry.MAC, "name", entry.Name)
		}
	}

	logger.Info("starting coolledux-controller",
		"api_listen", cfg.API.Listen,
		"mqtt_broker", cfg.MQTT.Broker,
		"devices", reg.IDs(),
	)

	// Connect each registered device. One device's BLE failure must not
	// block the others — log and keep going.
	for _, entry := range reg.List() {
		logger.Info("connecting to BLE device", "id", entry.ID, "mac", entry.MAC)
		if err := entry.Controller.Connect(ctx); err != nil {
			logger.Error("BLE connection failed, continuing without device",
				"id", entry.ID, "error", err)
			continue
		}
		logger.Info("BLE connected", "id", entry.ID)
	}

	primary := reg.Primary()
	if primary == nil {
		logger.Warn("no devices registered; running with MQTT and REST disabled for device control")
	}

	// MQTT now drives all registered devices, publishing one discovery
	// payload set per device and subscribing the command topics for each.
	var mqttClient *mqtt.Client
	if primary != nil {
		mqttClient = mqtt.NewClient(&cfg.MQTT, reg, logger)

		logger.Info("connecting to MQTT broker", "broker", cfg.MQTT.Broker)
		if err := mqttClient.Connect(ctx); err != nil {
			logger.Error("MQTT connection failed, continuing without MQTT", "error", err)
		} else {
			logger.Info("MQTT connected, HA discovery published")
		}
	}

	// REST serves legacy routes via the primary device and per-device
	// routes under /device/:id/... backed by the registry.
	var apiServer *api.Server
	if primary != nil {
		apiServer = api.NewServer(primary.Controller, cfg, logger, reg)
		go func() {
			if err := apiServer.Start(); err != nil {
				logger.Error("API server error", "error", err)
				cancel()
			}
		}()
	}

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

	if apiServer != nil {
		if err := apiServer.Shutdown(); err != nil {
			logger.Error("API shutdown error", "error", err)
		}
	}

	if mqttClient != nil {
		mqttClient.Disconnect()
	}

	for _, entry := range reg.List() {
		if err := entry.Controller.Disconnect(ctx); err != nil {
			logger.Error("BLE disconnect error", "id", entry.ID, "error", err)
		}
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
