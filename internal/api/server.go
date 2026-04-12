package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/liskl/coolledux-controller/internal/config"
	"github.com/liskl/coolledux-controller/internal/controller"
)

// Server is the HTTP server that exposes the REST API.
type Server struct {
	app    *fiber.App
	cfg    *config.Config
	logger *slog.Logger
}

// NewServer creates and configures a Fiber HTTP server with all routes
// registered and middleware applied.
func NewServer(ctrl *controller.Controller, cfg *config.Config, logger *slog.Logger) *Server {
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

	// Health & info
	app.Get("/health", h.HealthCheck)
	app.Get("/device/info", h.GetDeviceInfo)

	// Device control
	app.Post("/device/power", h.SetPower)
	app.Post("/device/brightness", h.SetBrightness)
	app.Post("/device/flip", h.SetFlip)
	app.Post("/device/channel", h.SetChannel)
	app.Post("/device/time", h.SyncTime)
	app.Post("/device/timer", h.SetTimers)
	app.Get("/device/timer", h.GetTimers)
	app.Post("/device/reset", h.ResetDevice)

	// Display
	app.Post("/display/text", h.DisplayText)
	app.Post("/display/image", h.DisplayImage)
	app.Post("/display/gif", h.DisplayGIF)

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
