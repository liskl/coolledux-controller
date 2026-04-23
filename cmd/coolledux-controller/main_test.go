package main

import (
	"testing"

	"github.com/liskl/coolledux-controller/internal/config"
)

func TestStdoutHandler_Debug_JSON(t *testing.T) {
	if newStdoutHandler(config.LogConfig{Level: "debug", Format: "json"}) == nil {
		t.Fatal("newStdoutHandler returned nil for debug+json")
	}
}

func TestStdoutHandler_Warn_Text(t *testing.T) {
	if newStdoutHandler(config.LogConfig{Level: "warn", Format: "text"}) == nil {
		t.Fatal("newStdoutHandler returned nil for warn+text")
	}
}

func TestStdoutHandler_Error_Text(t *testing.T) {
	if newStdoutHandler(config.LogConfig{Level: "error", Format: "text"}) == nil {
		t.Fatal("newStdoutHandler returned nil for error+text")
	}
}

func TestStdoutHandler_Info_JSON(t *testing.T) {
	if newStdoutHandler(config.LogConfig{Level: "info", Format: "json"}) == nil {
		t.Fatal("newStdoutHandler returned nil for info+json (default case)")
	}
}

func TestStdoutHandler_Unknown_Defaults(t *testing.T) {
	if newStdoutHandler(config.LogConfig{Level: "unknown", Format: "unknown"}) == nil {
		t.Fatal("newStdoutHandler returned nil for unknown+unknown")
	}
}

func TestStdoutHandler_EmptyConfig(t *testing.T) {
	if newStdoutHandler(config.LogConfig{}) == nil {
		t.Fatal("newStdoutHandler returned nil for empty config")
	}
}
