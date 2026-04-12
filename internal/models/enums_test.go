package models

import (
	"fmt"
	"strings"
	"testing"
)

func TestTextShowMode_String(t *testing.T) {
	tests := []struct {
		mode TextShowMode
		want string
	}{
		{TextShowModeStatic, "static"},
		{TextShowModeScrollLeft, "scroll_left"},
		{TextShowModeScrollRight, "scroll_right"},
		{TextShowModeScrollUp, "scroll_up"},
		{TextShowModeScrollDown, "scroll_down"},
		{TextShowModeBlink, "blink"},
		{TextShowModeFadeIn, "fade_in"},
		{TextShowModeFadeOut, "fade_out"},
		{TextShowModeZoomIn, "zoom_in"},
		{TextShowModeZoomOut, "zoom_out"},
		{TextShowModeRotate, "rotate"},
		{TextShowModeWave, "wave"},
		{TextShowModeCustom, "custom"},
		{TextShowMode(255), "TextShowMode(255)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("TextShowMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestParseTextShowMode(t *testing.T) {
	tests := []struct {
		input   string
		want    TextShowMode
		wantErr bool
	}{
		{"static", TextShowModeStatic, false},
		{"scroll_left", TextShowModeScrollLeft, false},
		{"scroll_right", TextShowModeScrollRight, false},
		{"scroll_up", TextShowModeScrollUp, false},
		{"scroll_down", TextShowModeScrollDown, false},
		{"blink", TextShowModeBlink, false},
		{"fade_in", TextShowModeFadeIn, false},
		{"fade_out", TextShowModeFadeOut, false},
		{"zoom_in", TextShowModeZoomIn, false},
		{"zoom_out", TextShowModeZoomOut, false},
		{"rotate", TextShowModeRotate, false},
		{"wave", TextShowModeWave, false},
		{"custom", TextShowModeCustom, false},
		{"STATIC", TextShowModeStatic, false},
		{"Scroll_Left", TextShowModeScrollLeft, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			got, err := ParseTextShowMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTextShowMode(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseTextShowMode(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestFlipMode_String(t *testing.T) {
	tests := []struct {
		mode FlipMode
		want string
	}{
		{FlipModeNone, "none"},
		{FlipModeHorizontal, "horizontal"},
		{FlipModeVertical, "vertical"},
		{FlipModeBoth, "both"},
		{FlipMode(99), "FlipMode(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("FlipMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestParseFlipMode(t *testing.T) {
	tests := []struct {
		input   string
		want    FlipMode
		wantErr bool
	}{
		{"none", FlipModeNone, false},
		{"horizontal", FlipModeHorizontal, false},
		{"vertical", FlipModeVertical, false},
		{"both", FlipModeBoth, false},
		{"NONE", FlipModeNone, false},
		{"Horizontal", FlipModeHorizontal, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			got, err := ParseFlipMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFlipMode(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseFlipMode(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestBorderMode_String(t *testing.T) {
	tests := []struct {
		mode BorderMode
		want string
	}{
		{BorderModeNone, "none"},
		{BorderModeStatic, "static"},
		{BorderModeDynamic, "dynamic"},
		{BorderModeCustom, "custom"},
		{BorderMode(42), "BorderMode(42)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("BorderMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestBorderType_String(t *testing.T) {
	tests := []struct {
		btype BorderType
		want  string
	}{
		{BorderTypeSolid, "solid"},
		{BorderTypeDotted, "dotted"},
		{BorderTypeDashed, "dashed"},
		{BorderTypeDouble, "double"},
		{BorderTypeGroove, "groove"},
		{BorderTypeRidge, "ridge"},
		{BorderTypeInset, "inset"},
		{BorderTypeOutset, "outset"},
		{BorderTypeWave, "wave"},
		{BorderTypeZigzag, "zigzag"},
		{BorderTypeSawtooth, "sawtooth"},
		{BorderTypeDiamond, "diamond"},
		{BorderTypeCircle, "circle"},
		{BorderTypeSquare, "square"},
		{BorderTypeTriangle, "triangle"},
		{BorderTypeStar, "star"},
		{BorderTypeHeart, "heart"},
		{BorderTypeFlower, "flower"},
		{BorderTypeCustom, "custom"},
		{BorderType(0), "BorderType(0)"},
		{BorderType(99), "BorderType(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.btype.String(); got != tt.want {
				t.Errorf("BorderType(%d).String() = %q, want %q", tt.btype, got, tt.want)
			}
		})
	}
}

func TestFullColorType_String(t *testing.T) {
	tests := []struct {
		ctype FullColorType
		want  string
	}{
		{FullColorTypeRGB, "rgb"},
		{FullColorTypeRed, "red"},
		{FullColorTypeGreen, "green"},
		{FullColorTypeBlue, "blue"},
		{FullColorTypeYellow, "yellow"},
		{FullColorTypeCyan, "cyan"},
		{FullColorTypeMagenta, "magenta"},
		{FullColorTypeWhite, "white"},
		{FullColorTypeBlack, "black"},
		{FullColorTypeOrange, "orange"},
		{FullColorTypePurple, "purple"},
		{FullColorTypePink, "pink"},
		{FullColorTypeBrown, "brown"},
		{FullColorTypeCustom, "custom"},
		{FullColorType(0), "FullColorType(0)"},
		{FullColorType(200), "FullColorType(200)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ctype.String(); got != tt.want {
				t.Errorf("FullColorType(%d).String() = %q, want %q", tt.ctype, got, tt.want)
			}
		})
	}
}

func TestTextShowMode_RoundTrip(t *testing.T) {
	// Every named mode should round-trip through String() -> ParseTextShowMode().
	allModes := []TextShowMode{
		TextShowModeStatic, TextShowModeScrollLeft, TextShowModeScrollRight,
		TextShowModeScrollUp, TextShowModeScrollDown, TextShowModeBlink,
		TextShowModeFadeIn, TextShowModeFadeOut, TextShowModeZoomIn,
		TextShowModeZoomOut, TextShowModeRotate, TextShowModeWave,
		TextShowModeCustom,
	}

	for _, mode := range allModes {
		name := mode.String()
		t.Run(name, func(t *testing.T) {
			parsed, err := ParseTextShowMode(name)
			if err != nil {
				t.Fatalf("ParseTextShowMode(%q): %v", name, err)
			}
			if parsed != mode {
				t.Errorf("round-trip: %d -> %q -> %d", mode, name, parsed)
			}
		})
	}
}

func TestFlipMode_RoundTrip(t *testing.T) {
	allModes := []FlipMode{FlipModeNone, FlipModeHorizontal, FlipModeVertical, FlipModeBoth}

	for _, mode := range allModes {
		name := mode.String()
		t.Run(name, func(t *testing.T) {
			parsed, err := ParseFlipMode(name)
			if err != nil {
				t.Fatalf("ParseFlipMode(%q): %v", name, err)
			}
			if parsed != mode {
				t.Errorf("round-trip: %d -> %q -> %d", mode, name, parsed)
			}
		})
	}
}

func TestParseTextShowMode_CaseInsensitive(t *testing.T) {
	// Verify case-insensitivity works for mixed case.
	inputs := []string{"STATIC", "Static", "sTaTiC", "scroll_LEFT", "FADE_IN"}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := ParseTextShowMode(input)
			if err != nil {
				t.Errorf("ParseTextShowMode(%q) should succeed: %v", input, err)
			}
		})
	}
}

func TestParseFlipMode_CaseInsensitive(t *testing.T) {
	inputs := []string{"NONE", "None", "HORIZONTAL", "Vertical", "BOTH"}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := ParseFlipMode(input)
			if err != nil {
				t.Errorf("ParseFlipMode(%q) should succeed: %v", input, err)
			}
		})
	}
}

func TestUnknownEnum_ContainsTypeName(t *testing.T) {
	// Unknown values should produce strings that contain the type name for debuggability.
	if got := TextShowMode(0).String(); !strings.Contains(got, "TextShowMode") {
		t.Errorf("unknown TextShowMode string = %q, want to contain 'TextShowMode'", got)
	}
	if got := FlipMode(99).String(); !strings.Contains(got, "FlipMode") {
		t.Errorf("unknown FlipMode string = %q, want to contain 'FlipMode'", got)
	}
	if got := BorderMode(50).String(); !strings.Contains(got, "BorderMode") {
		t.Errorf("unknown BorderMode string = %q, want to contain 'BorderMode'", got)
	}
	if got := BorderType(0).String(); !strings.Contains(got, "BorderType") {
		t.Errorf("unknown BorderType string = %q, want to contain 'BorderType'", got)
	}
	if got := FullColorType(0).String(); !strings.Contains(got, "FullColorType") {
		t.Errorf("unknown FullColorType string = %q, want to contain 'FullColorType'", got)
	}
}
