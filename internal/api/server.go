package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/controller"
	"github.com/liskl/coolledux-controller/internal/registry"
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
// and /scan. Pass nil for reg in single-device tests.
func NewServer(ctrl *controller.Controller, cfg *config.Config, logger *slog.Logger, reg *registry.Registry) *Server {
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

	// Apply middleware.
	app.Use(RecoveryMiddleware(logger))
	app.Use(LoggingMiddleware(logger))
	app.Use(CORSMiddleware(cfg.API.CORSOrigins))

	// Register routes.
	h := NewHandlers(ctrl, logger)
	h.reg = reg
	h.scanTO = cfg.BLE.ScanTimeout

	// Health & info
	app.Get("/health", h.HealthCheck)
	app.Get("/device/info", h.GetDeviceInfo)

	// Multi-device registry endpoints
	app.Get("/devices", h.ListDevices)
	app.Post("/scan", h.ScanDevices)

	// Device control
	app.Post("/device/power", h.SetPower)
	app.Post("/device/brightness", h.SetBrightness)
	app.Post("/device/flip", h.SetFlip)
	app.Post("/device/channel", h.SetChannel)
	app.Post("/device/time", h.SyncTime)
	app.Post("/device/timer", h.SetTimers)
	app.Get("/device/timer", h.GetTimers)
	app.Post("/device/reset", h.ResetDevice)
	app.Post("/device/show-id", h.SetShowDeviceID)
	app.Post("/device/remote", h.SetRemote)

	// Display
	app.Post("/display/text", h.DisplayText)
	app.Post("/display/image", h.DisplayImage)
	app.Post("/display/gif", h.DisplayGIF)
	app.Post("/display/color", h.SetColor)
	app.Post("/display/color/mode", h.SetColorMode)
	app.Post("/display/color/speed", h.SetColorSpeed)
	app.Get("/fonts", h.ListFonts)

	// Overlays
	app.Post("/countdown", h.Countdown)
	app.Post("/stopwatch", h.Stopwatch)
	app.Post("/scoreboard", h.Scoreboard)
	app.Post("/debug/timecount", h.CountdownProbeHandler)

	// Per-device routes. Every legacy device route above has a twin here
	// under /device/:id/... that targets the requested MAC instead of the
	// primary. The resolve helper in Handlers reads :id and dispatches;
	// legacy routes reuse the same handler with no :id set so they still
	// hit the primary. Once these are hardware-verified, a follow-up
	// bead removes the legacy routes.
	app.Get("/device/:id/info", h.GetDeviceInfo)
	app.Post("/device/:id/power", h.SetPower)
	app.Post("/device/:id/brightness", h.SetBrightness)
	app.Post("/device/:id/flip", h.SetFlip)
	app.Post("/device/:id/channel", h.SetChannel)
	app.Post("/device/:id/time", h.SyncTime)
	app.Post("/device/:id/timer", h.SetTimers)
	app.Get("/device/:id/timer", h.GetTimers)
	app.Post("/device/:id/reset", h.ResetDevice)
	app.Post("/device/:id/show-id", h.SetShowDeviceID)
	app.Post("/device/:id/remote", h.SetRemote)
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
