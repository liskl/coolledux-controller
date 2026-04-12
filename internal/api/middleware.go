package api

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// LoggingMiddleware logs each request's method, path, status code, and duration.
func LoggingMiddleware(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		logger.Info("http request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration", duration.String(),
		)
		return err
	}
}

// RecoveryMiddleware recovers from panics, logs the error, and returns 500.
func RecoveryMiddleware(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered",
					"error", fmt.Sprintf("%v", r),
					"method", c.Method(),
					"path", c.Path(),
				)
				err = c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
					Success: false,
					Error:   "internal server error",
				})
			}
		}()
		return c.Next()
	}
}

// CORSMiddleware applies CORS headers using Fiber's built-in middleware.
func CORSMiddleware(origins []string) fiber.Handler {
	allowOrigins := "*"
	if len(origins) > 0 {
		allowOrigins = ""
		for i, o := range origins {
			if i > 0 {
				allowOrigins += ","
			}
			allowOrigins += o
		}
	}
	return cors.New(cors.Config{
		AllowOrigins: allowOrigins,
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	})
}
