package api

import (
	"log/slog"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/controller"
	"github.com/liskl/coolledux-controller/internal/registry"
	"github.com/liskl/coolledux-controller/internal/telemetry"
)

// Server is the HTTP server that exposes the REST API.
type Server struct {
	app    *fiber.App
	cfg    *config.Config
	logger *slog.Logger
}

// NewServer creates and configures a Fiber HTTP server with all routes
// registered and middleware applied. The primary controller backs the
// legacy single-device routes; the registry (optional) backs /devices
// and /scan. Pass nil for reg in single-device tests. tel is optional;
// nil falls back to no-op telemetry (useful in tests).
func NewServer(ctrl *controller.Controller, cfg *config.Config, logger *slog.Logger, reg *registry.Registry, tel *telemetry.Provider) *Server {
	app := fiber.New(fiber.Config{
		// Return JSON errors instead of plaintext.
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(SuccessResponse{
				Success: false,
				Error:   err.Error(),
			})
		},
		// 10 MB body limit for images and GIFs.
		BodyLimit: 10 * 1024 * 1024,
	})

	// Apply middleware. otelfiber goes first so request spans enclose
	// recovery/logging output. Gate on whether traces or metrics are
	// actually enabled — gating only on tel.Enabled() would still run
	// otelfiber's per-request wrapper in logs-only configurations,
	// where every request would build span attributes / option chains
	// just to feed noop providers. With both off there's nothing for
	// otelfiber to produce, so skip registration entirely.
	if tel != nil && (tel.TracesEnabled() || tel.MetricsEnabled()) {
		app.Use(otelfiber.Middleware(
			otelfiber.WithTracerProvider(tel.TracerProvider()),
			otelfiber.WithMeterProvider(tel.MeterProvider()),
		))
	}
	app.Use(RecoveryMiddleware(logger))
	app.Use(LoggingMiddleware(logger))
	app.Use(CORSMiddleware(cfg.API.CORSOrigins))

	// Register routes.
	h := NewHandlers(ctrl, logger)
	h.reg = reg
	h.scanTO = cfg.BLE.ScanTimeout
	h.excluded = cfg.BLE.IsExcluded

	// Service-level endpoints.
	app.Get("/health", h.HealthCheck)
	app.Get("/fonts", h.ListFonts)
	app.Get("/devices", h.ListDevices)
	app.Post("/scan", h.ScanDevices)
	app.Get("/openapi.yaml", h.GetOpenAPIYAML)
	app.Get("/openapi.json", h.GetOpenAPIJSON)
	app.Get("/docs", h.GetDocs)

	// Per-device routes. Every device-addressed action is keyed by the
	// registry ID (normalized MAC) in the URL; no "primary device"
	// fallback routes exist.
	app.Get("/device/:id/info", h.GetDeviceInfo)
	app.Post("/device/:id/power", h.SetPower)
	app.Post("/device/:id/brightness", h.SetBrightness)
	app.Post("/device/:id/flip", h.SetFlip)
	app.Post("/device/:id/channel", h.SetChannel)
	app.Post("/device/:id/time", h.SyncTime)
	app.Post("/device/:id/timer", h.SetTimers)
	app.Get("/device/:id/timer", h.GetTimers)
	app.Post("/device/:id/show-id", h.SetShowDeviceID)
	app.Post("/device/:id/remote", h.SetRemote)
	app.Post("/device/:id/password/check", h.CheckPassword)
	app.Post("/device/:id/password/set", h.SetPassword)
	app.Post("/device/:id/text", h.DisplayText)
	app.Post("/device/:id/image", h.DisplayImage)
	app.Post("/device/:id/gif", h.DisplayGIF)
	app.Post("/device/:id/color", h.SetColor)
	app.Post("/device/:id/color/mode", h.SetColorMode)
	app.Post("/device/:id/color/speed", h.SetColorSpeed)
	app.Post("/device/:id/countdown", h.Countdown)
	app.Post("/device/:id/stopwatch", h.Stopwatch)
	app.Post("/device/:id/scoreboard", h.Scoreboard)
	app.Post("/device/:id/debug/timecount", h.CountdownProbeHandler)

	return &Server{
		app:    app,
		cfg:    cfg,
		logger: logger,
	}
}

// Start begins listening on the configured address.
func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "listen", s.cfg.API.Listen)
	return s.app.Listen(s.cfg.API.Listen)
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown() error {
	s.logger.Info("shutting down HTTP server")
	return s.app.Shutdown()
}
