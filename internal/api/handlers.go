package api

import (
	"encoding/base64"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/liskl/coolledux-controller/internal/controller"
	"github.com/liskl/coolledux-controller/internal/models"
	"github.com/liskl/coolledux-controller/internal/protocol"
)

// Handlers holds the dependencies for all HTTP route handlers.
type Handlers struct {
	ctrl      *controller.Controller
	startTime time.Time
	logger    *slog.Logger
}

// NewHandlers creates a Handlers instance wired to the given controller.
func NewHandlers(ctrl *controller.Controller, logger *slog.Logger) *Handlers {
	return &Handlers{
		ctrl:      ctrl,
		startTime: time.Now(),
		logger:    logger,
	}
}

// HealthCheck returns service health information including uptime and
// connection status.
func (h *Handlers) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(HealthResponse{
		Status:        "ok",
		BLEConnected:  h.ctrl.IsConnected(),
		MQTTConnected: false, // MQTT integration wired separately
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
	})
}

// GetDeviceInfo queries the device over BLE and returns its identity and state.
func (h *Handlers) GetDeviceInfo(c *fiber.Ctx) error {
	info, err := h.ctrl.GetDeviceInfo(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(DeviceInfoResponse{
		Model:           info.Model,
		FirmwareVersion: info.FirmwareVersion,
		HardwareVersion: info.HardwareVersion,
		Columns:         info.Columns,
		Rows:            info.Rows,
		Power:           info.Power,
		Brightness:      info.Brightness,
		FlipMode:        int(info.FlipMode),
	})
}

// SetPower turns the display on or off.
func (h *Handlers) SetPower(c *fiber.Ctx) error {
	var req PowerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
	}

	state := strings.ToLower(req.State)
	var on bool
	switch state {
	case "on":
		on = true
	case "off":
		on = false
	default:
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "state must be \"on\" or \"off\"",
		})
	}

	if err := h.ctrl.SetPower(c.Context(), on); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// SetBrightness sets the display brightness level.
func (h *Handlers) SetBrightness(c *fiber.Ctx) error {
	var req BrightnessRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
	}

	if err := h.ctrl.SetBrightness(c.Context(), req.Brightness); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// SetFlip sets the display orientation flip mode.
func (h *Handlers) SetFlip(c *fiber.Ctx) error {
	var req FlipRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
	}

	mode, err := models.ParseFlipMode(req.Mode)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	if err := h.ctrl.SetFlip(c.Context(), mode); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// SetChannel switches the displayed program/channel slot.
func (h *Handlers) SetChannel(c *fiber.Ctx) error {
	var req ChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
	}

	if err := h.ctrl.SetChannel(c.Context(), req.Channel); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// SyncTime sets the device clock.
func (h *Handlers) SyncTime(c *fiber.Ctx) error {
	var req TimeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
	}

	if err := h.ctrl.SyncTime(c.Context(), req.Hour, req.Minute, req.Second); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// SetTimers configures the device timer schedule.
func (h *Handlers) SetTimers(c *fiber.Ctx) error {
	var req TimerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
	}

	items := make([]protocol.TimerItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = protocol.TimerItem{
			Hour:   item.Hour,
			Minute: item.Minute,
			On:     item.On,
			Days:   item.Days,
		}
	}

	if err := h.ctrl.SetTimers(c.Context(), items); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// ResetDevice sends a factory reset command to the device.
func (h *Handlers) ResetDevice(c *fiber.Ctx) error {
	if err := h.ctrl.ResetDevice(c.Context()); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// DisplayText renders and displays text on the LED matrix.
func (h *Handlers) DisplayText(c *fiber.Ctx) error {
	var req TextRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
	}

	mode, err := models.ParseTextShowMode(req.Mode)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	color, err := parseColor(req.Color)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	if err := h.ctrl.DisplayText(c.Context(), req.Text, mode, req.Speed, req.StayTime, req.FontSize, color); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// DisplayImage decodes a base64-encoded image and displays it on the LED matrix.
func (h *Handlers) DisplayImage(c *fiber.Ctx) error {
	var req ImageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
	}

	imgData, err := base64.StdEncoding.DecodeString(req.ImageBase64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid base64 image data: " + err.Error(),
		})
	}

	mode, err := models.ParseTextShowMode(req.Mode)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	if err := h.ctrl.DisplayImage(c.Context(), imgData, mode, req.Speed, req.StayTime); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// DisplayGIF decodes a base64-encoded GIF and displays it as an animation.
func (h *Handlers) DisplayGIF(c *fiber.Ctx) error {
	var req GIFRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
	}

	gifData, err := base64.StdEncoding.DecodeString(req.GIFBase64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid base64 gif data: " + err.Error(),
		})
	}

	if err := h.ctrl.DisplayGIF(c.Context(), gifData, req.FrameDuration); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}
