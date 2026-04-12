package models

import (
	"fmt"
	"strings"
)

// TextShowMode defines how text or content is displayed on the LED matrix.
type TextShowMode uint8

const (
	TextShowModeStatic      TextShowMode = 1
	TextShowModeScrollLeft  TextShowMode = 2
	TextShowModeScrollRight TextShowMode = 3
	TextShowModeScrollUp    TextShowMode = 4
	TextShowModeScrollDown  TextShowMode = 5
	TextShowModeBlink       TextShowMode = 6
	TextShowModeFadeIn      TextShowMode = 7
	TextShowModeFadeOut     TextShowMode = 8
	TextShowModeZoomIn      TextShowMode = 9
	TextShowModeZoomOut     TextShowMode = 10
	TextShowModeRotate      TextShowMode = 11
	TextShowModeWave        TextShowMode = 12
	TextShowModeCustom      TextShowMode = 13
)

var textShowModeNames = map[TextShowMode]string{
	TextShowModeStatic:      "static",
	TextShowModeScrollLeft:  "scroll_left",
	TextShowModeScrollRight: "scroll_right",
	TextShowModeScrollUp:    "scroll_up",
	TextShowModeScrollDown:  "scroll_down",
	TextShowModeBlink:       "blink",
	TextShowModeFadeIn:      "fade_in",
	TextShowModeFadeOut:     "fade_out",
	TextShowModeZoomIn:      "zoom_in",
	TextShowModeZoomOut:     "zoom_out",
	TextShowModeRotate:      "rotate",
	TextShowModeWave:        "wave",
	TextShowModeCustom:      "custom",
}

var textShowModeByName map[string]TextShowMode

func init() {
	textShowModeByName = make(map[string]TextShowMode, len(textShowModeNames))
	for mode, name := range textShowModeNames {
		textShowModeByName[name] = mode
	}
}

func (m TextShowMode) String() string {
	if name, ok := textShowModeNames[m]; ok {
		return name
	}
	return fmt.Sprintf("TextShowMode(%d)", m)
}

// ParseTextShowMode parses a string into a TextShowMode.
// The input is case-insensitive.
func ParseTextShowMode(s string) (TextShowMode, error) {
	if mode, ok := textShowModeByName[strings.ToLower(s)]; ok {
		return mode, nil
	}
	return 0, fmt.Errorf("unknown TextShowMode: %q", s)
}

// FlipMode defines display orientation flipping.
type FlipMode uint8

const (
	FlipModeNone       FlipMode = 0
	FlipModeHorizontal FlipMode = 1
	FlipModeVertical   FlipMode = 2
	FlipModeBoth       FlipMode = 3
)

var flipModeNames = map[FlipMode]string{
	FlipModeNone:       "none",
	FlipModeHorizontal: "horizontal",
	FlipModeVertical:   "vertical",
	FlipModeBoth:       "both",
}

var flipModeByName map[string]FlipMode

func init() {
	flipModeByName = make(map[string]FlipMode, len(flipModeNames))
	for mode, name := range flipModeNames {
		flipModeByName[name] = mode
	}
}

func (m FlipMode) String() string {
	if name, ok := flipModeNames[m]; ok {
		return name
	}
	return fmt.Sprintf("FlipMode(%d)", m)
}

// ParseFlipMode parses a string into a FlipMode.
// The input is case-insensitive.
func ParseFlipMode(s string) (FlipMode, error) {
	if mode, ok := flipModeByName[strings.ToLower(s)]; ok {
		return mode, nil
	}
	return 0, fmt.Errorf("unknown FlipMode: %q", s)
}

// BorderMode defines the type of border animation.
type BorderMode uint8

const (
	BorderModeNone    BorderMode = 0
	BorderModeStatic  BorderMode = 1
	BorderModeDynamic BorderMode = 2
	BorderModeCustom  BorderMode = 3
)

func (m BorderMode) String() string {
	switch m {
	case BorderModeNone:
		return "none"
	case BorderModeStatic:
		return "static"
	case BorderModeDynamic:
		return "dynamic"
	case BorderModeCustom:
		return "custom"
	default:
		return fmt.Sprintf("BorderMode(%d)", m)
	}
}

// BorderType defines the visual style of borders.
type BorderType uint8

const (
	BorderTypeSolid    BorderType = 1
	BorderTypeDotted   BorderType = 2
	BorderTypeDashed   BorderType = 3
	BorderTypeDouble   BorderType = 4
	BorderTypeGroove   BorderType = 5
	BorderTypeRidge    BorderType = 6
	BorderTypeInset    BorderType = 7
	BorderTypeOutset   BorderType = 8
	BorderTypeWave     BorderType = 9
	BorderTypeZigzag   BorderType = 10
	BorderTypeSawtooth BorderType = 11
	BorderTypeDiamond  BorderType = 12
	BorderTypeCircle   BorderType = 13
	BorderTypeSquare   BorderType = 14
	BorderTypeTriangle BorderType = 15
	BorderTypeStar     BorderType = 16
	BorderTypeHeart    BorderType = 17
	BorderTypeFlower   BorderType = 18
	BorderTypeCustom   BorderType = 20
)

func (t BorderType) String() string {
	switch t {
	case BorderTypeSolid:
		return "solid"
	case BorderTypeDotted:
		return "dotted"
	case BorderTypeDashed:
		return "dashed"
	case BorderTypeDouble:
		return "double"
	case BorderTypeGroove:
		return "groove"
	case BorderTypeRidge:
		return "ridge"
	case BorderTypeInset:
		return "inset"
	case BorderTypeOutset:
		return "outset"
	case BorderTypeWave:
		return "wave"
	case BorderTypeZigzag:
		return "zigzag"
	case BorderTypeSawtooth:
		return "sawtooth"
	case BorderTypeDiamond:
		return "diamond"
	case BorderTypeCircle:
		return "circle"
	case BorderTypeSquare:
		return "square"
	case BorderTypeTriangle:
		return "triangle"
	case BorderTypeStar:
		return "star"
	case BorderTypeHeart:
		return "heart"
	case BorderTypeFlower:
		return "flower"
	case BorderTypeCustom:
		return "custom"
	default:
		return fmt.Sprintf("BorderType(%d)", t)
	}
}

// FullColorType defines preset color selections.
type FullColorType uint8

const (
	FullColorTypeRGB     FullColorType = 1
	FullColorTypeRed     FullColorType = 2
	FullColorTypeGreen   FullColorType = 3
	FullColorTypeBlue    FullColorType = 4
	FullColorTypeYellow  FullColorType = 5
	FullColorTypeCyan    FullColorType = 6
	FullColorTypeMagenta FullColorType = 7
	FullColorTypeWhite   FullColorType = 8
	FullColorTypeBlack   FullColorType = 9
	FullColorTypeOrange  FullColorType = 10
	FullColorTypePurple  FullColorType = 11
	FullColorTypePink    FullColorType = 12
	FullColorTypeBrown   FullColorType = 13
	FullColorTypeCustom  FullColorType = 14
)

func (t FullColorType) String() string {
	switch t {
	case FullColorTypeRGB:
		return "rgb"
	case FullColorTypeRed:
		return "red"
	case FullColorTypeGreen:
		return "green"
	case FullColorTypeBlue:
		return "blue"
	case FullColorTypeYellow:
		return "yellow"
	case FullColorTypeCyan:
		return "cyan"
	case FullColorTypeMagenta:
		return "magenta"
	case FullColorTypeWhite:
		return "white"
	case FullColorTypeBlack:
		return "black"
	case FullColorTypeOrange:
		return "orange"
	case FullColorTypePurple:
		return "purple"
	case FullColorTypePink:
		return "pink"
	case FullColorTypeBrown:
		return "brown"
	case FullColorTypeCustom:
		return "custom"
	default:
		return fmt.Sprintf("FullColorType(%d)", t)
	}
}
