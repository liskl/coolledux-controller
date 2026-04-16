package mqtt

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/liskl/coolledux-controller/internal/controller"
	ledimage "github.com/liskl/coolledux-controller/internal/image"
	"github.com/liskl/coolledux-controller/internal/models"
)

// CommandHandler processes incoming MQTT command messages and translates them
// into controller calls. It also tracks the current light state for HA.
type CommandHandler struct {
	ctrl   *controller.Controller
	state  *LightState
	mu     sync.Mutex
	logger *slog.Logger
}

// NewCommandHandler creates a CommandHandler with sensible initial state.
func NewCommandHandler(ctrl *controller.Controller, logger *slog.Logger) *CommandHandler {
	return &CommandHandler{
		ctrl: ctrl,
		state: &LightState{
			State:      "ON",
			Brightness: 128,
			Color:      &RGBColor{R: 255, G: 255, B: 255},
		},
		logger: logger,
	}
}

// HandleLightCommand processes a JSON light command (power, brightness, color,
// effect) and forwards the appropriate calls to the controller.
func (h *CommandHandler) HandleLightCommand(payload []byte) error {
	var cmd LightCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return fmt.Errorf("parsing light command: %w", err)
	}

	ctx := context.Background()

	h.mu.Lock()
	defer h.mu.Unlock()

	if cmd.State != "" {
		on := strings.EqualFold(cmd.State, "ON")
		if err := h.ctrl.SetPower(ctx, on); err != nil {
			return fmt.Errorf("setting power: %w", err)
		}
		if on {
			h.state.State = "ON"
		} else {
			h.state.State = "OFF"
		}
	}

	if cmd.Brightness != nil {
		if err := h.ctrl.SetBrightness(ctx, *cmd.Brightness); err != nil {
			return fmt.Errorf("setting brightness: %w", err)
		}
		h.state.Brightness = *cmd.Brightness
	}

	if cmd.Color != nil {
		h.state.Color = cmd.Color
	}

	if cmd.Effect != "" {
		h.state.Effect = cmd.Effect
	}

	return nil
}

// HandleTextCommand processes a JSON text display command.
func (h *CommandHandler) HandleTextCommand(payload []byte) error {
	var cmd TextCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return fmt.Errorf("parsing text command: %w", err)
	}

	mode, err := models.ParseTextShowMode(cmd.Mode)
	if err != nil {
		return fmt.Errorf("parsing text mode: %w", err)
	}

	color, err := parseHexColor(cmd.Color)
	if err != nil {
		return fmt.Errorf("parsing color %q: %w", cmd.Color, err)
	}

	ctx := context.Background()
	if err := h.ctrl.DisplayText(ctx, cmd.Text, mode, cmd.Speed, 0, cmd.FontSize, color, cmd.Font); err != nil {
		return fmt.Errorf("displaying text: %w", err)
	}

	h.logger.Info("text command processed", "text", cmd.Text, "mode", cmd.Mode)
	return nil
}

// HandleImageCommand processes a JSON image display command.
func (h *CommandHandler) HandleImageCommand(payload []byte) error {
	var cmd ImageCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return fmt.Errorf("parsing image command: %w", err)
	}

	imgData, err := base64.StdEncoding.DecodeString(cmd.ImageBase64)
	if err != nil {
		return fmt.Errorf("decoding image base64: %w", err)
	}

	mode, err := models.ParseTextShowMode(cmd.Mode)
	if err != nil {
		return fmt.Errorf("parsing image mode: %w", err)
	}

	fit, err := ledimage.ParseFitMode(cmd.Fit)
	if err != nil {
		return fmt.Errorf("parsing image fit: %w", err)
	}

	ctx := context.Background()
	if err := h.ctrl.DisplayImage(ctx, imgData, mode, 5, 0, fit, cmd.X, cmd.Y, cmd.Width, cmd.Height); err != nil {
		return fmt.Errorf("displaying image: %w", err)
	}

	h.logger.Info("image command processed", "mode", cmd.Mode)
	return nil
}

// HandleGIFCommand processes a JSON GIF display command.
func (h *CommandHandler) HandleGIFCommand(payload []byte) error {
	var cmd GIFCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return fmt.Errorf("parsing gif command: %w", err)
	}

	gifData, err := base64.StdEncoding.DecodeString(cmd.GIFBase64)
	if err != nil {
		return fmt.Errorf("decoding gif base64: %w", err)
	}

	fit, err := ledimage.ParseFitMode(cmd.Fit)
	if err != nil {
		return fmt.Errorf("parsing gif fit: %w", err)
	}

	ctx := context.Background()
	if err := h.ctrl.DisplayGIF(ctx, gifData, cmd.FrameDuration, fit, cmd.X, cmd.Y, cmd.Width, cmd.Height); err != nil {
		return fmt.Errorf("displaying gif: %w", err)
	}

	h.logger.Info("gif command processed", "frame_duration", cmd.FrameDuration)
	return nil
}

// HandleColorModeCommand processes a plain-text select command. Payload is
// either a decimal mode ID ("1", "5", ..., "31") or "off" to revert to the
// static single-color setting (which we approximate by re-sending the last
// known RGB color). Invalid payloads are rejected.
func (h *CommandHandler) HandleColorModeCommand(payload []byte) error {
	ctx := context.Background()
	val := strings.TrimSpace(string(payload))

	if strings.EqualFold(val, "off") {
		h.mu.Lock()
		color := h.state.Color
		h.mu.Unlock()
		if color == nil {
			return nil // nothing to revert to; no-op
		}
		rgb := uint32(color.R)<<16 | uint32(color.G)<<8 | uint32(color.B)
		if err := h.ctrl.SetColor(ctx, rgb); err != nil {
			return fmt.Errorf("restoring single-color: %w", err)
		}
		return nil
	}

	mode, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("parsing mode %q: %w", val, err)
	}
	if err := h.ctrl.SetColorMode(ctx, mode); err != nil {
		return fmt.Errorf("setting color mode: %w", err)
	}
	return nil
}

// HandleColorSpeedCommand processes a plain-text number command. Payload is
// a decimal speed (1-10). Values outside that range are clamped to the
// valid range rather than rejected so a HA slider edge never produces a
// 500 response.
func (h *CommandHandler) HandleColorSpeedCommand(payload []byte) error {
	ctx := context.Background()
	val := strings.TrimSpace(string(payload))

	n, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("parsing speed %q: %w", val, err)
	}
	if n < 1 {
		n = 1
	}
	if n > 10 {
		n = 10
	}
	if err := h.ctrl.SetColorSpeed(ctx, uint8(n)); err != nil {
		return fmt.Errorf("setting color speed: %w", err)
	}
	return nil
}

// HandleShowDeviceIDCommand processes the "ON"/"OFF" payload for the
// show-device-id switch topic.
func (h *CommandHandler) HandleShowDeviceIDCommand(payload []byte) error {
	return h.handleSwitchToggle(payload, h.ctrl.SetShowDeviceID, "show_id")
}

// HandleRemoteCommand processes the "ON"/"OFF" payload for the remote
// switch topic.
func (h *CommandHandler) HandleRemoteCommand(payload []byte) error {
	return h.handleSwitchToggle(payload, h.ctrl.SetRemoteEnabled, "remote")
}

// handleSwitchToggle parses an HA-style "ON"/"OFF" payload and routes it
// to the given controller setter. Any other payload is an error.
func (h *CommandHandler) handleSwitchToggle(payload []byte, setter func(ctx context.Context, on bool) error, label string) error {
	val := strings.TrimSpace(string(payload))
	var on bool
	switch strings.ToUpper(val) {
	case "ON":
		on = true
	case "OFF":
		on = false
	default:
		return fmt.Errorf("%s: expected ON/OFF, got %q", label, val)
	}
	if err := setter(context.Background(), on); err != nil {
		return fmt.Errorf("setting %s: %w", label, err)
	}
	return nil
}

// GetCurrentState returns the current light state as JSON bytes.
func (h *CommandHandler) GetCurrentState() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, _ := json.Marshal(h.state)
	return data
}

// parseHexColor converts a "#RRGGBB" hex string into a uint32 (0x00RRGGBB).
func parseHexColor(s string) (uint32, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return 0, fmt.Errorf("expected 6-character hex color, got %q", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}
