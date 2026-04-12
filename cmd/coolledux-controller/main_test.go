package main

import (
	"testing"

	"github.com/liskl/coolledux-controller/internal/config"
)

func TestSetupLogger_Debug_JSON(t *testing.T) {
	logger := setupLogger(config.LogConfig{Level: "debug", Format: "json"})
	if logger == nil {
		t.Fatal("setupLogger returned nil for debug+json")
	}
}

func TestSetupLogger_Warn_Text(t *testing.T) {
	logger := setupLogger(config.LogConfig{Level: "warn", Format: "text"})
	if logger == nil {
		t.Fatal("setupLogger returned nil for warn+text")
	}
}

func TestSetupLogger_Error_Text(t *testing.T) {
	logger := setupLogger(config.LogConfig{Level: "error", Format: "text"})
	if logger == nil {
		t.Fatal("setupLogger returned nil for error+text")
	}
}

func TestSetupLogger_Info_JSON(t *testing.T) {
	logger := setupLogger(config.LogConfig{Level: "info", Format: "json"})
	if logger == nil {
		t.Fatal("setupLogger returned nil for info+json (default case)")
	}
}

func TestSetupLogger_Unknown_Defaults(t *testing.T) {
	// Unknown level should default to info, unknown format should default to JSON.
	logger := setupLogger(config.LogConfig{Level: "unknown", Format: "unknown"})
	if logger == nil {
		t.Fatal("setupLogger returned nil for unknown+unknown")
	}
}

func TestSetupLogger_EmptyConfig(t *testing.T) {
	// Empty strings should also hit defaults (info level, json format).
	logger := setupLogger(config.LogConfig{})
	if logger == nil {
		t.Fatal("setupLogger returned nil for empty config")
	}
}
