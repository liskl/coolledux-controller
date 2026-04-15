package api

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/liskl/coolledux-controller/internal/controller"
	ledimage "github.com/liskl/coolledux-controller/internal/image"
	"github.com/liskl/coolledux-controller/internal/models"
	"github.com/liskl/coolledux-controller/internal/protocol"
	"github.com/liskl/coolledux-controller/internal/text"
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
	return c.JSON(info)
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

// SyncTime sets the device clock. If no time fields are provided in the request
// body (or the body is empty), the current system time is used.
func (h *Handlers) SyncTime(c *fiber.Ctx) error {
	var req TimeRequest
	// BodyParser may fail on empty body; that's fine, we default to now.
	_ = c.BodyParser(&req)

	var t time.Time
	if req.Hour == 0 && req.Minute == 0 && req.Second == 0 {
		t = time.Now()
	} else {
		now := time.Now()
		t = time.Date(now.Year(), now.Month(), now.Day(),
			int(req.Hour), int(req.Minute), int(req.Second), 0, now.Location())
	}

	if err := h.ctrl.SyncTime(c.Context(), t); err != nil {
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
			Enable:  item.Enable,
			Hour:    item.Hour,
			Minute:  item.Minute,
			Days:    item.Days,
			PowerOn: item.PowerOn,
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

// GetTimers retrieves the device timer schedule as raw bytes.
func (h *Handlers) GetTimers(c *fiber.Ctx) error {
	resp, err := h.ctrl.GetTimers(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
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

	if err := h.ctrl.DisplayText(c.Context(), req.Text, mode, req.Speed, req.StayTime, req.FontSize, color, req.Font); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// CountdownProbeHandler accepts a 39-byte hex-encoded digit bitmap and uses
// it for all 10 digit positions, then starts a 1-hour countdown so all six
// digit slots display the probe pattern. Used to reverse-engineer the
// firmware byte→pixel mapping for content-type 0x0a digit cells.
//
//	POST /debug/timecount { "probe_hex": "80000000...", "color": "#00FF00" }
func (h *Handlers) CountdownProbeHandler(c *fiber.Ctx) error {
	var req struct {
		ProbeHex string `json:"probe_hex"`
		Color    string `json:"color"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false, Error: err.Error(),
		})
	}
	bitmap := make([]byte, 0, 39)
	if len(req.ProbeHex)%2 != 0 || len(req.ProbeHex) != 78 {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false, Error: "probe_hex must be 78 hex chars (39 bytes)",
		})
	}
	for i := 0; i < len(req.ProbeHex); i += 2 {
		var b byte
		if _, err := fmt.Sscanf(req.ProbeHex[i:i+2], "%02x", &b); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
				Success: false, Error: "invalid hex at byte " + strconv.Itoa(i/2),
			})
		}
		bitmap = append(bitmap, b)
	}
	color := uint32(0xFFFFFF)
	if req.Color != "" {
		parsed, perr := parseColor(req.Color)
		if perr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
				Success: false, Error: perr.Error(),
			})
		}
		color = parsed
	}
	if err := h.ctrl.CountdownProbe(c.Context(), bitmap, color); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// Countdown handles POST /countdown { action, hour?, minute?, second? }.
func (h *Handlers) Countdown(c *fiber.Ctx) error {
	var req CountdownRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false, Error: "invalid request body: " + err.Error(),
		})
	}
	var err error
	switch strings.ToLower(req.Action) {
	case "status":
		err = h.ctrl.CountdownStatus(c.Context())
	case "set":
		err = h.ctrl.CountdownSet(c.Context(), req.Hour, req.Minute, req.Second)
	case "start":
		err = h.ctrl.CountdownStartStop(c.Context(), true)
	case "stop":
		err = h.ctrl.CountdownStartStop(c.Context(), false)
	case "show":
		color := uint32(0xFFFFFF)
		if req.Color != "" {
			parsed, perr := parseColor(req.Color)
			if perr != nil {
				return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
					Success: false, Error: perr.Error(),
				})
			}
			color = parsed
		}
		err = h.ctrl.CountdownDisplay(c.Context(), req.Hour, req.Minute, req.Second, color)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false, Error: `action must be one of "status", "set", "start", "stop", "show"`,
		})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// Stopwatch handles POST /stopwatch { action }.
func (h *Handlers) Stopwatch(c *fiber.Ctx) error {
	var req StopwatchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false, Error: "invalid request body: " + err.Error(),
		})
	}
	var err error
	switch strings.ToLower(req.Action) {
	case "status":
		err = h.ctrl.StopwatchStatus(c.Context())
	case "reset":
		err = h.ctrl.StopwatchReset(c.Context())
	case "start":
		err = h.ctrl.StopwatchStartStop(c.Context(), true)
	case "stop":
		err = h.ctrl.StopwatchStartStop(c.Context(), false)
	case "show":
		color := uint32(0xFFFFFF)
		if req.Color != "" {
			parsed, perr := parseColor(req.Color)
			if perr != nil {
				return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
					Success: false, Error: perr.Error(),
				})
			}
			color = parsed
		}
		err = h.ctrl.StopwatchDisplay(c.Context(), color)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false, Error: `action must be one of "status", "reset", "start", "stop", "show"`,
		})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// Scoreboard handles POST /scoreboard { action, ... }.
// Note: scoreboard packets are ACKed but not visible on the 16x96 firmware.
func (h *Handlers) Scoreboard(c *fiber.Ctx) error {
	var req ScoreboardRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false, Error: "invalid request body: " + err.Error(),
		})
	}
	var err error
	switch strings.ToLower(req.Action) {
	case "status":
		err = h.ctrl.ScoreboardStatus(c.Context())
	case "set_scores":
		err = h.ctrl.ScoreboardSetScores(c.Context(), req.ScoreA, req.ScoreB, req.TotalA, req.TotalB)
	case "set_time":
		err = h.ctrl.ScoreboardSetTime(c.Context(), req.Hour, req.Minute, req.IsTimer)
	case "start":
		err = h.ctrl.ScoreboardStartStop(c.Context(), true)
	case "stop":
		err = h.ctrl.ScoreboardStartStop(c.Context(), false)
	case "show":
		color := uint32(0xFFFFFF)
		if req.Color != "" {
			parsed, perr := parseColor(req.Color)
			if perr != nil {
				return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
					Success: false, Error: perr.Error(),
				})
			}
			color = parsed
		}
		err = h.ctrl.ScoreboardDisplay(c.Context(), color)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false, Error: `action must be one of "status", "set_scores", "set_time", "start", "stop", "show"`,
		})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}

// ListFonts returns all font faces available for /display/text.
func (h *Handlers) ListFonts(c *fiber.Ctx) error {
	available := text.AvailableFonts()
	out := FontsResponse{
		Default: text.DefaultFontName,
		Fonts:   make([]FontInfoResponse, 0, len(available)),
	}
	for _, f := range available {
		out.Fonts = append(out.Fonts, FontInfoResponse{
			Name:        f.Name,
			Description: f.Description,
			AdvancePx:   f.Width,
			LinePx:      f.Height,
			Monospace:   f.Monospace,
		})
	}
	return c.JSON(out)
}

// SetColor sets the device's global tint color.
func (h *Handlers) SetColor(c *fiber.Ctx) error {
	var req ColorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
	}

	color, err := parseColor(req.Color)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	if err := h.ctrl.SetColor(c.Context(), color); err != nil {
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

	fit, err := ledimage.ParseFitMode(req.Fit)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	if err := h.ctrl.DisplayImage(c.Context(), imgData, mode, req.Speed, req.StayTime, fit, req.X, req.Y, req.Width, req.Height); err != nil {
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

	if req.Raw {
		if err := h.ctrl.DisplayRawGIF(c.Context(), gifData, req.X, req.Y, req.Width, req.Height); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
				Success: false,
				Error:   err.Error(),
			})
		}
		return c.JSON(SuccessResponse{Success: true})
	}

	fit, err := ledimage.ParseFitMode(req.Fit)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	if err := h.ctrl.DisplayGIF(c.Context(), gifData, req.FrameDuration, fit, req.X, req.Y, req.Width, req.Height); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(SuccessResponse{
			Success: false,
			Error:   err.Error(),
		})
	}
	return c.JSON(SuccessResponse{Success: true})
}
