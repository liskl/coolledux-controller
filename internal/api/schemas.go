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
	Enable  bool  `json:"enable"`
	Hour    uint8 `json:"hour"`
	Minute  uint8 `json:"minute"`
	Days    uint8 `json:"days"`
	PowerOn bool  `json:"power_on"`
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
	Font     string `json:"font"` // registered font name; empty = default
}

// CountdownRequest controls the device's countdown timer overlay.
type CountdownRequest struct {
	Action string `json:"action"`           // "show", "set", "start", "stop", "status"
	Hour   uint8  `json:"hour,omitempty"`   // for "show"/"set" (0-23)
	Minute uint8  `json:"minute,omitempty"` // for "show"/"set" (0-59)
	Second uint8  `json:"second,omitempty"` // for "show"/"set" (0-59)
	Color  string `json:"color,omitempty"`  // for "show": "#RRGGBB", default white
}

// StopwatchRequest controls the device's stopwatch overlay.
type StopwatchRequest struct {
	Action string `json:"action"` // "show", "reset", "start", "stop", "status"
	Color  string `json:"color"`  // optional "#RRGGBB" tint for the digits (show only)
}

// ScoreboardRequest controls the device's scoreboard overlay.
// Note: produces no visible output on the 16x96 firmware.
type ScoreboardRequest struct {
	Action  string `json:"action"`             // "set_scores", "set_time", "start", "stop", "status"
	ScoreA  uint16 `json:"score_a,omitempty"`  // for "set_scores"
	ScoreB  uint16 `json:"score_b,omitempty"`  // for "set_scores"
	Hour    uint8  `json:"hour,omitempty"`     // for "set_time"
	Minute  uint8  `json:"minute,omitempty"`   // for "set_time"
	IsTimer bool   `json:"is_timer,omitempty"` // for "set_time"
}

// FontInfoResponse describes a single available font.
type FontInfoResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AdvancePx   int    `json:"advance_px"`
	LinePx      int    `json:"line_px"`
	Monospace   bool   `json:"monospace"`
}

// FontsResponse is the list payload for GET /fonts.
type FontsResponse struct {
	Default string             `json:"default"`
	Fonts   []FontInfoResponse `json:"fonts"`
}

// ImageRequest displays a static image on the LED matrix.
type ImageRequest struct {
	ImageBase64 string `json:"image_base64"`
	Mode        string `json:"mode"`
	Speed       uint8  `json:"speed"`
	StayTime    uint8  `json:"stay_time"`
	Fit         string `json:"fit"` // "letterbox" (default), "stretch", "cover"
	// Placement on the matrix. (0,0) is top-left; width/height of 0 means
	// "fill remaining space from the offset". Defaults cover the whole display.
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// GIFRequest displays an animated GIF on the LED matrix.
type GIFRequest struct {
	GIFBase64     string `json:"gif_base64"`
	FrameDuration uint16 `json:"frame_duration"`
	Fit           string `json:"fit"` // "letterbox" (default), "stretch", "cover"
	// Placement on the matrix. Same semantics as ImageRequest.
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ColorRequest sets the device's global tint color.
type ColorRequest struct {
	Color string `json:"color"` // "#RRGGBB" hex format
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

// parseColor converts a hex color string ("#RRGGBB" or "RRGGBB") to a uint32.
// An empty string returns (0, nil) so callers can treat "unset" as "keep
// whatever color is currently applied on the device".
func parseColor(hex string) (uint32, error) {
	if hex == "" {
		return 0, nil
	}
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
