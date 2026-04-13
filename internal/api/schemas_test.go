package api

import (
	"testing"
)

func TestParseColor(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint32
		wantErr bool
	}{
		{"red with hash", "#FF0000", 0xFF0000, false},
		{"green without hash", "00FF00", 0x00FF00, false},
		{"blue", "#0000FF", 0x0000FF, false},
		{"black", "#000000", 0x000000, false},
		{"white", "#FFFFFF", 0xFFFFFF, false},
		{"mixed case", "#aAbBcC", 0xAABBCC, false},
		{"lowercase", "#ff8800", 0xFF8800, false},
		{"short invalid", "XYZ", 0, true},
		{"empty string", "", 0, false}, // "" means "leave current color alone"
		{"invalid hex", "#GG0000", 0, true},
		{"too short with hash", "#FFF", 0, true},
		{"too long", "#FFFFFFF", 0, true},
		{"just hash", "#", 0, true},
		{"5 chars", "#FFFFF", 0, true},
		{"7 chars no hash", "FFFFFFF", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseColor(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("input %q: expected 0x%06X, got 0x%06X", tt.input, tt.want, got)
			}
		})
	}
}

func TestParseColor_BoundaryValues(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  uint32
	}{
		{"min value", "#000000", 0x000000},
		{"max value", "#FFFFFF", 0xFFFFFF},
		{"one", "#000001", 0x000001},
		{"almost max", "#FFFFFE", 0xFFFFFE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseColor(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected 0x%06X, got 0x%06X", tt.want, got)
			}
		})
	}
}
