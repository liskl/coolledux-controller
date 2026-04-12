package api

import (
	"fmt"
	"strconv"
	"strings"
)

// --- Request types ---

// PowerRequest controls the display power state.
type PowerRequest struct {
	State string `json:"state"` // "on" or "off"
}

// BrightnessRequest sets the display brightness.
type BrightnessRequest struct {
	Brightness uint8 `json:"brightness"`
}

// FlipRequest sets the display flip/mirror mode.
type FlipRequest struct {
	Mode string `json:"mode"` // "none", "horizontal", "vertical", "both"
}

// ChannelRequest switches the displayed program/channel slot.
type ChannelRequest struct {
	Channel uint8 `json:"channel"`
}

// TimeRequest sets the device clock.
type TimeRequest struct {
	Hour   uint8 `json:"hour"`
	Minute uint8 `json:"minute"`
	Second uint8 `json:"second"`
}

// TimerItemRequest represents a single timer schedule entry.
type TimerItemRequest struct {
	Hour   uint8 `json:"hour"`
	Minute uint8 `json:"minute"`
	On     bool  `json:"on"`
	Days   uint8 `json:"days"`
}

// TimerRequest configures the device timer schedule.
type TimerRequest struct {
	Items []TimerItemRequest `json:"items"`
}

// TextRequest displays text on the LED matrix.
type TextRequest struct {
	Text     string `json:"text"`
	Mode     string `json:"mode"`      // "scroll_left", "static", etc.
	Speed    uint8  `json:"speed"`
	Color    string `json:"color"`     // "#FF0000" hex format
	FontSize int    `json:"font_size"`
	StayTime uint8  `json:"stay_time"`
}

// ImageRequest displays a static image on the LED matrix.
type ImageRequest struct {
	ImageBase64 string `json:"image_base64"`
	Mode        string `json:"mode"`
	Speed       uint8  `json:"speed"`
	StayTime    uint8  `json:"stay_time"`
}

// GIFRequest displays an animated GIF on the LED matrix.
type GIFRequest struct {
	GIFBase64     string `json:"gif_base64"`
	FrameDuration uint16 `json:"frame_duration"`
}

// --- Response types ---

// SuccessResponse is the standard response for mutating operations.
type SuccessResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// HealthResponse reports the service health status.
type HealthResponse struct {
	Status        string `json:"status"`
	BLEConnected  bool   `json:"ble_connected"`
	MQTTConnected bool   `json:"mqtt_connected"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// DeviceInfoResponse holds identity and state information returned by the device.
type DeviceInfoResponse struct {
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmware_version"`
	HardwareVersion string `json:"hardware_version"`
	Columns         int    `json:"columns"`
	Rows            int    `json:"rows"`
	Power           bool   `json:"power"`
	Brightness      uint8  `json:"brightness"`
	FlipMode        int    `json:"flip_mode"`
}

// parseColor converts a hex color string ("#RRGGBB" or "RRGGBB") to a uint32.
func parseColor(hex string) (uint32, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, fmt.Errorf("invalid color format %q: expected 6 hex digits", hex)
	}
	val, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid color %q: %w", hex, err)
	}
	return uint32(val), nil
}
