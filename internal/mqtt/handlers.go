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

	ctx := context.Background()
	if err := h.ctrl.DisplayImage(ctx, imgData, mode, 5, 0); err != nil {
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

	ctx := context.Background()
	if err := h.ctrl.DisplayGIF(ctx, gifData, cmd.FrameDuration); err != nil {
		return fmt.Errorf("displaying gif: %w", err)
	}

	h.logger.Info("gif command processed", "frame_duration", cmd.FrameDuration)
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
